// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/rbac"
)

// withUserCtx adds a middleware.UserContext to the request (test helper).
func withUserCtx(r *http.Request, userID string, groups ...string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID: userID,
		Groups: groups,
	})
	return r.WithContext(ctx)
}

// newTestResourceAggHandler creates a handler with the given mocks and a real RemoteWatcher cache.
func newTestResourceAggHandler(ps *mockProjectService, enforcer *mockPolicyEnforcer, rw *watcher.RemoteWatcher) *ResourceAggregationHandler {
	return NewResourceAggregationHandler(ps, enforcer, rw)
}

// newTestRemoteWatcher creates a RemoteWatcher for testing (never started, only cache used).
func newTestRemoteWatcher() *watcher.RemoteWatcher {
	return watcher.NewRemoteWatcher(nil, nil)
}

// resourceReq creates a test request with optional query string.
func resourceReq(method, path, query string) *http.Request {
	url := path
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(method, url, nil)
	return req
}

// makeProject creates a project for testing with the given destinations.
func makeProject(name string, destinations []rbac.Destination) *rbac.Project {
	return &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: rbac.ProjectSpec{
			Destinations: destinations,
		},
	}
}

// Test 4.1: valid Certificate query returns empty results (remote aggregation disabled)
func TestListProjectResources_CertificateReturnsEmpty(t *testing.T) {
	ps := newMockProjectService()
	ps.projects["team-alpha"] = makeProject("team-alpha", []rbac.Destination{
		{Namespace: "team-alpha-ns"},
	})

	rw := newTestRemoteWatcher()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "kind=Certificate")
	req.SetPathValue("name", "team-alpha")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp ResourceAggregationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	// Remote resource aggregation is disabled; cluster refs are empty
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 (remote aggregation disabled)", resp.TotalCount)
	}
}

// Test 4.2: valid Ingress query returns empty results (remote aggregation disabled)
func TestListProjectResources_Ingress(t *testing.T) {
	ps := newMockProjectService()
	ps.projects["team-alpha"] = makeProject("team-alpha", []rbac.Destination{
		{Namespace: "team-alpha-ns"},
	})

	rw := newTestRemoteWatcher()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "kind=Ingress")
	req.SetPathValue("name", "team-alpha")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp ResourceAggregationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Remote resource aggregation is disabled; cluster refs are empty
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 (remote aggregation disabled)", resp.TotalCount)
	}
}

// Test 4.3: project with destinations returns empty results (remote aggregation disabled)
func TestListProjectResources_DestinationScoped(t *testing.T) {
	ps := newMockProjectService()
	ps.projects["platform"] = makeProject("platform", []rbac.Destination{
		{Namespace: "cert-manager"},
	})

	rw := newTestRemoteWatcher()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/platform/resources", "kind=Certificate")
	req.SetPathValue("name", "platform")
	req = withUserCtx(req, "admin@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp ResourceAggregationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Remote resource aggregation is disabled; cluster refs are empty
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 (remote aggregation disabled)", resp.TotalCount)
	}
}

// Test: missing kind query param returns 400 with clear message
func TestListProjectResources_MissingKind(t *testing.T) {
	ps := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "")
	req.SetPathValue("name", "team-alpha")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var errResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&errResp)
	msg, _ := errResp["message"].(string)
	if msg != "missing required query parameter: kind" {
		t.Errorf("message = %q, want 'missing required query parameter: kind'", msg)
	}
}

// Test 4.6: unsupported kind returns 400 with supported kinds list
func TestListProjectResources_UnsupportedKind(t *testing.T) {
	ps := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "kind=Deployment")
	req.SetPathValue("name", "team-alpha")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var errResp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&errResp)
	msg, _ := errResp["message"].(string)
	if msg == "" {
		t.Error("expected error message")
	}
}

// Test 4.7: missing instances/get permission returns 403
func TestListProjectResources_Forbidden(t *testing.T) {
	ps := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "kind=Certificate")
	req.SetPathValue("name", "team-alpha")
	req = withUserCtx(req, "viewer@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// Test 4.8: non-existent project returns 404
func TestListProjectResources_ProjectNotFound(t *testing.T) {
	ps := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/nonexistent/resources", "kind=Certificate")
	req.SetPathValue("name", "nonexistent")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// Test 4.9: project returns empty items (remote aggregation disabled)
func TestListProjectResources_EmptyResults(t *testing.T) {
	ps := newMockProjectService()
	ps.projects["mono"] = makeProject("mono", []rbac.Destination{
		{Namespace: "mono-ns"},
	})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/mono/resources", "kind=Certificate")
	req.SetPathValue("name", "mono")
	req = withUserCtx(req, "dev@test.local")
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp ResourceAggregationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
	if len(resp.Items) != 0 {
		t.Errorf("Items len = %d, want 0", len(resp.Items))
	}
	if resp.ClusterStatus != nil {
		t.Errorf("ClusterStatus should be nil, got %v", resp.ClusterStatus)
	}
}

// Test 4.10: unauthenticated request returns 401
func TestListProjectResources_Unauthenticated(t *testing.T) {
	ps := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	rw := newTestRemoteWatcher()
	h := newTestResourceAggHandler(ps, enforcer, rw)

	req := resourceReq(http.MethodGet, "/api/v1/projects/team-alpha/resources", "kind=Certificate")
	req.SetPathValue("name", "team-alpha")
	// No user context set
	w := httptest.NewRecorder()

	h.ListProjectResources(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
