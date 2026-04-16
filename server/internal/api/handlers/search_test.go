// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
)

// --- Mock implementations ---

// mockSearchRGDProvider implements services.RGDProvider for search tests.
type mockSearchRGDProvider struct {
	rgds []models.CatalogRGD
}

func (m *mockSearchRGDProvider) ListRGDs(_ models.ListOptions) models.CatalogRGDList {
	return models.CatalogRGDList{
		Items:      m.rgds,
		TotalCount: len(m.rgds),
		Page:       1,
		PageSize:   50,
	}
}

func (m *mockSearchRGDProvider) GetRGD(namespace, name string) (*models.CatalogRGD, bool) {
	for _, rgd := range m.rgds {
		if rgd.Name == name && rgd.Namespace == namespace {
			return &rgd, true
		}
	}
	return nil, false
}

func (m *mockSearchRGDProvider) GetRGDByName(name string) (*models.CatalogRGD, bool) {
	for _, rgd := range m.rgds {
		if rgd.Name == name {
			return &rgd, true
		}
	}
	return nil, false
}

// mockSearchProjectService implements rbac.ProjectServiceInterface for search tests.
type mockSearchProjectService struct {
	projects []rbac.Project
	err      error
}

func (m *mockSearchProjectService) CreateProject(_ context.Context, _ string, _ rbac.ProjectSpec, _ string) (*rbac.Project, error) {
	return nil, nil
}

func (m *mockSearchProjectService) GetProject(_ context.Context, name string) (*rbac.Project, error) {
	for _, p := range m.projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, nil
}

func (m *mockSearchProjectService) ListProjects(_ context.Context) (*rbac.ProjectList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &rbac.ProjectList{Items: m.projects}, nil
}

func (m *mockSearchProjectService) UpdateProject(_ context.Context, _ *rbac.Project, _ string) (*rbac.Project, error) {
	return nil, nil
}

func (m *mockSearchProjectService) DeleteProject(_ context.Context, _ string) error {
	return nil
}

func (m *mockSearchProjectService) UpdateProjectStatus(_ context.Context, project *rbac.Project) (*rbac.Project, error) {
	return project, nil
}

func (m *mockSearchProjectService) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// mockSearchPolicyEnforcer implements services.PolicyEnforcer for scoped access tests.
type mockSearchPolicyEnforcer struct {
	accessibleProjects []string
}

func (m *mockSearchPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return true, nil
}

func (m *mockSearchPolicyEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return m.accessibleProjects, nil
}

// mockSearchNamespaceProvider implements services.NamespaceProvider for scoped access tests.
type mockSearchNamespaceProvider struct {
	namespaces []string
}

func (m *mockSearchNamespaceProvider) GetUserNamespacesWithGroups(_ context.Context, _ string, _ []string) ([]string, error) {
	return m.namespaces, nil
}

// --- Helpers ---

func newSearchRequest(query string, userCtx *middleware.UserContext) *http.Request {
	path := "/api/v1/search"
	if query != "" {
		path += "?q=" + url.QueryEscape(query)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

func defaultTestSearchUser() *middleware.UserContext {
	return &middleware.UserContext{
		UserID: "user@test.local",
		Groups: []string{"developers"},
	}
}

func parseSearchResponse(t *testing.T, rec *httptest.ResponseRecorder) SearchResponse {
	t.Helper()
	var resp SearchResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	return resp
}

// --- Tests ---

func TestSearchHandler_EmptyQuery_Returns400(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(SearchHandlerConfig{})
	rec := httptest.NewRecorder()
	req := newSearchRequest("", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BAD_REQUEST")
}

func TestSearchHandler_WhitespaceQuery_Returns400(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(SearchHandlerConfig{})
	rec := httptest.NewRecorder()
	req := newSearchRequest("   ", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "BAD_REQUEST")
}

func TestSearchHandler_NoAuth_Returns401(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(SearchHandlerConfig{})
	rec := httptest.NewRecorder()
	req := newSearchRequest("test", nil) // no user context

	handler.Search(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSearchHandler_ReturnsRGDResults(t *testing.T) {
	t.Parallel()

	provider := &mockSearchRGDProvider{
		rgds: []models.CatalogRGD{
			{Name: "postgres-db", Title: "PostgreSQL Database", Category: "database", Description: "Managed PostgreSQL"},
			{Name: "redis-cache", Title: "Redis Cache", Category: "caching", Description: "Redis key-value store"},
		},
	}
	catalogService := services.NewCatalogService(services.CatalogServiceConfig{
		RGDProvider: provider,
	})

	handler := NewSearchHandler(SearchHandlerConfig{
		CatalogService: catalogService,
	})

	rec := httptest.NewRecorder()
	req := newSearchRequest("postgres", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)
	assert.Equal(t, "postgres", resp.Query)
	assert.NotEmpty(t, resp.Results.RGDs)

	// Verify RGD result fields
	found := false
	for _, rgd := range resp.Results.RGDs {
		if rgd.Name == "postgres-db" {
			assert.Equal(t, "PostgreSQL Database", rgd.DisplayName)
			assert.Equal(t, "database", rgd.Category)
			assert.Equal(t, "Managed PostgreSQL", rgd.Description)
			found = true
		}
	}
	assert.True(t, found, "expected postgres-db in results")
}

func TestSearchHandler_ReturnsInstanceResults(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	inst := &models.Instance{
		Name:      "my-postgres",
		Namespace: "default",
		Kind:      "MyDB",
		Health:    models.HealthHealthy,
		Labels:    map[string]string{models.ProjectLabel: "alpha"},
	}
	cache.Set(inst)
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// Auth service grants access to the "default" namespace
	authService := services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &mockSearchPolicyEnforcer{
			accessibleProjects: []string{"alpha"},
		},
		NamespaceProvider: &mockSearchNamespaceProvider{
			namespaces: []string{"default"},
		},
	})

	handler := NewSearchHandler(SearchHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authService,
	})

	rec := httptest.NewRecorder()
	req := newSearchRequest("postgres", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)

	assert.NotEmpty(t, resp.Results.Instances)
	inst0 := resp.Results.Instances[0]
	assert.Equal(t, "my-postgres", inst0.Name)
	assert.Equal(t, "alpha", inst0.Project)
	assert.Equal(t, "default", inst0.Namespace)
	assert.Equal(t, "Healthy", inst0.Status)
	assert.Equal(t, "MyDB", inst0.Kind)
}

func TestSearchHandler_ReturnsProjectResults(t *testing.T) {
	t.Parallel()

	projectSvc := &mockSearchProjectService{
		projects: []rbac.Project{
			{Spec: rbac.ProjectSpec{Description: "Alpha team project"}},
			{Spec: rbac.ProjectSpec{Description: "Beta team project"}},
		},
	}
	projectSvc.projects[0].Name = "alpha"
	projectSvc.projects[1].Name = "beta"

	// Auth service grants access to both projects
	authService := services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &mockSearchPolicyEnforcer{
			accessibleProjects: []string{"alpha", "beta"},
		},
		NamespaceProvider: &mockSearchNamespaceProvider{
			namespaces: []string{"default"},
		},
	})

	handler := NewSearchHandler(SearchHandlerConfig{
		ProjectService: projectSvc,
		AuthService:    authService,
	})

	rec := httptest.NewRecorder()
	req := newSearchRequest("alpha", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)

	assert.Len(t, resp.Results.Projects, 1)
	assert.Equal(t, "alpha", resp.Results.Projects[0].Name)
	assert.Equal(t, "Alpha team project", resp.Results.Projects[0].Description)
}

func TestSearchHandler_ProjectScopedAccess(t *testing.T) {
	t.Parallel()

	projectSvc := &mockSearchProjectService{
		projects: []rbac.Project{
			{Spec: rbac.ProjectSpec{Description: "Alpha project"}},
			{Spec: rbac.ProjectSpec{Description: "Secret project"}},
		},
	}
	projectSvc.projects[0].Name = "alpha"
	projectSvc.projects[1].Name = "secret"

	// User only has access to "alpha" project
	authService := services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &mockSearchPolicyEnforcer{
			accessibleProjects: []string{"alpha"},
		},
		NamespaceProvider: &mockSearchNamespaceProvider{
			namespaces: []string{"default"},
		},
	})

	handler := NewSearchHandler(SearchHandlerConfig{
		ProjectService: projectSvc,
		AuthService:    authService,
	})

	rec := httptest.NewRecorder()
	req := newSearchRequest("project", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)

	// Should only see "alpha", not "secret"
	assert.Len(t, resp.Results.Projects, 1)
	assert.Equal(t, "alpha", resp.Results.Projects[0].Name)
}

func TestSearchHandler_ResponseIncludesQueryAndTotalCount(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(SearchHandlerConfig{})

	rec := httptest.NewRecorder()
	req := newSearchRequest("test-query", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)

	assert.Equal(t, "test-query", resp.Query)
	assert.Equal(t, 0, resp.TotalCount)
	assert.NotNil(t, resp.Results.RGDs)
	assert.NotNil(t, resp.Results.Instances)
	assert.NotNil(t, resp.Results.Projects)
}

func TestSearchHandler_NilServicesReturnEmptyArrays(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(SearchHandlerConfig{})

	rec := httptest.NewRecorder()
	req := newSearchRequest("anything", defaultTestSearchUser())

	handler.Search(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := parseSearchResponse(t, rec)

	assert.Empty(t, resp.Results.RGDs)
	assert.Empty(t, resp.Results.Instances)
	assert.Empty(t, resp.Results.Projects)
	assert.Equal(t, 0, resp.TotalCount)
}
