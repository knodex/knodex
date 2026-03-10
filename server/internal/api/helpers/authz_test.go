// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
)

// MockPolicyEnforcer is a mock implementation of rbac.PolicyEnforcer
type MockPolicyEnforcer struct {
	mock.Mock
}

func (m *MockPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	args := m.Called(ctx, user, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *MockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, userID string, groups []string, object, action string) (bool, error) {
	args := m.Called(ctx, userID, groups, object, action)
	return args.Bool(0), args.Error(1)
}

func (m *MockPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	args := m.Called(ctx, user, projectName, action)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	args := m.Called(ctx, user, roles)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	args := m.Called(ctx, user)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	args := m.Called(ctx, user, role)
	return args.Bool(0), args.Error(1)
}

func (m *MockPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	args := m.Called(ctx, user, role)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	args := m.Called(ctx, projectName)
	return args.Error(0)
}

func (m *MockPolicyEnforcer) InvalidateCache() {
	m.Called()
}

func (m *MockPolicyEnforcer) InvalidateCacheForUser(user string) int {
	args := m.Called(user)
	return args.Int(0)
}

func (m *MockPolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	args := m.Called(projectName)
	return args.Int(0)
}

func (m *MockPolicyEnforcer) CacheStats() rbac.CacheStats {
	args := m.Called()
	return args.Get(0).(rbac.CacheStats)
}

func (m *MockPolicyEnforcer) Metrics() rbac.PolicyMetrics {
	args := m.Called()
	return args.Get(0).(rbac.PolicyMetrics)
}

func (m *MockPolicyEnforcer) IncrementPolicyReloads() {
	m.Called()
}

func (m *MockPolicyEnforcer) IncrementBackgroundSyncs() {
	m.Called()
}

func (m *MockPolicyEnforcer) IncrementWatcherRestarts() {
	m.Called()
}

func (m *MockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	args := m.Called(ctx, user, groups)
	return args.Get(0).([]string), args.Error(1)
}

func TestCheckAccess(t *testing.T) {
	ctx := context.Background()
	userCtx := &middleware.UserContext{
		UserID:      "user@example.com",
		Groups:      []string{"group1"},
		CasbinRoles: []string{"role:serveradmin"},
	}

	t.Run("nil enforcer returns false", func(t *testing.T) {
		allowed, err := CheckAccess(ctx, nil, userCtx, "projects/test", "get")
		assert.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("allowed access", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "get").
			Return(true, nil)

		allowed, err := CheckAccess(ctx, mockEnforcer, userCtx, "projects/test", "get")
		assert.NoError(t, err)
		assert.True(t, allowed)
		mockEnforcer.AssertExpectations(t)
	})

	t.Run("denied access", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "delete").
			Return(false, nil)

		allowed, err := CheckAccess(ctx, mockEnforcer, userCtx, "projects/test", "delete")
		assert.NoError(t, err)
		assert.False(t, allowed)
		mockEnforcer.AssertExpectations(t)
	})

	t.Run("error from enforcer", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "get").
			Return(false, errors.New("enforcer error"))

		allowed, err := CheckAccess(ctx, mockEnforcer, userCtx, "projects/test", "get")
		assert.Error(t, err)
		assert.False(t, allowed)
		mockEnforcer.AssertExpectations(t)
	})
}

func TestRequireAccess(t *testing.T) {
	ctx := context.Background()
	userCtx := &middleware.UserContext{
		UserID:      "user@example.com",
		Groups:      []string{"group1"},
		CasbinRoles: []string{"role:serveradmin"},
	}
	requestID := "test-request-id"

	t.Run("allowed returns true", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "get").
			Return(true, nil)

		w := httptest.NewRecorder()
		result := RequireAccess(w, ctx, mockEnforcer, userCtx, "projects/test", "get", requestID)

		assert.True(t, result)
		assert.Equal(t, 200, w.Code) // No response written
		mockEnforcer.AssertExpectations(t)
	})

	t.Run("denied returns false with 403", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "delete").
			Return(false, nil)

		w := httptest.NewRecorder()
		result := RequireAccess(w, ctx, mockEnforcer, userCtx, "projects/test", "delete", requestID)

		assert.False(t, result)
		assert.Equal(t, 403, w.Code)
		assert.Contains(t, w.Body.String(), "Insufficient permissions")
		mockEnforcer.AssertExpectations(t)
	})

	t.Run("error returns false with 500", func(t *testing.T) {
		mockEnforcer := new(MockPolicyEnforcer)
		mockEnforcer.On("CanAccessWithGroups", ctx, userCtx.UserID, userCtx.Groups, "projects/test", "get").
			Return(false, errors.New("enforcer error"))

		w := httptest.NewRecorder()
		result := RequireAccess(w, ctx, mockEnforcer, userCtx, "projects/test", "get", requestID)

		assert.False(t, result)
		assert.Equal(t, 500, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to check authorization")
		mockEnforcer.AssertExpectations(t)
	})

	t.Run("nil enforcer returns false with 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		result := RequireAccess(w, ctx, nil, userCtx, "projects/test", "get", requestID)

		assert.False(t, result)
		assert.Equal(t, 403, w.Code)
	})
}
