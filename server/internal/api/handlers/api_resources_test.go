// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/fake"
	faketesting "k8s.io/client-go/testing"
)

func TestAPIResourcesHandler_ListAPIResources(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		resources      []*metav1.APIResourceList
		wantStatus     int
		wantMinCount   int
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:        "returns all resources",
			queryParams: map[string]string{},
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1", // core API group
					APIResources: []metav1.APIResource{
						{Name: "pods", Kind: "Pod"},
						{Name: "services", Kind: "Service"},
						{Name: "pods/log", Kind: "Pod"}, // subresource - should be skipped
					},
				},
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "deployments", Kind: "Deployment"},
						{Name: "statefulsets", Kind: "StatefulSet"},
					},
				},
			},
			wantStatus:   http.StatusOK,
			wantMinCount: 4,
			wantContains: []string{"Pod", "Service", "Deployment", "StatefulSet"},
		},
		{
			name:        "search filter by kind",
			queryParams: map[string]string{"search": "pod"},
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Kind: "Pod"},
						{Name: "services", Kind: "Service"},
					},
				},
			},
			wantStatus:     http.StatusOK,
			wantMinCount:   1,
			wantContains:   []string{"Pod"},
			wantNotContain: []string{"Service"},
		},
		{
			name:        "filter by API group",
			queryParams: map[string]string{"apiGroup": "apps"},
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Kind: "Pod"},
					},
				},
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "deployments", Kind: "Deployment"},
					},
				},
			},
			wantStatus:     http.StatusOK,
			wantMinCount:   1,
			wantContains:   []string{"Deployment"},
			wantNotContain: []string{"Pod"},
		},
		{
			name:        "filter by core API group using 'core' alias",
			queryParams: map[string]string{"apiGroup": "core"},
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Kind: "Pod"},
					},
				},
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "deployments", Kind: "Deployment"},
					},
				},
			},
			wantStatus:     http.StatusOK,
			wantMinCount:   1,
			wantContains:   []string{"Pod"},
			wantNotContain: []string{"Deployment"},
		},
		{
			name:        "case insensitive search",
			queryParams: map[string]string{"search": "DEPLOY"},
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "deployments", Kind: "Deployment"},
						{Name: "statefulsets", Kind: "StatefulSet"},
					},
				},
			},
			wantStatus:     http.StatusOK,
			wantMinCount:   1,
			wantContains:   []string{"Deployment"},
			wantNotContain: []string{"StatefulSet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake discovery client
			fakeDiscovery := &fake.FakeDiscovery{
				Fake: &faketesting.Fake{},
			}

			// Set up the fake resources
			fakeDiscovery.Resources = tt.resources

			// Create fresh handler for each test (avoids cache interference)
			handler := NewAPIResourcesHandler(fakeDiscovery)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.ListAPIResources(rr, req)

			// Check status code
			if rr.Code != tt.wantStatus {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatus)
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			// Parse response
			var resp APIResourcesResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			// Check minimum count
			if resp.Count < tt.wantMinCount {
				t.Errorf("count = %v, want at least %v", resp.Count, tt.wantMinCount)
			}

			// Check contains
			for _, wantKind := range tt.wantContains {
				found := false
				for _, r := range resp.Resources {
					if r.Kind == wantKind {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("response should contain kind %q", wantKind)
				}
			}

			// Check not contains
			for _, wantNotKind := range tt.wantNotContain {
				for _, r := range resp.Resources {
					if r.Kind == wantNotKind {
						t.Errorf("response should not contain kind %q", wantNotKind)
					}
				}
			}
		})
	}
}

func TestAPIResourcesHandler_NilDiscoveryClient(t *testing.T) {
	// Create handler with nil discovery client
	handler := NewAPIResourcesHandler(nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ListAPIResources(rr, req)

	// Should return 503 Service Unavailable
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %v, want %v", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestAPIResourcesHandler_Caching(t *testing.T) {
	// Create fake discovery client
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}

	// Set up initial resources
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod"},
			},
		},
	}

	// Create handler
	handler := NewAPIResourcesHandler(fakeDiscovery)

	// First request - populates cache
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)
	rr1 := httptest.NewRecorder()
	handler.ListAPIResources(rr1, req1)

	var resp1 APIResourcesResponse
	if err := json.Unmarshal(rr1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("failed to parse first response: %v", err)
	}

	// Change resources (but cache should still be used)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod"},
				{Name: "services", Kind: "Service"},
			},
		},
	}

	// Second request - should use cache
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)
	rr2 := httptest.NewRecorder()
	handler.ListAPIResources(rr2, req2)

	var resp2 APIResourcesResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("failed to parse second response: %v", err)
	}

	// Both responses should be the same (cache was used)
	if resp1.Count != resp2.Count {
		t.Errorf("cache not used: first count = %d, second count = %d", resp1.Count, resp2.Count)
	}
}

func TestAPIResourcesHandler_CoreAPIGroupDisplayed(t *testing.T) {
	// Create fake discovery client
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}

	// Set up resources with core API group
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1", // core API group has no group prefix
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod"},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment"},
			},
		},
	}

	// Create handler
	handler := NewAPIResourcesHandler(fakeDiscovery)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)
	rr := httptest.NewRecorder()

	// Call handler
	handler.ListAPIResources(rr, req)

	// Parse response
	var resp APIResourcesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check that core API group is represented with empty string
	var foundPod, foundDeployment bool
	for _, r := range resp.Resources {
		if r.Kind == "Pod" {
			foundPod = true
			if r.APIGroup != "" {
				t.Errorf("Pod should have empty apiGroup (core), got %q", r.APIGroup)
			}
		}
		if r.Kind == "Deployment" {
			foundDeployment = true
			if r.APIGroup != "apps" {
				t.Errorf("Deployment should have apiGroup 'apps', got %q", r.APIGroup)
			}
		}
	}

	if !foundPod {
		t.Error("Pod not found in response")
	}
	if !foundDeployment {
		t.Error("Deployment not found in response")
	}
}

func TestAPIResourcesHandler_SubresourcesFiltered(t *testing.T) {
	// Create fake discovery client
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}

	// Set up resources including subresources
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod"},
				{Name: "pods/log", Kind: "Pod"},    // subresource
				{Name: "pods/exec", Kind: "Pod"},   // subresource
				{Name: "pods/status", Kind: "Pod"}, // subresource
				{Name: "services", Kind: "Service"},
			},
		},
	}

	// Create handler
	handler := NewAPIResourcesHandler(fakeDiscovery)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/api-resources", nil)
	rr := httptest.NewRecorder()

	// Call handler
	handler.ListAPIResources(rr, req)

	// Parse response
	var resp APIResourcesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should only have 2 resources (Pod and Service), no subresources
	if resp.Count != 2 {
		t.Errorf("count = %d, want 2 (subresources should be filtered)", resp.Count)
	}
}

// mockDiscoveryClient implements discovery.DiscoveryInterface for testing
type mockDiscoveryClient struct {
	discovery.DiscoveryInterface
	resources []*metav1.APIResourceList
	err       error
}

func (m *mockDiscoveryClient) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, m.resources, m.err
}
