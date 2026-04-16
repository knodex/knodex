// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package userprefs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func withUserContext(r *http.Request, userID string) *http.Request {
	userCtx := &middleware.UserContext{UserID: userID}
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
	return r.WithContext(ctx)
}

// --- Store tests ---

func TestRedisStore_Get_ReturnsEmptyForNewUser(t *testing.T) {
	t.Parallel()
	client, _ := newTestRedis(t)
	store := NewRedisStore(client)

	prefs, err := store.Get(context.Background(), "new-user@test.local")

	require.NoError(t, err)
	assert.Empty(t, prefs.FavoriteRgds)
	assert.Empty(t, prefs.RecentRgds)
}

func TestRedisStore_PutAndGet(t *testing.T) {
	t.Parallel()
	client, _ := newTestRedis(t)
	store := NewRedisStore(client)

	prefs := &UserPreferences{
		FavoriteRgds: []string{"default/postgresql", "prod/redis"},
		RecentRgds:   []string{"default/mysql"},
	}

	err := store.Put(context.Background(), "user@test.local", prefs)
	require.NoError(t, err)

	got, err := store.Get(context.Background(), "user@test.local")
	require.NoError(t, err)
	assert.Equal(t, prefs.FavoriteRgds, got.FavoriteRgds)
	assert.Equal(t, prefs.RecentRgds, got.RecentRgds)
}

func TestRedisStore_Put_SetsTTL(t *testing.T) {
	t.Parallel()
	client, mr := newTestRedis(t)
	store := NewRedisStore(client)

	prefs := &UserPreferences{
		FavoriteRgds: []string{"default/postgresql"},
		RecentRgds:   []string{},
	}

	err := store.Put(context.Background(), "user@test.local", prefs)
	require.NoError(t, err)

	ttl := mr.TTL(redisKey("user@test.local"))
	// TTL should be approximately 90 days (7776000 seconds)
	assert.Greater(t, ttl, 89*24*time.Hour)
	assert.LessOrEqual(t, ttl, 90*24*time.Hour)
}

func TestRedisStore_Put_TruncatesFavorites(t *testing.T) {
	t.Parallel()
	client, _ := newTestRedis(t)
	store := NewRedisStore(client)

	// Create 15 favorites (exceeds max of 10)
	favorites := make([]string, 15)
	for i := range favorites {
		favorites[i] = "rgd-" + string(rune('a'+i))
	}

	prefs := &UserPreferences{
		FavoriteRgds: favorites,
		RecentRgds:   []string{},
	}

	err := store.Put(context.Background(), "user@test.local", prefs)
	require.NoError(t, err)

	got, err := store.Get(context.Background(), "user@test.local")
	require.NoError(t, err)
	assert.Len(t, got.FavoriteRgds, MaxFavorites)
}

func TestRedisStore_Put_TruncatesRecent(t *testing.T) {
	t.Parallel()
	client, _ := newTestRedis(t)
	store := NewRedisStore(client)

	// Create 25 recent items (exceeds max of 20)
	recent := make([]string, 25)
	for i := range recent {
		recent[i] = "rgd-" + string(rune('a'+i))
	}

	prefs := &UserPreferences{
		FavoriteRgds: []string{},
		RecentRgds:   recent,
	}

	err := store.Put(context.Background(), "user@test.local", prefs)
	require.NoError(t, err)

	got, err := store.Get(context.Background(), "user@test.local")
	require.NoError(t, err)
	assert.Len(t, got.RecentRgds, MaxRecent)
}

// --- Handler tests ---

func TestHandler_GetPreferences_Returns401_NoAuth(t *testing.T) {
	t.Parallel()
	handler := NewHandler(&mockStore{}, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/preferences", nil)

	handler.GetPreferences(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_PutPreferences_Returns401_NoAuth(t *testing.T) {
	t.Parallel()
	handler := NewHandler(&mockStore{}, nil)
	rec := httptest.NewRecorder()
	body := `{"favoriteRgds":[],"recentRgds":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/preferences", bytes.NewBufferString(body))

	handler.PutPreferences(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_GetPreferences_ReturnsEmpty_NewUser(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	handler := NewHandler(store, nil)
	rec := httptest.NewRecorder()
	req := withUserContext(
		httptest.NewRequest(http.MethodGet, "/api/v1/users/preferences", nil),
		"user@test.local",
	)

	handler.GetPreferences(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var prefs UserPreferences
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&prefs))
	assert.Empty(t, prefs.FavoriteRgds)
	assert.Empty(t, prefs.RecentRgds)
}

func TestHandler_PutPreferences_Returns200(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	handler := NewHandler(store, nil)
	rec := httptest.NewRecorder()
	body := `{"favoriteRgds":["default/postgresql"],"recentRgds":["default/redis"]}`
	req := withUserContext(
		httptest.NewRequest(http.MethodPut, "/api/v1/users/preferences", bytes.NewBufferString(body)),
		"user@test.local",
	)

	handler.PutPreferences(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var prefs UserPreferences
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&prefs))
	assert.Equal(t, []string{"default/postgresql"}, prefs.FavoriteRgds)
	assert.Equal(t, []string{"default/redis"}, prefs.RecentRgds)
}

func TestHandler_PutPreferences_Returns400_InvalidBody(t *testing.T) {
	t.Parallel()
	handler := NewHandler(&mockStore{}, nil)
	rec := httptest.NewRecorder()
	req := withUserContext(
		httptest.NewRequest(http.MethodPut, "/api/v1/users/preferences", bytes.NewBufferString("not json")),
		"user@test.local",
	)

	handler.PutPreferences(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BAD_REQUEST")
}

// --- Mock store ---

type mockStore struct {
	prefs map[string]*UserPreferences
}

func (m *mockStore) Get(_ context.Context, userID string) (*UserPreferences, error) {
	if m.prefs == nil {
		return &UserPreferences{FavoriteRgds: []string{}, RecentRgds: []string{}}, nil
	}
	if p, ok := m.prefs[userID]; ok {
		return p, nil
	}
	return &UserPreferences{FavoriteRgds: []string{}, RecentRgds: []string{}}, nil
}

func (m *mockStore) Put(_ context.Context, userID string, prefs *UserPreferences) error {
	if m.prefs == nil {
		m.prefs = make(map[string]*UserPreferences)
	}
	// Apply truncation like the real store
	fav := prefs.FavoriteRgds
	if len(fav) > MaxFavorites {
		fav = fav[:MaxFavorites]
	}
	rec := prefs.RecentRgds
	if len(rec) > MaxRecent {
		rec = rec[:MaxRecent]
	}
	m.prefs[userID] = &UserPreferences{FavoriteRgds: fav, RecentRgds: rec}
	return nil
}
