// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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

	"github.com/knodex/knodex/server/internal/testutil"
)

// newTestPolicyEnforcerWithRedis creates a PolicyEnforcer with a RedisRoleStore backed by miniredis.
// Returns the enforcer, miniredis instance (for TTL simulation), and the RedisRoleStore.
func newTestPolicyEnforcerWithRedis(t *testing.T) (PolicyEnforcer, *miniredis.Miniredis, *RedisRoleStore) {
	t.Helper()

	mr, client := testutil.NewRedis(t)
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	return pe, mr, roleStore
}

func TestPolicyEnforcer_AssignUserRoles_PersistsToRedis(t *testing.T) {
	t.Parallel()

	pe, _, roleStore := newTestPolicyEnforcerWithRedis(t)
	ctx := context.Background()

	// Assign roles via PolicyEnforcer
	err := pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin", "proj:alpha:developer"})
	require.NoError(t, err)

	// Verify roles are persisted in Redis
	roles, err := roleStore.LoadUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin", "proj:alpha:developer"}, roles)
}

func TestPolicyEnforcer_AssignUserRoles_ReplacesInRedis(t *testing.T) {
	t.Parallel()

	pe, _, roleStore := newTestPolicyEnforcerWithRedis(t)
	ctx := context.Background()

	// Assign initial roles
	err := pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Re-assign with new roles
	err = pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"proj:beta:readonly"})
	require.NoError(t, err)

	// Verify only new roles are in Redis
	roles, err := roleStore.LoadUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"proj:beta:readonly"}, roles)
}

func TestPolicyEnforcer_RemoveUserRoles_DeletesFromRedis(t *testing.T) {
	t.Parallel()

	pe, _, roleStore := newTestPolicyEnforcerWithRedis(t)
	ctx := context.Background()

	// Assign roles
	err := pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Remove all roles
	err = pe.RemoveUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)

	// Verify Redis is empty for this user
	roles, err := roleStore.LoadUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestPolicyEnforcer_RemoveUserRole_UpdatesRedis(t *testing.T) {
	t.Parallel()

	pe, _, roleStore := newTestPolicyEnforcerWithRedis(t)
	ctx := context.Background()

	// Assign multiple roles
	err := pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin", "proj:alpha:developer"})
	require.NoError(t, err)

	// Remove one role
	err = pe.RemoveUserRole(ctx, "user:alice@test.com", "role:serveradmin")
	require.NoError(t, err)

	// Verify only the remaining role is in Redis
	roles, err := roleStore.LoadUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"proj:alpha:developer"}, roles)
}

func TestPolicyEnforcer_RestorePersistedRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Set up miniredis and role store
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	// Pre-populate Redis with roles (simulating data from before restart)
	err := roleStore.SaveUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)
	err = roleStore.SaveUserRoles(ctx, "user:bob@test.com", []string{"proj:alpha:developer", "proj:beta:readonly"})
	require.NoError(t, err)

	// Create a FRESH enforcer (simulating pod restart)
	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	// Before restore, users should have no roles
	aliceRoles, err := pe.GetUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.Empty(t, aliceRoles)

	// Restore roles from Redis
	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)

	// After restore, users should have their roles back
	aliceRoles, err = pe.GetUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin"}, aliceRoles)

	bobRoles, err := pe.GetUserRoles(ctx, "user:bob@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"proj:alpha:developer", "proj:beta:readonly"}, bobRoles)
}

func TestPolicyEnforcer_RestorePersistedRoles_NoRoleStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create enforcer without Redis role store
	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
	)

	// Should be a no-op without error
	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)
}

func TestPolicyEnforcer_RestorePersistedRoles_EmptyRedis(t *testing.T) {
	t.Parallel()

	pe, _, _ := newTestPolicyEnforcerWithRedis(t)
	ctx := context.Background()

	// Restore from empty Redis — should succeed with no side effects
	err := pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)
}

func TestPolicyEnforcer_RestorePersistedRoles_SkipsInvalidRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	// Save a mix of valid and invalid roles
	err := roleStore.SaveUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin", "invalid-role-format", "proj:alpha:developer"})
	require.NoError(t, err)

	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)

	// Only valid roles should be restored
	roles, err := pe.GetUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin", "proj:alpha:developer"}, roles)
}

func TestPolicyEnforcer_RestoreAndAuthorize(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Set up Redis with persisted admin role
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	err := roleStore.SaveUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Create fresh enforcer and restore
	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)

	// Alice should now be able to access resources as server admin
	// role:serveradmin has built-in policy: *, *, allow
	allowed, err := pe.CanAccess(ctx, "user:alice@test.com", "projects/test", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "admin should have access after role restore")

	// Non-existing user should not have access
	allowed, err = pe.CanAccess(ctx, "user:nobody@test.com", "projects/test", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "unknown user should not have access")
}

func TestPolicyEnforcer_RestorePersistedRoles_ExpiredKeysDuringRestore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// Use a very short TTL so keys expire between save and restore
	roleStore := NewRedisRoleStore(client, 1*time.Second, slog.Default())

	// Save roles for two users
	err := roleStore.SaveUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)
	err = roleStore.SaveUserRoles(ctx, "user:bob@test.com", []string{"proj:alpha:developer"})
	require.NoError(t, err)

	// Fast-forward so keys expire (simulates SCAN finding keys that then expire before SMEMBERS)
	mr.FastForward(2 * time.Second)

	// Create fresh enforcer and attempt restore
	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	// Restore should succeed (gracefully handles expired/missing keys)
	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)

	// Neither user should have roles (all expired)
	aliceRoles, err := pe.GetUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.Empty(t, aliceRoles)

	bobRoles, err := pe.GetUserRoles(ctx, "user:bob@test.com")
	require.NoError(t, err)
	assert.Empty(t, bobRoles)
}

func TestPolicyEnforcer_RestorePersistedRoles_OnlyCountsUsersWithValidRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	// Save ONLY invalid roles for one user
	err := roleStore.SaveUserRoles(ctx, "user:bad@test.com", []string{"invalid-format", "also-invalid"})
	require.NoError(t, err)

	// Save valid roles for another user
	err = roleStore.SaveUserRoles(ctx, "user:good@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)

	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	err = pe.RestorePersistedRoles(ctx)
	require.NoError(t, err)

	// Only good user should have roles restored
	goodRoles, err := pe.GetUserRoles(ctx, "user:good@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin"}, goodRoles)

	badRoles, err := pe.GetUserRoles(ctx, "user:bad@test.com")
	require.NoError(t, err)
	assert.Empty(t, badRoles)
}

func TestPolicyEnforcer_RedisDegradation_AssignStillWorks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	roleStore := NewRedisRoleStore(client, 24*time.Hour, slog.Default())

	casbinEnforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(
		casbinEnforcer,
		newMockProjectReader(),
		PolicyEnforcerConfig{
			CacheEnabled: false,
			Logger:       slog.Default(),
		},
		WithRedisRoleStore(roleStore),
	)

	// Close Redis to simulate unavailability
	mr.Close()

	// AssignUserRoles should still succeed (in-memory works, Redis gracefully degrades)
	err = pe.AssignUserRoles(ctx, "user:alice@test.com", []string{"role:serveradmin"})
	require.NoError(t, err)

	// In-memory roles should be present
	roles, err := pe.GetUserRoles(ctx, "user:alice@test.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"role:serveradmin"}, roles)
}
