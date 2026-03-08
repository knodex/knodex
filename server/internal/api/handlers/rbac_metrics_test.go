package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
)

// mockPolicyEnforcerForMetrics implements rbac.PolicyEnforcer for testing
type mockPolicyEnforcerForMetrics struct {
	cacheStats      rbac.CacheStats
	metrics         rbac.PolicyMetrics
	canAccessResult bool // controls what CanAccess/CanAccessWithGroups returns
}

func (m *mockPolicyEnforcerForMetrics) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	return m.canAccessResult, nil
}

func (m *mockPolicyEnforcerForMetrics) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return nil, nil
}

func (m *mockPolicyEnforcerForMetrics) HasRole(ctx context.Context, user, role string) (bool, error) {
	return false, nil
}

func (m *mockPolicyEnforcerForMetrics) RemoveUserRoles(ctx context.Context, user string) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) RemoveUserRole(ctx context.Context, user, role string) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return nil
}

func (m *mockPolicyEnforcerForMetrics) InvalidateCache() {
}

func (m *mockPolicyEnforcerForMetrics) IncrementPolicyReloads() {
}

func (m *mockPolicyEnforcerForMetrics) IncrementWatcherRestarts() {
}

func (m *mockPolicyEnforcerForMetrics) IncrementBackgroundSyncs() {
}

func (m *mockPolicyEnforcerForMetrics) CacheStats() rbac.CacheStats {
	return m.cacheStats
}

func (m *mockPolicyEnforcerForMetrics) Metrics() rbac.PolicyMetrics {
	return m.metrics
}

// CanAccessWithGroups implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcerForMetrics) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.canAccessResult, nil
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcerForMetrics) InvalidateCacheForUser(user string) int {
	return 0
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcerForMetrics) InvalidateCacheForProject(projectName string) int {
	return 0
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcerForMetrics) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	return nil, nil
}

// setAdminContext adds an admin user context to the request
func setAdminContext(req *http.Request) *http.Request {
	userCtx := &middleware.UserContext{
		UserID: "user-local-admin",
		Email:  "admin@local",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	return req.WithContext(ctx)
}

// setReadonlyContext adds a readonly user context to the request
func setReadonlyContext(req *http.Request) *http.Request {
	userCtx := &middleware.UserContext{
		UserID: "user-readonly",
		Email:  "readonly@example.com",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	return req.WithContext(ctx)
}

func TestNewRBACMetricsHandler(t *testing.T) {
	enforcer := &mockPolicyEnforcerForMetrics{}
	handler := NewRBACMetricsHandler(enforcer, nil)

	if handler == nil {
		t.Fatal("expected handler to be created")
	}
}

func TestRBACMetricsHandler_GetMetrics_WithEnforcer(t *testing.T) {
	enforcer := &mockPolicyEnforcerForMetrics{
		cacheStats: rbac.CacheStats{
			Hits:    100,
			Misses:  20,
			HitRate: 83.33,
			Size:    50,
		},
		metrics: rbac.PolicyMetrics{
			PolicyReloads:   5,
			BackgroundSyncs: 10,
			WatcherRestarts: 2,
		},
		canAccessResult: true,
	}
	handler := NewRBACMetricsHandler(enforcer, nil)

	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	req = setAdminContext(req)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var response RBACMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify cache stats
	if response.Cache.Hits != 100 {
		t.Errorf("expected 100 hits, got %d", response.Cache.Hits)
	}
	if response.Cache.Misses != 20 {
		t.Errorf("expected 20 misses, got %d", response.Cache.Misses)
	}
	if response.Cache.HitRate != 83.33 {
		t.Errorf("expected hit rate 83.33, got %.2f", response.Cache.HitRate)
	}

	// Verify policy metrics
	if response.Policy.PolicyReloads != 5 {
		t.Errorf("expected 5 policy reloads, got %d", response.Policy.PolicyReloads)
	}
	if response.Policy.BackgroundSyncs != 10 {
		t.Errorf("expected 10 background syncs, got %d", response.Policy.BackgroundSyncs)
	}

	// CacheManager should be nil
	if response.CacheManager != nil {
		t.Error("expected CacheManager to be nil when not configured")
	}
}

func TestRBACMetricsHandler_GetMetrics_WithCacheManager(t *testing.T) {
	enforcer := &mockPolicyEnforcerForMetrics{
		cacheStats: rbac.CacheStats{
			Hits:    50,
			Misses:  10,
			HitRate: 83.33,
			Size:    25,
		},
		metrics: rbac.PolicyMetrics{
			PolicyReloads:   3,
			BackgroundSyncs: 6,
			WatcherRestarts: 1,
		},
		canAccessResult: true,
	}

	// Create a real PolicyCacheManager for testing
	cacheManager := rbac.NewPolicyCacheManager(enforcer, nil, nil, nil)

	handler := NewRBACMetricsHandler(enforcer, cacheManager)

	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	req = setAdminContext(req)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var response RBACMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// CacheManager status should be present
	if response.CacheManager == nil {
		t.Fatal("expected CacheManager to be present")
	}

	if !response.CacheManager.CacheEnabled {
		t.Error("expected CacheEnabled to be true")
	}
}

func TestRBACMetricsHandler_GetMetrics_Unauthorized(t *testing.T) {
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: true}
	handler := NewRBACMetricsHandler(enforcer, nil)

	// Request without user context should get 401
	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestRBACMetricsHandler_GetMetrics_Forbidden(t *testing.T) {
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: false}
	handler := NewRBACMetricsHandler(enforcer, nil)

	// Request with user context but no admin permission should get 403
	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	req = setReadonlyContext(req)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestRBACMetricsHandler_GetMetrics_NilEnforcer(t *testing.T) {
	// With nil enforcer, authorization fails closed (403)
	handler := NewRBACMetricsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	req = setReadonlyContext(req)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	resp := w.Result()
	// With nil enforcer, RequireAccess returns false (fail closed) → 403
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 with nil enforcer, got %d", resp.StatusCode)
	}
}

func TestRBACMetricsResponse_Serialization(t *testing.T) {
	now := time.Now()
	response := RBACMetricsResponse{
		Cache: rbac.CacheStats{
			Hits:    100,
			Misses:  20,
			HitRate: 83.33,
			Size:    50,
		},
		Policy: rbac.PolicyMetrics{
			PolicyReloads:   5,
			BackgroundSyncs: 10,
			WatcherRestarts: 2,
		},
		CacheManager: &rbac.PolicyCacheStatus{
			CacheEnabled:    true,
			WatcherRunning:  true,
			WatcherLastSync: now,
			SyncerRunning:   true,
			SyncerLastSync:  now,
			PolicyReloads:   5,
			BackgroundSyncs: 10,
			WatcherRestarts: 2,
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Deserialize back
	var decoded RBACMetricsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify fields preserved
	if decoded.Cache.Hits != 100 {
		t.Errorf("expected 100 hits after serialization, got %d", decoded.Cache.Hits)
	}
	if decoded.Policy.PolicyReloads != 5 {
		t.Errorf("expected 5 policy reloads after serialization, got %d", decoded.Policy.PolicyReloads)
	}
	if decoded.CacheManager == nil {
		t.Fatal("expected CacheManager after serialization")
	}
	if !decoded.CacheManager.CacheEnabled {
		t.Error("expected CacheEnabled after serialization")
	}
}

// Benchmark tests
func BenchmarkRBACMetricsHandler_GetMetrics(b *testing.B) {
	enforcer := &mockPolicyEnforcerForMetrics{
		cacheStats: rbac.CacheStats{
			Hits:    100,
			Misses:  20,
			HitRate: 83.33,
			Size:    50,
		},
		metrics: rbac.PolicyMetrics{
			PolicyReloads:   5,
			BackgroundSyncs: 10,
			WatcherRestarts: 2,
		},
		canAccessResult: true,
	}
	handler := NewRBACMetricsHandler(enforcer, nil)

	req := httptest.NewRequest("GET", "/api/v1/rbac/metrics", nil)
	req = setAdminContext(req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetMetrics(w, req)
	}
}
