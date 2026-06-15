// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package groups

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is a controllable clock for deterministic last-seen scoring.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestStore(t *testing.T) (*RedisStore, *miniredis.Miniredis, *fakeClock) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	store := NewRedisStore(client)
	store.now = clk.now
	return store, mr, clk
}

func names(groups []ObservedGroup) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.Name
	}
	return out
}

func TestRedisStore_List_EmptyWhenNothingRecorded(t *testing.T) {
	t.Parallel()
	store, _, _ := newTestStore(t)

	got, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRedisStore_Record_StoresDistinctGroupsMostRecentFirst(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, []string{"alpha-devs"}))
	clk.advance(2 * time.Second)
	require.NoError(t, store.Record(ctx, []string{"beta-ops"}))
	clk.advance(2 * time.Second)
	require.NoError(t, store.Record(ctx, []string{"gamma-admins"}))

	got, err := store.List(ctx)
	require.NoError(t, err)
	// Most-recently-seen first (AC #2).
	assert.Equal(t, []string{"gamma-admins", "beta-ops", "alpha-devs"}, names(got))
	for _, g := range got {
		assert.False(t, g.LastSeen.IsZero(), "last-seen timestamp should be set")
	}
}

func TestRedisStore_Record_DedupsAndAdvancesLastSeen(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, []string{"alpha-devs", "beta-ops"}))
	clk.advance(10 * time.Second)
	// Re-record alpha-devs: it must dedup (appear once) and become most-recent.
	require.NoError(t, store.Record(ctx, []string{"alpha-devs"}))

	got, err := store.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha-devs", "beta-ops"}, names(got), "deduped, alpha-devs now most-recent")
	assert.Len(t, got, 2, "alpha-devs appears once despite two recordings")
}

func TestRedisStore_Record_DedupsRepeatedWithinOneCall(t *testing.T) {
	t.Parallel()
	store, _, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, []string{"alpha-devs", "alpha-devs", "alpha-devs"}))

	got, err := store.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha-devs"}, names(got))
}

func TestRedisStore_Record_PrunesToCapKeepingMostRecent(t *testing.T) {
	t.Parallel()
	store, _, clk := newTestStore(t)
	ctx := context.Background()

	// Record maxEntries+50 distinct groups, each at a strictly later time so the
	// last-seen ordering is deterministic.
	total := maxEntries + 50
	for i := 0; i < total; i++ {
		require.NoError(t, store.Record(ctx, []string{fmt.Sprintf("group-%05d", i)}))
		clk.advance(time.Second)
	}

	got, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, got, maxEntries, "store is bounded to maxEntries (AC #3)")
	// The most-recently-seen group is the last one recorded.
	assert.Equal(t, fmt.Sprintf("group-%05d", total-1), got[0].Name)
	// The oldest survivor is group at index (total - maxEntries); everything
	// older was pruned.
	oldestSurvivor := fmt.Sprintf("group-%05d", total-maxEntries)
	assert.Equal(t, oldestSurvivor, got[len(got)-1].Name)
}

func TestRedisStore_Record_StripsControlCharsAndSkipsEmpty(t *testing.T) {
	t.Parallel()
	store, _, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, []string{
		"  spaced-group  ",  // trimmed
		"ctrl\x00chars\x1b", // control chars stripped
		"",                  // skipped
		"   ",               // whitespace-only -> skipped
		"\t\n",              // control/whitespace-only -> skipped
	}))

	got, err := store.List(ctx)
	require.NoError(t, err)
	storedNames := names(got)
	assert.Contains(t, storedNames, "spaced-group")
	assert.Contains(t, storedNames, "ctrlchars")
	assert.Len(t, storedNames, 2, "empty and whitespace-only groups skipped")
}

func TestRedisStore_Record_NoGroupsIsNoop(t *testing.T) {
	t.Parallel()
	store, mr, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, nil))
	require.NoError(t, store.Record(ctx, []string{}))

	assert.False(t, mr.Exists(observedKey), "no key created when nothing recorded")
}

func TestRedisStore_Record_SetsKeyTTL(t *testing.T) {
	t.Parallel()
	store, mr, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Record(ctx, []string{"alpha-devs"}))

	ttl := mr.TTL(observedKey)
	assert.Greater(t, ttl, 89*24*time.Hour)
	assert.LessOrEqual(t, ttl, observedTTL)
}

func TestRedisStore_NilSafe(t *testing.T) {
	t.Parallel()
	var s *RedisStore
	require.NoError(t, s.Record(context.Background(), []string{"x"}))
	got, err := s.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}
