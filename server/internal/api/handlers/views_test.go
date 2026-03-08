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

// mockViewsService implements services.ViewsService for testing.
type mockViewsService struct {
	enabled bool
	views   []services.View
}

func (m *mockViewsService) IsEnabled() bool { return m.enabled }

func (m *mockViewsService) ListViews(_ context.Context) services.ViewList {
	return services.ViewList{Views: m.views}
}

func (m *mockViewsService) GetView(slug string) *services.View {
	for i := range m.views {
		if m.views[i].Slug == slug {
			return &m.views[i]
		}
	}
	return nil
}

// mockViewsAuthorizer implements rbac.Authorizer for views tests.
type mockViewsAuthorizer struct {
	canAccessResult bool
	canAccessErr    error
}

func (m *mockViewsAuthorizer) CanAccess(_ context.Context, _, _, _ string) (bool, error) {
	return m.canAccessResult, m.canAccessErr
}

func (m *mockViewsAuthorizer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return m.canAccessResult, m.canAccessErr
}

func (m *mockViewsAuthorizer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	if !m.canAccessResult {
		return errors.New("access denied")
	}
	return nil
}

func (m *mockViewsAuthorizer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

func (m *mockViewsAuthorizer) HasRole(_ context.Context, _, _ string) (bool, error) {
	return m.canAccessResult, nil
}

// mockViewsLicenseService implements services.LicenseService for views tests.
type mockViewsLicenseService struct {
	licensed       bool
	featureEnabled bool
	gracePeriod    bool
	readOnly       bool
	hasFeature     bool
}

func (m *mockViewsLicenseService) IsLicensed() bool                   { return m.licensed }
func (m *mockViewsLicenseService) IsFeatureEnabled(_ string) bool     { return m.featureEnabled }
func (m *mockViewsLicenseService) IsGracePeriod() bool                { return m.gracePeriod }
func (m *mockViewsLicenseService) IsReadOnly() bool                   { return m.readOnly }
func (m *mockViewsLicenseService) HasFeature(_ string) bool           { return m.hasFeature }
func (m *mockViewsLicenseService) GetLicense() *services.LicenseInfo  { return nil }
func (m *mockViewsLicenseService) GetStatus() *services.LicenseStatus { return nil }
func (m *mockViewsLicenseService) UpdateLicense(_ string) error       { return nil }

func newViewsTestRequest(method, path string, userCtx *middleware.UserContext) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Request-Id", "test-request-id")
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

func sampleViews() []services.View {
	return []services.View{
		{Name: "Networking", Slug: "networking", Icon: "network", Category: "networking", Order: 1, Count: 3},
		{Name: "Storage", Slug: "storage", Icon: "database", Category: "storage", Order: 2, Count: 5},
	}
}

// =============================================================================
// ListViews tests
// =============================================================================

func TestViewsHandler_ListViews_Success(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"admins"}}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result services.ViewList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result.Views, 2)
	assert.Equal(t, "networking", result.Views[0].Slug)
}

func TestViewsHandler_ListViews_NoUserContext_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", nil) // no user context
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestViewsHandler_ListViews_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: false}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "viewer@test.local", Groups: []string{}}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestViewsHandler_ListViews_PermissionError_Returns500(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessErr: errors.New("policy check failed")}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "user@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestViewsHandler_ListViews_NilEnforcer_Returns403(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	handler := NewViewsHandler(svc, nil, nil) // nil enforcer = fail closed

	userCtx := &middleware.UserContext{UserID: "user@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestViewsHandler_ListViews_NilService_Returns503(t *testing.T) {
	t.Parallel()

	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(nil, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestViewsHandler_ListViews_LicenseRequired_Returns402(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)
	handler.SetLicenseService(&mockViewsLicenseService{
		licensed:       false,
		featureEnabled: false,
	})

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
}

func TestViewsHandler_ListViews_NoLicenseService_SkipsCheck(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)
	// No SetLicenseService call — licenseService is nil

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// =============================================================================
// GetView tests
// =============================================================================

func TestViewsHandler_GetView_Success(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local", Groups: []string{"admins"}}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views/networking", userCtx)
	req.SetPathValue("slug", "networking")
	rec := httptest.NewRecorder()

	handler.GetView(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var view services.View
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&view))
	assert.Equal(t, "networking", view.Slug)
	assert.Equal(t, "Networking", view.Name)
	assert.Equal(t, 3, view.Count)
}

func TestViewsHandler_GetView_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views/nonexistent", userCtx)
	req.SetPathValue("slug", "nonexistent")
	rec := httptest.NewRecorder()

	handler.GetView(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestViewsHandler_GetView_NoSlug_Returns400(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views/", userCtx)
	// No SetPathValue — slug is empty
	rec := httptest.NewRecorder()

	handler.GetView(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestViewsHandler_GetView_NoUserContext_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)

	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views/networking", nil)
	req.SetPathValue("slug", "networking")
	rec := httptest.NewRecorder()

	handler.GetView(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestViewsHandler_GetView_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: false}
	handler := NewViewsHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{UserID: "viewer@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views/networking", userCtx)
	req.SetPathValue("slug", "networking")
	rec := httptest.NewRecorder()

	handler.GetView(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// =============================================================================
// License integration tests
// =============================================================================

func TestViewsHandler_ListViews_GracePeriod_SetsWarningHeader(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)
	handler.SetLicenseService(&mockViewsLicenseService{
		licensed:       true,
		featureEnabled: true,
		gracePeriod:    true,
	})

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "expired", rec.Header().Get("X-License-Warning"))
}

func TestViewsHandler_ListViews_ReadOnlyMode_AllowsAccess(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)
	handler.SetLicenseService(&mockViewsLicenseService{
		licensed:       false,
		featureEnabled: false,
		readOnly:       true,
		hasFeature:     true,
	})

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestViewsHandler_ListViews_FeatureNotInLicense_Returns402(t *testing.T) {
	t.Parallel()

	svc := &mockViewsService{enabled: true, views: sampleViews()}
	enforcer := &mockViewsAuthorizer{canAccessResult: true}
	handler := NewViewsHandler(svc, enforcer, nil)
	handler.SetLicenseService(&mockViewsLicenseService{
		licensed:       true,
		featureEnabled: false, // licensed but views feature not included
	})

	userCtx := &middleware.UserContext{UserID: "admin@test.local"}
	req := newViewsTestRequest(http.MethodGet, "/api/v1/ee/views", userCtx)
	rec := httptest.NewRecorder()

	handler.ListViews(rec, req)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
}
