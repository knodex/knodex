// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCategoryService implements services.CategoryService for testing.
type mockCategoryService struct {
	categories []services.Category
}

func (m *mockCategoryService) ListCategories(_ context.Context) services.CategoryList {
	return services.CategoryList{Categories: m.categories}
}

func (m *mockCategoryService) GetCategory(_ context.Context, slug string) *services.Category {
	for i := range m.categories {
		if m.categories[i].Slug == slug {
			return &m.categories[i]
		}
	}
	return nil
}

// mockCategoriesAuthorizer implements rbac.Authorizer for categories tests.
// canAccessMap maps "object:action" → (allowed, error) for per-item control.
type mockCategoriesAuthorizer struct {
	defaultAllowed bool
	defaultErr     error
	perItem        map[string]bool // "object:action" → allowed (overrides default)
}

func (m *mockCategoriesAuthorizer) CanAccess(_ context.Context, _, _, _ string) (bool, error) {
	return m.defaultAllowed, m.defaultErr
}

func (m *mockCategoriesAuthorizer) CanAccessWithGroups(_ context.Context, _ string, _ []string, object, action string) (bool, error) {
	if m.defaultErr != nil {
		return false, m.defaultErr
	}
	if m.perItem != nil {
		key := object + ":" + action
		if allowed, ok := m.perItem[key]; ok {
			return allowed, nil
		}
	}
	return m.defaultAllowed, nil
}

func (m *mockCategoriesAuthorizer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	if !m.defaultAllowed {
		return errors.New("access denied")
	}
	return nil
}

func (m *mockCategoriesAuthorizer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

func (m *mockCategoriesAuthorizer) HasRole(_ context.Context, _, _ string) (bool, error) {
	return m.defaultAllowed, nil
}

func newCategoriesTestRequest(method, path string, userCtx *middleware.UserContext) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Request-Id", "test-request-id")
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

func sampleCategories() []services.Category {
	return []services.Category{
		{Name: "infrastructure", Slug: "infrastructure", Icon: "layout-grid", Count: 3},
		{Name: "networking", Slug: "networking", Icon: "layout-grid", Count: 2},
		{Name: "storage", Slug: "storage", Icon: "layout-grid", Count: 5},
	}
}

func sampleCategoryConfig() []services.CategoryEntry {
	return []services.CategoryEntry{
		{Name: "Infrastructure", Weight: 10},
		{Name: "Networking", Weight: 20},
		{Name: "Storage", Weight: 30},
	}
}

// =============================================================================
// ListCategories tests
// =============================================================================

func TestCategoriesHandler_ListCategories_Success_AllVisible(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result.Categories, 3)
}

func TestCategoriesHandler_ListCategories_PerCategoryFiltering(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{
		defaultAllowed: false,
		perItem: map[string]bool{
			"rgds/infrastructure/*:get": true,
			// networking and storage are denied
		},
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "operator@test.local", Groups: []string{}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 1)
	assert.Equal(t, "infrastructure", result.Categories[0].Slug)
}

func TestCategoriesHandler_ListCategories_NoCategories_WhenAllDenied(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: false}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "no-access@test.local", Groups: []string{}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Empty(t, result.Categories)
}

func TestCategoriesHandler_ListCategories_NoUserContext_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, nil, nil)

	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCategoriesHandler_ListCategories_NilEnforcer_Returns403(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	handler := NewCategoriesHandler(svc, nil, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "user@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCategoriesHandler_ListCategories_NilService_Returns500(t *testing.T) {
	t.Parallel()

	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(nil, enforcer, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "user@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// =============================================================================
// GetCategory tests
// =============================================================================

func TestCategoriesHandler_GetCategory_Success(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", userCtx)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var cat services.Category
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cat))
	assert.Equal(t, "infrastructure", cat.Slug)
	assert.Equal(t, 3, cat.Count)
}

func TestCategoriesHandler_GetCategory_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/nonexistent", userCtx)
	req.SetPathValue("slug", "nonexistent")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCategoriesHandler_GetCategory_NoSlug_Returns400(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/", userCtx)
	// No SetPathValue — slug is empty
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCategoriesHandler_GetCategory_NoUserContext_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, nil, nil)

	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", nil)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCategoriesHandler_GetCategory_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: false}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "viewer@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", userCtx)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCategoriesHandler_GetCategory_NilService_Returns500(t *testing.T) {
	t.Parallel()

	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(nil, enforcer, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "user@test.local"}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", userCtx)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// =============================================================================
// New tests — Task 8
// =============================================================================

func TestCategoriesHandler_ListCategories_NilCategoryConfig_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Empty(t, result.Categories)
}

func TestCategoriesHandler_ListCategories_FiltersByConfigMap(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Infrastructure", Weight: 10},
		{Name: "Storage", Weight: 30},
		// Networking excluded
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 2)
	slugs := []string{result.Categories[0].Slug, result.Categories[1].Slug}
	assert.Contains(t, slugs, "infrastructure")
	assert.Contains(t, slugs, "storage")
	for _, cat := range result.Categories {
		assert.NotEqual(t, "networking", cat.Slug)
	}
}

func TestCategoriesHandler_ListCategories_SortsByWeight(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Storage", Weight: 10},
		{Name: "Infrastructure", Weight: 5},
		{Name: "Networking", Weight: 20},
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 3)
	assert.Equal(t, "infrastructure", result.Categories[0].Slug) // weight 5
	assert.Equal(t, "storage", result.Categories[1].Slug)        // weight 10
	assert.Equal(t, "networking", result.Categories[2].Slug)     // weight 20
}

func TestCategoriesHandler_ListCategories_EqualWeights_SortedAlphabetically(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Storage", Weight: 10},
		{Name: "Infrastructure", Weight: 10},
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 2)
	assert.Equal(t, "infrastructure", result.Categories[0].Slug) // alphabetical tiebreak
	assert.Equal(t, "storage", result.Categories[1].Slug)
}

func TestCategoriesHandler_ListCategories_IconOverrideFromConfig(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Infrastructure", Weight: 10, Icon: "server"},
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 1)
	assert.Equal(t, "server", result.Categories[0].Icon)
	assert.Equal(t, "lucide", result.Categories[0].IconType)
}

func TestCategoriesHandler_ListCategories_IconFallsBackWhenNoConfigIcon(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Infrastructure", Weight: 10}, // no Icon
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil) // iconRegistry nil

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Categories, 1)
	assert.Equal(t, "lucide", result.Categories[0].IconType) // non-empty
}

func TestCategoriesHandler_ListCategories_EmptyService_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: []services.Category{}}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories", userCtx)
	rec := httptest.NewRecorder()

	handler.ListCategories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.CategoryList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Empty(t, result.Categories)
}

func TestCategoriesHandler_GetCategory_NilCategoryConfig_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, nil, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", userCtx)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCategoriesHandler_GetCategory_SlugNotInConfig_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryService{categories: sampleCategories()}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	config := []services.CategoryEntry{
		{Name: "Infrastructure", Weight: 10},
		// no networking
	}
	handler := NewCategoriesHandler(svc, enforcer, nil, config, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/networking", userCtx)
	req.SetPathValue("slug", "networking")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// mockCategoryServiceWithOverride overrides GetCategory for a specific slug.
type mockCategoryServiceWithOverride struct {
	mockCategoryService
	overrideSlug   string
	overrideResult *services.Category
}

func (m *mockCategoryServiceWithOverride) GetCategory(ctx context.Context, slug string) *services.Category {
	if slug == m.overrideSlug {
		return m.overrideResult
	}
	return m.mockCategoryService.GetCategory(ctx, slug)
}

func TestCategoriesHandler_GetCategory_InConfigButServiceReturnsNil_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockCategoryServiceWithOverride{
		mockCategoryService: mockCategoryService{categories: sampleCategories()},
		overrideSlug:        "infrastructure",
		overrideResult:      nil, // service returns nil for this slug
	}
	enforcer := &mockCategoriesAuthorizer{defaultAllowed: true}
	handler := NewCategoriesHandler(svc, enforcer, nil, sampleCategoryConfig(), nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}}
	req := newCategoriesTestRequest(http.MethodGet, "/api/v1/categories/infrastructure", userCtx)
	req.SetPathValue("slug", "infrastructure")
	rec := httptest.NewRecorder()

	handler.GetCategory(rec, req)

	// 404 from h.service.GetCategory returning nil, before ConfigMap gate
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
