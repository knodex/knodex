// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPolicyEnforcer implements PolicyEnforcer for testing
type mockPolicyEnforcer struct {
	accessibleProjects []string
	projectsErr        error
	canAccess          bool
	canAccessErr       error
}

func (m *mockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	if m.projectsErr != nil {
		return nil, m.projectsErr
	}
	return m.accessibleProjects, nil
}

func (m *mockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	if m.canAccessErr != nil {
		return false, m.canAccessErr
	}
	return m.canAccess, nil
}

// mockNamespaceProvider implements NamespaceProvider for testing
type mockNamespaceProvider struct {
	namespaces    []string
	namespacesErr error
}

func (m *mockNamespaceProvider) GetUserNamespacesWithGroups(ctx context.Context, userID string, groups []string) ([]string, error) {
	if m.namespacesErr != nil {
		return nil, m.namespacesErr
	}
	return m.namespaces, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewAuthorizationService(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{}
	provider := &mockNamespaceProvider{}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.policyEnforcer)
	assert.NotNil(t, svc.namespaceProvider)
	assert.NotNil(t, svc.logger)
}

func TestNewAuthorizationService_NilLogger(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    &mockPolicyEnforcer{},
		NamespaceProvider: &mockNamespaceProvider{},
		Logger:            nil,
	})

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.logger) // Should use default logger
}

func TestGetUserAuthContext_NilUserContext(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    &mockPolicyEnforcer{},
		NamespaceProvider: &mockNamespaceProvider{},
		Logger:            testLogger(),
	})

	authCtx, err := svc.GetUserAuthContext(context.Background(), nil)

	assert.NoError(t, err)
	assert.Nil(t, authCtx)
}

func TestGetUserAuthContext_Success(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		accessibleProjects: []string{"project-a", "project-b"},
	}
	provider := &mockNamespaceProvider{
		namespaces: []string{"ns-a", "ns-b"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		Groups:      []string{"group-1", "group-2"},
		CasbinRoles: []string{"role:developer"},
		Projects:    []string{"jwt-project"},
	}

	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	assert.Equal(t, "user-123", authCtx.UserID)
	assert.Equal(t, []string{"group-1", "group-2"}, authCtx.Groups)
	assert.NotEmpty(t, authCtx.Roles, "auth context should have roles populated")
	assert.Equal(t, userCtx.CasbinRoles, authCtx.Roles, "roles should match middleware input")
	assert.Equal(t, []string{"project-a", "project-b"}, authCtx.AccessibleProjects)
	assert.Equal(t, []string{"ns-a", "ns-b"}, authCtx.AccessibleNamespaces)
}

func TestGetUserAuthContext_GlobalAdmin(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		accessibleProjects: []string{"project-a", "project-b", "project-c"},
	}
	provider := &mockNamespaceProvider{
		namespaces: []string{"*"}, // ["*"] indicates global access
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "admin",
		CasbinRoles: []string{"role:serveradmin"},
	}

	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	assert.Equal(t, []string{"*"}, authCtx.AccessibleNamespaces)
}

func TestGetUserAuthContext_PolicyEnforcerError(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		projectsErr: errors.New("policy enforcer error"),
	}
	provider := &mockNamespaceProvider{
		namespaces: []string{"ns-1"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	// Should succeed with empty projects (secure default)
	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	assert.Empty(t, authCtx.AccessibleProjects) // Secure default
	assert.Equal(t, []string{"ns-1"}, authCtx.AccessibleNamespaces)
}

func TestGetUserAuthContext_NamespaceProviderError(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		accessibleProjects: []string{"project-1"},
	}
	provider := &mockNamespaceProvider{
		namespacesErr: errors.New("namespace provider error"),
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	// Should succeed with empty namespaces (secure default)
	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	assert.Equal(t, []string{"project-1"}, authCtx.AccessibleProjects)
	assert.Empty(t, authCtx.AccessibleNamespaces) // Secure default
}

func TestGetUserAuthContext_NilPolicyEnforcer(t *testing.T) {
	t.Parallel()

	provider := &mockNamespaceProvider{
		namespaces: []string{"ns-1"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    nil, // No enforcer
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:   "user-123",
		Projects: []string{"jwt-project-1", "jwt-project-2"},
	}

	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	// Should fall back to JWT claims
	assert.Equal(t, []string{"jwt-project-1", "jwt-project-2"}, authCtx.AccessibleProjects)
}

func TestGetUserAuthContext_NilNamespaceProvider(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		accessibleProjects: []string{"project-1"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    enforcer,
		NamespaceProvider: nil, // No provider
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	authCtx, err := svc.GetUserAuthContext(context.Background(), userCtx)

	require.NoError(t, err)
	require.NotNil(t, authCtx)
	// Should fail closed: no namespaces accessible when provider is nil
	assert.NotNil(t, authCtx.AccessibleNamespaces)
	assert.Empty(t, authCtx.AccessibleNamespaces)
}

func TestGetAccessibleProjects_NilUserContext(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer:    &mockPolicyEnforcer{accessibleProjects: []string{"project-1"}},
		NamespaceProvider: &mockNamespaceProvider{},
		Logger:            testLogger(),
	})

	projects, err := svc.GetAccessibleProjects(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestGetAccessibleProjects_Success(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		accessibleProjects: []string{"project-a", "project-b"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "user-123",
		Groups:      []string{"group-1"},
		CasbinRoles: []string{"role:developer"},
	}

	projects, err := svc.GetAccessibleProjects(context.Background(), userCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"project-a", "project-b"}, projects)
}

func TestGetAccessibleProjects_NilEnforcer(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: nil,
		Logger:         testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:   "user-123",
		Projects: []string{"jwt-project-1", "jwt-project-2"},
	}

	projects, err := svc.GetAccessibleProjects(context.Background(), userCtx)

	require.NoError(t, err)
	// Should fall back to JWT claims
	assert.Equal(t, []string{"jwt-project-1", "jwt-project-2"}, projects)
}

func TestGetAccessibleProjects_EnforcerError(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		projectsErr: errors.New("enforcer error"),
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	projects, err := svc.GetAccessibleProjects(context.Background(), userCtx)

	assert.Error(t, err)
	assert.Empty(t, projects)
}

func TestGetAccessibleNamespaces_NilUserContext(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		NamespaceProvider: &mockNamespaceProvider{namespaces: []string{"ns-1"}},
		Logger:            testLogger(),
	})

	namespaces, err := svc.GetAccessibleNamespaces(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, namespaces) // Secure default
}

func TestGetAccessibleNamespaces_Success(t *testing.T) {
	t.Parallel()

	provider := &mockNamespaceProvider{
		namespaces: []string{"ns-a", "ns-b", "ns-c"},
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "user-123",
		Groups:      []string{"group-1"},
		CasbinRoles: []string{"role:developer"},
	}

	namespaces, err := svc.GetAccessibleNamespaces(context.Background(), userCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"ns-a", "ns-b", "ns-c"}, namespaces)
}

func TestGetAccessibleNamespaces_GlobalAdmin(t *testing.T) {
	t.Parallel()

	provider := &mockNamespaceProvider{
		namespaces: []string{"*"}, // ["*"] = global access
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "admin",
		CasbinRoles: []string{"role:serveradmin"},
	}

	namespaces, err := svc.GetAccessibleNamespaces(context.Background(), userCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"*"}, namespaces) // ["*"] indicates global access
}

func TestGetAccessibleNamespaces_NilProvider(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		NamespaceProvider: nil,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	namespaces, err := svc.GetAccessibleNamespaces(context.Background(), userCtx)

	require.NoError(t, err)
	assert.NotNil(t, namespaces)
	assert.Empty(t, namespaces) // fail-closed: no namespaces when provider is nil
}

func TestGetAccessibleNamespaces_ProviderError(t *testing.T) {
	t.Parallel()

	provider := &mockNamespaceProvider{
		namespacesErr: errors.New("provider error"),
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		NamespaceProvider: provider,
		Logger:            testLogger(),
	})

	userCtx := &middleware.UserContext{
		UserID: "user-123",
	}

	namespaces, err := svc.GetAccessibleNamespaces(context.Background(), userCtx)

	assert.Error(t, err)
	assert.Empty(t, namespaces) // Secure default
}

func TestCanAccess_NilAuthContext(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccess: true}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	allowed, err := svc.CanAccess(context.Background(), nil, "projects", "get", "projects/test")

	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCanAccess_Allowed(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccess: true}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	authCtx := &UserAuthContext{
		UserID: "user-123",
		Groups: []string{"group-1"},
		Roles:  []string{"role:developer"},
	}

	allowed, err := svc.CanAccess(context.Background(), authCtx, "projects", "get", "projects/test-project")

	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCanAccess_Denied(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccess: false}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	authCtx := &UserAuthContext{
		UserID: "user-123",
		Roles:  []string{"role:viewer"},
	}

	allowed, err := svc.CanAccess(context.Background(), authCtx, "projects", "delete", "projects/test-project")

	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCanAccess_NilEnforcer(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: nil,
		Logger:         testLogger(),
	})

	authCtx := &UserAuthContext{
		UserID: "user-123",
	}

	// Should allow (backward compatibility)
	allowed, err := svc.CanAccess(context.Background(), authCtx, "projects", "get", "projects/test")

	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCanAccess_EnforcerError(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{
		canAccessErr: errors.New("enforcer error"),
	}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	authCtx := &UserAuthContext{
		UserID: "user-123",
	}

	allowed, err := svc.CanAccess(context.Background(), authCtx, "projects", "get", "projects/test")

	assert.Error(t, err)
	assert.False(t, allowed)
}

func TestCanAccessProject(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccess: true}

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		PolicyEnforcer: enforcer,
		Logger:         testLogger(),
	})

	authCtx := &UserAuthContext{
		UserID: "user-123",
	}

	allowed, err := svc.CanAccessProject(context.Background(), authCtx, "my-project", "get")

	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestHasProjectAccess_NilAuthContext(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		Logger: testLogger(),
	})

	has := svc.HasProjectAccess(nil, "project-a")

	assert.False(t, has)
}

func TestHasProjectAccess_Found(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		Logger: testLogger(),
	})

	authCtx := &UserAuthContext{
		AccessibleProjects: []string{"project-a", "project-b", "project-c"},
	}

	assert.True(t, svc.HasProjectAccess(authCtx, "project-a"))
	assert.True(t, svc.HasProjectAccess(authCtx, "project-b"))
	assert.True(t, svc.HasProjectAccess(authCtx, "project-c"))
}

func TestHasProjectAccess_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		Logger: testLogger(),
	})

	authCtx := &UserAuthContext{
		AccessibleProjects: []string{"project-a", "project-b"},
	}

	assert.False(t, svc.HasProjectAccess(authCtx, "project-x"))
	assert.False(t, svc.HasProjectAccess(authCtx, "project-c"))
}

func TestHasProjectAccess_EmptyProjects(t *testing.T) {
	t.Parallel()

	svc := NewAuthorizationService(AuthorizationServiceConfig{
		Logger: testLogger(),
	})

	authCtx := &UserAuthContext{
		AccessibleProjects: []string{},
	}

	assert.False(t, svc.HasProjectAccess(authCtx, "any-project"))
}

// Tests for types.go

func TestNewUserAuthContextFromMiddleware_Nil(t *testing.T) {
	t.Parallel()

	authCtx := NewUserAuthContextFromMiddleware(nil)
	assert.Nil(t, authCtx)
}

func TestNewUserAuthContextFromMiddleware_Success(t *testing.T) {
	t.Parallel()

	userCtx := &middleware.UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		Groups:      []string{"group-1", "group-2"},
		CasbinRoles: []string{"role:serveradmin"},
	}

	authCtx := NewUserAuthContextFromMiddleware(userCtx)

	require.NotNil(t, authCtx)
	assert.Equal(t, "user-123", authCtx.UserID)
	assert.Equal(t, []string{"group-1", "group-2"}, authCtx.Groups)
	assert.NotEmpty(t, authCtx.Roles, "admin middleware context should have roles")
	assert.Equal(t, userCtx.CasbinRoles, authCtx.Roles, "roles should match middleware input")
}

func TestDefaultRGDFilters(t *testing.T) {
	t.Parallel()

	filters := DefaultRGDFilters()

	assert.Equal(t, 1, filters.Page)
	assert.Equal(t, 20, filters.PageSize)
	assert.Equal(t, "name", filters.SortBy)
	assert.Equal(t, "asc", filters.SortOrder)
	assert.Empty(t, filters.Namespace)
	assert.Empty(t, filters.Category)
	assert.Empty(t, filters.Tags)
	assert.Empty(t, filters.Search)
}

// mockProjectService implements rbac.ProjectServiceInterface for testing.
type mockProjectService struct {
	projects map[string]*rbac.Project
	err      error
}

func (m *mockProjectService) GetProject(_ context.Context, name string) (*rbac.Project, error) {
	if m.err != nil {
		return nil, m.err
	}
	if p, ok := m.projects[name]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

func (m *mockProjectService) CreateProject(_ context.Context, _ string, _ rbac.ProjectSpec, _ string) (*rbac.Project, error) {
	return nil, nil
}

func (m *mockProjectService) ListProjects(_ context.Context) (*rbac.ProjectList, error) {
	return nil, nil
}

func (m *mockProjectService) UpdateProject(_ context.Context, p *rbac.Project, _ string) (*rbac.Project, error) {
	return p, nil
}

func (m *mockProjectService) DeleteProject(_ context.Context, _ string) error {
	return nil
}

func (m *mockProjectService) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.projects[name]
	return ok, nil
}

func (m *mockProjectService) UpdateProjectStatus(_ context.Context, p *rbac.Project) (*rbac.Project, error) {
	return p, nil
}

// --- ProjectTypeResolver tests ---

func TestNewProjectTypeResolver_NilProjectService(t *testing.T) {
	t.Parallel()
	resolver := NewProjectTypeResolver(nil, testLogger())
	assert.Nil(t, resolver, "nil project service should return nil resolver")
}

func TestProjectTypeResolver_SingleAppProject(t *testing.T) {
	t.Parallel()
	ps := &mockProjectService{
		projects: map[string]*rbac.Project{
			"my-app": {Spec: rbac.ProjectSpec{Type: rbac.ProjectTypeApp}},
		},
	}
	resolver := NewProjectTypeResolver(ps, testLogger())
	types, err := resolver.GetProjectTypes(context.Background(), []string{"my-app"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"my-app": "app"}, types)
}

func TestProjectTypeResolver_SinglePlatformProject(t *testing.T) {
	t.Parallel()
	ps := &mockProjectService{
		projects: map[string]*rbac.Project{
			"infra": {Spec: rbac.ProjectSpec{Type: rbac.ProjectTypePlatform}},
		},
	}
	resolver := NewProjectTypeResolver(ps, testLogger())
	types, err := resolver.GetProjectTypes(context.Background(), []string{"infra"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"infra": "platform"}, types)
}

func TestProjectTypeResolver_MixedProjects(t *testing.T) {
	t.Parallel()
	ps := &mockProjectService{
		projects: map[string]*rbac.Project{
			"my-app": {Spec: rbac.ProjectSpec{Type: rbac.ProjectTypeApp}},
			"infra":  {Spec: rbac.ProjectSpec{Type: rbac.ProjectTypePlatform}},
		},
	}
	resolver := NewProjectTypeResolver(ps, testLogger())
	types, err := resolver.GetProjectTypes(context.Background(), []string{"my-app", "infra"})
	require.NoError(t, err)
	assert.Equal(t, "app", types["my-app"])
	assert.Equal(t, "platform", types["infra"])
}

func TestProjectTypeResolver_MissingTypeDefaultsToApp(t *testing.T) {
	t.Parallel()
	ps := &mockProjectService{
		projects: map[string]*rbac.Project{
			"legacy": {Spec: rbac.ProjectSpec{}}, // No Type set
		},
	}
	resolver := NewProjectTypeResolver(ps, testLogger())
	types, err := resolver.GetProjectTypes(context.Background(), []string{"legacy"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"legacy": "app"}, types, "empty type should default to app")
}

func TestProjectTypeResolver_ProjectNotFoundDefaultsToApp(t *testing.T) {
	t.Parallel()
	ps := &mockProjectService{
		projects: map[string]*rbac.Project{}, // No projects
	}
	resolver := NewProjectTypeResolver(ps, testLogger())
	types, err := resolver.GetProjectTypes(context.Background(), []string{"nonexistent"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"nonexistent": "app"}, types, "not-found project should default to app")
}
