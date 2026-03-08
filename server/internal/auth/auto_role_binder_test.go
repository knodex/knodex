// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package auth provides authentication and authorization services.
// This file contains unit tests for AutoRoleBinder.
package auth

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared mock state
// (testify mock.Mock call tracking and expectation state shared across subtests).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"testing"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// autoRoleBinderMockPolicyEnforcer is a mock implementation of PolicyEnforcer for testing
type autoRoleBinderMockPolicyEnforcer struct {
	mock.Mock
}

func (m *autoRoleBinderMockPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	args := m.Called(ctx, user, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *autoRoleBinderMockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	args := m.Called(ctx, user, groups, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *autoRoleBinderMockPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	args := m.Called(ctx, user, projectName, action)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	args := m.Called(ctx, user, roles)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *autoRoleBinderMockPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	args := m.Called(ctx, user, role)
	return args.Bool(0), args.Error(1)
}

func (m *autoRoleBinderMockPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	args := m.Called(ctx, user, role)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	args := m.Called(ctx, projectName)
	return args.Error(0)
}

func (m *autoRoleBinderMockPolicyEnforcer) InvalidateCache() {
	m.Called()
}

func (m *autoRoleBinderMockPolicyEnforcer) CacheStats() rbac.CacheStats {
	args := m.Called()
	return args.Get(0).(rbac.CacheStats)
}

func (m *autoRoleBinderMockPolicyEnforcer) Metrics() rbac.PolicyMetrics {
	args := m.Called()
	return args.Get(0).(rbac.PolicyMetrics)
}

func (m *autoRoleBinderMockPolicyEnforcer) IncrementPolicyReloads() {
	m.Called()
}

func (m *autoRoleBinderMockPolicyEnforcer) IncrementBackgroundSyncs() {
	m.Called()
}

func (m *autoRoleBinderMockPolicyEnforcer) IncrementWatcherRestarts() {
	m.Called()
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *autoRoleBinderMockPolicyEnforcer) InvalidateCacheForUser(user string) int {
	args := m.Called(user)
	return args.Int(0)
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *autoRoleBinderMockPolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	args := m.Called(projectName)
	return args.Int(0)
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *autoRoleBinderMockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	args := m.Called(ctx, user, groups)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// Verify interface implementation at compile time
var _ rbac.PolicyEnforcer = (*autoRoleBinderMockPolicyEnforcer)(nil)

// Removed autoRoleBinderMockUserService - UserRoleUpdater interface no longer exists
// Global admin status is now tracked solely via Casbin role:serveradmin

func TestNewAutoRoleBinder(t *testing.T) {
	t.Run("nil groupMapper returns error", func(t *testing.T) {
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		binder, err := NewAutoRoleBinder(nil, mockEnforcer)
		assert.Nil(t, binder)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "groupMapper is required")
	})

	t.Run("nil policyEnforcer returns error", func(t *testing.T) {
		mapper := NewGroupMapper(nil)

		binder, err := NewAutoRoleBinder(mapper, nil)
		assert.Nil(t, binder)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policyEnforcer is required")
	})

	t.Run("valid dependencies creates binder", func(t *testing.T) {
		mapper := NewGroupMapper(nil)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)

		require.NoError(t, err)
		require.NotNil(t, binder)
	})

	t.Run("WithLogger option sets logger", func(t *testing.T) {
		mapper := NewGroupMapper(nil)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)
		customLogger := slog.Default()

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithLogger(customLogger))

		require.NoError(t, err)
		require.NotNil(t, binder)
		assert.Equal(t, customLogger, binder.logger)
	})
}

func TestAutoRoleBinder_SyncUserRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("empty groups returns empty result", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "user@example.com", result.UserID)
		assert.Equal(t, 0, result.GroupsEvaluated)
		assert.Empty(t, result.RolesAssigned)
		assert.False(t, result.GlobalAdminGranted)

		// No calls to enforcer expected
		mockEnforcer.AssertNotCalled(t, "AssignUserRoles")
	})

	t.Run("no matching groups returns empty result", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"marketing", "sales"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.GroupsEvaluated)
		assert.Empty(t, result.RolesAssigned)
		assert.False(t, result.GlobalAdminGranted)

		// No calls to enforcer expected
		mockEnforcer.AssertNotCalled(t, "AssignUserRoles")
	})

	t.Run("single group match assigns project role", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Expect AssignUserRoles to be called with the correct roles
		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == "proj:eng-project:developer"
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"engineering"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.GroupsEvaluated)
		assert.Len(t, result.RolesAssigned, 1)
		assert.Equal(t, "eng-project", result.RolesAssigned[0].ProjectID)
		assert.Equal(t, rbac.RoleDeveloper, result.RolesAssigned[0].Role)
		assert.False(t, result.GlobalAdminGranted)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("multiple group matches assign multiple project roles", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
			{Group: "platform", Project: "platform-project", Role: rbac.RolePlatformAdmin},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Expect AssignUserRoles with both roles (order doesn't matter)
		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			if len(roles) != 2 {
				return false
			}
			sort.Strings(roles)
			return roles[0] == "proj:eng-project:developer" && roles[1] == "proj:platform-project:platform-admin"
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"engineering", "platform"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.GroupsEvaluated)
		assert.Len(t, result.RolesAssigned, 2)
		assert.False(t, result.GlobalAdminGranted)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("globalAdmin mapping grants global admin role", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "kro-admins", GlobalAdmin: true},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Expect AssignUserRoles with global admin role
		mockEnforcer.On("AssignUserRoles", ctx, "user:admin@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == rbac.CasbinRoleServerAdmin
		})).Return(nil)

		// Global admin status is now solely determined by Casbin role:serveradmin

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "admin@example.com", []string{"kro-admins"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.GlobalAdminGranted)
		assert.Empty(t, result.RolesAssigned) // GlobalAdmin doesn't add to RolesAssigned

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("globalAdmin with project roles grants both", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "kro-admins", GlobalAdmin: true},
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Expect AssignUserRoles with both project role and global admin role
		mockEnforcer.On("AssignUserRoles", ctx, "user:superuser@example.com", mock.MatchedBy(func(roles []string) bool {
			if len(roles) != 2 {
				return false
			}
			hasProject := false
			hasGlobalAdmin := false
			for _, r := range roles {
				if r == "proj:eng-project:developer" {
					hasProject = true
				}
				if r == rbac.CasbinRoleServerAdmin {
					hasGlobalAdmin = true
				}
			}
			return hasProject && hasGlobalAdmin
		})).Return(nil)

		// Global admin status is now solely determined by Casbin role:serveradmin

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "superuser@example.com", []string{"kro-admins", "engineering"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.GlobalAdminGranted)
		assert.Len(t, result.RolesAssigned, 1)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("globalAdmin assigns Casbin role only", func(t *testing.T) {
		// Global admin status is now tracked solely via Casbin role:serveradmin
		mappings := []config.OIDCGroupMapping{
			{Group: "kro-admins", GlobalAdmin: true},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Expect AssignUserRoles with global admin role
		mockEnforcer.On("AssignUserRoles", ctx, "user:admin@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == rbac.CasbinRoleServerAdmin
		})).Return(nil)

		// Global admin is now handled entirely through Casbin role assignment

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "admin@example.com", []string{"kro-admins"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.GlobalAdminGranted)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("wildcard pattern matching works", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "dev-*", Project: "dev-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == "proj:dev-project:developer"
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"dev-frontend", "dev-backend"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.RolesAssigned, 1)
		assert.Equal(t, "dev-project", result.RolesAssigned[0].ProjectID)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("enforcer error is returned", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.Anything).
			Return(errors.New("enforcer error"))

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"engineering"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to assign user roles")
		require.NotNil(t, result) // result should still be returned even on error

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("role format is correct", func(t *testing.T) {
		// Verify the exact format of Casbin roles
		mappings := []config.OIDCGroupMapping{
			{Group: "admins", Project: "my-project", Role: rbac.RolePlatformAdmin},
			{Group: "devs", Project: "my-project", Role: rbac.RoleDeveloper},
			{Group: "viewers", Project: "other-project", Role: rbac.RoleViewer},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		// Capture the roles being assigned
		var assignedRoles []string
		mockEnforcer.On("AssignUserRoles", ctx, "user:test@example.com", mock.Anything).
			Run(func(args mock.Arguments) {
				assignedRoles = args.Get(2).([]string)
			}).
			Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
		require.NoError(t, err)

		_, err = binder.SyncUserRoles(ctx, "test@example.com", []string{"admins", "devs", "viewers"})
		require.NoError(t, err)

		// Only platform-admin and viewer should be in results (devs role is lower than admins for same project)
		sort.Strings(assignedRoles)
		assert.Contains(t, assignedRoles, "proj:my-project:platform-admin")
		assert.Contains(t, assignedRoles, "proj:other-project:viewer")
		// proj:my-project:developer should NOT be present because platform-admin wins
		assert.NotContains(t, assignedRoles, "proj:my-project:developer")

		mockEnforcer.AssertExpectations(t)
	})
}

func TestAutoRoleBinder_DefaultRole(t *testing.T) {
	ctx := context.Background()

	t.Run("default role assigned when no groups and defaultRole configured", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == "role:test-default"
		})).Return(nil)

		configuredDefaultRole := "role:test-default"
		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole(configuredDefaultRole))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.RolesAssigned, 1)
		assert.NotEmpty(t, result.RolesAssigned[0].Role, "should assign a role")
		assert.Equal(t, configuredDefaultRole, result.RolesAssigned[0].Role, "should assign the configured default role")
		assert.Equal(t, "default-role-config", result.RolesAssigned[0].Source)
		assert.False(t, result.GlobalAdminGranted)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("default role assigned when groups present but no mappings match", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		configuredDefaultRole := "role:test-default"
		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == configuredDefaultRole
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole(configuredDefaultRole))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"marketing", "sales"})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.RolesAssigned, 1)
		assert.NotEmpty(t, result.RolesAssigned[0].Role, "should assign a role")
		assert.Equal(t, configuredDefaultRole, result.RolesAssigned[0].Role, "should assign the configured default role")
		assert.Equal(t, "default-role-config", result.RolesAssigned[0].Source)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("default role NOT assigned when group mappings match", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == "proj:eng-project:developer"
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole("role:test-default"))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"engineering"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.RolesAssigned, 1)
		assert.Equal(t, "eng-project", result.RolesAssigned[0].ProjectID)
		assert.Equal(t, "oidc-group-mapping", result.RolesAssigned[0].Source)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("empty string defaultRole disables default assignment", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole(""))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.RolesAssigned)

		// No calls to enforcer expected
		mockEnforcer.AssertNotCalled(t, "AssignUserRoles")
	})

	t.Run("default role:serveradmin grants GlobalAdminGranted", func(t *testing.T) {
		mapper := NewGroupMapper(nil)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == rbac.CasbinRoleServerAdmin
		})).Return(nil)

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole(rbac.CasbinRoleServerAdmin))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.GlobalAdminGranted)
		assert.Len(t, result.RolesAssigned, 1)
		assert.Equal(t, rbac.CasbinRoleServerAdmin, result.RolesAssigned[0].Role)

		mockEnforcer.AssertExpectations(t)
	})

	t.Run("default role enforcer error is propagated", func(t *testing.T) {
		mapper := NewGroupMapper(nil)
		mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

		mockEnforcer.On("AssignUserRoles", ctx, "user:user@example.com", mock.MatchedBy(func(roles []string) bool {
			return len(roles) == 1 && roles[0] == "role:test-default"
		})).Return(errors.New("enforcer error"))

		binder, err := NewAutoRoleBinder(mapper, mockEnforcer, WithDefaultRole("role:test-default"))
		require.NoError(t, err)

		result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to assign default role")
		require.NotNil(t, result)
		// RolesAssigned should still contain the attempted assignment
		assert.Len(t, result.RolesAssigned, 1)
		assert.Equal(t, "default-role-config", result.RolesAssigned[0].Source)

		mockEnforcer.AssertExpectations(t)
	})
}

func TestAutoRoleBinder_ConcurrentSyncs(t *testing.T) {
	ctx := context.Background()

	mappings := []config.OIDCGroupMapping{
		{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
	}
	mapper := NewGroupMapper(mappings)
	mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)

	// Allow any number of AssignUserRoles calls
	mockEnforcer.On("AssignUserRoles", ctx, mock.Anything, mock.Anything).Return(nil)

	binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
	require.NoError(t, err)

	// Run concurrent syncs
	var wg sync.WaitGroup
	syncErrors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			_, err := binder.SyncUserRoles(ctx, userID, []string{"engineering"})
			if err != nil {
				syncErrors <- err
			}
		}(fmt.Sprintf("user-%d@example.com", i))
	}

	wg.Wait()
	close(syncErrors)

	// No errors expected
	for err := range syncErrors {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRoleAssignment_SourceField(t *testing.T) {
	ctx := context.Background()

	mappings := []config.OIDCGroupMapping{
		{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
	}
	mapper := NewGroupMapper(mappings)
	mockEnforcer := new(autoRoleBinderMockPolicyEnforcer)
	mockEnforcer.On("AssignUserRoles", ctx, mock.Anything, mock.Anything).Return(nil)

	binder, err := NewAutoRoleBinder(mapper, mockEnforcer)
	require.NoError(t, err)

	result, err := binder.SyncUserRoles(ctx, "user@example.com", []string{"engineering"})

	require.NoError(t, err)
	require.Len(t, result.RolesAssigned, 1)

	// Verify Source field is set correctly
	assert.Equal(t, "oidc-group-mapping", result.RolesAssigned[0].Source)
}
