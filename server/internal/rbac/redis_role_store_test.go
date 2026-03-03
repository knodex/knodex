package rbac

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provops-org/knodex/server/internal/testutil"
)

func newTestRedisRoleStore(t *testing.T) (*RedisRoleStore, *miniredis.Miniredis) {
	t.Helper()
	mr, client := testutil.NewRedis(t)
	store := NewRedisRoleStore(client, 24*time.Hour, slog.Default())
	return store, mr
}

func TestRedisRoleStore_SaveAndLoadUserRoles(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save roles
	err := store.SaveUserRoles(ctx, "user:abc123", []string{"role:serveradmin", "proj:alpha:developer"})
	require.NoError(t, err)

	// Load roles
	roles, err := store.LoadUserRoles(ctx, "user:abc123")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin", "proj:alpha:developer"}, roles)
}

func TestRedisRoleStore_SaveReplacesPreviousRoles(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save initial roles
	err := store.SaveUserRoles(ctx, "user:abc123", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Save new roles (should replace)
	err = store.SaveUserRoles(ctx, "user:abc123", []string{"role:test-viewer", "proj:beta:admin"})
	require.NoError(t, err)

	// Load should return only new roles
	roles, err := store.LoadUserRoles(ctx, "user:abc123")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:test-viewer", "proj:beta:admin"}, roles)
}

func TestRedisRoleStore_LoadNonExistentUser(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	roles, err := store.LoadUserRoles(ctx, "user:nonexistent")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestRedisRoleStore_LoadAllUserRoles(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save roles for multiple users
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	require.NoError(t, err)
	err = store.SaveUserRoles(ctx, "user:def", []string{"role:test-viewer", "proj:alpha:developer"})
	require.NoError(t, err)
	err = store.SaveUserRoles(ctx, "user:ghi", []string{"proj:beta:admin"})
	require.NoError(t, err)

	// Load all
	all, err := store.LoadAllUserRoles(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.ElementsMatch(t, []string{"role:serveradmin"}, all["user:abc"])
	assert.ElementsMatch(t, []string{"role:test-viewer", "proj:alpha:developer"}, all["user:def"])
	assert.ElementsMatch(t, []string{"proj:beta:admin"}, all["user:ghi"])
}

func TestRedisRoleStore_LoadAllUserRoles_Empty(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	all, err := store.LoadAllUserRoles(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestRedisRoleStore_DeleteUserRoles(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save roles
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Delete roles
	err = store.DeleteUserRoles(ctx, "user:abc")
	require.NoError(t, err)

	// Should be empty after delete
	roles, err := store.LoadUserRoles(ctx, "user:abc")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestRedisRoleStore_TTLExpiry(t *testing.T) {
	t.Parallel()

	store, mr := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save roles with 24h TTL
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Verify roles exist
	roles, err := store.LoadUserRoles(ctx, "user:abc")
	require.NoError(t, err)
	assert.Len(t, roles, 1)

	// Fast forward past TTL
	mr.FastForward(25 * time.Hour)

	// Roles should be expired
	roles, err = store.LoadUserRoles(ctx, "user:abc")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestRedisRoleStore_NilClient_GracefulDegradation(t *testing.T) {
	t.Parallel()

	store := NewRedisRoleStore(nil, 24*time.Hour, slog.Default())
	ctx := context.Background()

	// All operations should return nil/empty without error
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	assert.NoError(t, err)

	roles, err := store.LoadUserRoles(ctx, "user:abc")
	assert.NoError(t, err)
	assert.Nil(t, roles)

	all, err := store.LoadAllUserRoles(ctx)
	assert.NoError(t, err)
	assert.Nil(t, all)

	err = store.DeleteUserRoles(ctx, "user:abc")
	assert.NoError(t, err)
}

func TestRedisRoleStore_EmptyUserID(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	err := store.SaveUserRoles(ctx, "", []string{"role:serveradmin"})
	assert.Error(t, err)

	_, err = store.LoadUserRoles(ctx, "")
	assert.Error(t, err)

	err = store.DeleteUserRoles(ctx, "")
	assert.Error(t, err)
}

func TestRedisRoleStore_SaveEmptyRoles(t *testing.T) {
	t.Parallel()

	store, _ := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save with existing roles first
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Save empty roles (should clear)
	err = store.SaveUserRoles(ctx, "user:abc", []string{})
	require.NoError(t, err)

	// Should be empty
	roles, err := store.LoadUserRoles(ctx, "user:abc")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestRedisRoleStore_TTLRefreshOnReAssignment(t *testing.T) {
	t.Parallel()

	store, mr := newTestRedisRoleStore(t)
	ctx := context.Background()

	// Save roles
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Fast forward 20 hours (4 hours left before expiry)
	mr.FastForward(20 * time.Hour)

	// Re-assign roles (should refresh TTL)
	err = store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin", "proj:alpha:developer"})
	require.NoError(t, err)

	// Fast forward another 20 hours (would be expired if TTL wasn't refreshed)
	mr.FastForward(20 * time.Hour)

	// Roles should still exist because TTL was refreshed
	roles, err := store.LoadUserRoles(ctx, "user:abc")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin", "proj:alpha:developer"}, roles)
}

func TestRedisRoleStore_ClosedConnection_GracefulDegradation(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRoleStore(client, 24*time.Hour, slog.Default())
	ctx := context.Background()

	// Close miniredis to simulate unavailability
	mr.Close()

	// All operations should return nil without panicking
	err := store.SaveUserRoles(ctx, "user:abc", []string{"role:serveradmin"})
	assert.NoError(t, err) // Graceful degradation - no error

	roles, err := store.LoadUserRoles(ctx, "user:abc")
	assert.NoError(t, err)
	assert.Nil(t, roles)

	all, err := store.LoadAllUserRoles(ctx)
	assert.NoError(t, err)
	assert.Nil(t, all)

	err = store.DeleteUserRoles(ctx, "user:abc")
	assert.NoError(t, err)
}
