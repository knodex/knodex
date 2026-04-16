// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
)

// testE2EAdminPolicyEnforcer implements rbac.PolicyEnforcer for E2E tests.
// All authorization checks grant access (global admin).
type testE2EAdminPolicyEnforcer struct{}

// Authorizer methods
func (t *testE2EAdminPolicyEnforcer) CanAccess(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}
func (t *testE2EAdminPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return true, nil
}
func (t *testE2EAdminPolicyEnforcer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	return nil
}
func (t *testE2EAdminPolicyEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil // nil = global admin (all projects)
}
func (t *testE2EAdminPolicyEnforcer) HasRole(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

// PolicyLoader methods
func (t *testE2EAdminPolicyEnforcer) LoadProjectPolicies(_ context.Context, _ *rbac.Project) error {
	return nil
}
func (t *testE2EAdminPolicyEnforcer) SyncPolicies(_ context.Context) error { return nil }
func (t *testE2EAdminPolicyEnforcer) RemoveProjectPolicies(_ context.Context, _ string) error {
	return nil
}

// RoleManager methods
func (t *testE2EAdminPolicyEnforcer) AssignUserRoles(_ context.Context, _ string, _ []string) error {
	return nil
}
func (t *testE2EAdminPolicyEnforcer) GetUserRoles(_ context.Context, _ string) ([]string, error) {
	return []string{"role:serveradmin"}, nil
}
func (t *testE2EAdminPolicyEnforcer) RemoveUserRoles(_ context.Context, _ string) error { return nil }
func (t *testE2EAdminPolicyEnforcer) RemoveUserRole(_ context.Context, _, _ string) error {
	return nil
}
func (t *testE2EAdminPolicyEnforcer) RestorePersistedRoles(_ context.Context) error { return nil }

// CacheController methods
func (t *testE2EAdminPolicyEnforcer) InvalidateCache()                       {}
func (t *testE2EAdminPolicyEnforcer) InvalidateCacheForUser(_ string) int    { return 0 }
func (t *testE2EAdminPolicyEnforcer) InvalidateCacheForProject(_ string) int { return 0 }
func (t *testE2EAdminPolicyEnforcer) CacheStats() rbac.CacheStats            { return rbac.CacheStats{} }

// MetricsProvider methods
func (t *testE2EAdminPolicyEnforcer) Metrics() rbac.PolicyMetrics { return rbac.PolicyMetrics{} }
func (t *testE2EAdminPolicyEnforcer) IncrementPolicyReloads()     {}
func (t *testE2EAdminPolicyEnforcer) IncrementBackgroundSyncs()   {}
func (t *testE2EAdminPolicyEnforcer) IncrementWatcherRestarts()   {}

// testGlobalAdminMiddleware injects a global admin user context into all requests.
// This is used for testing endpoints that require authentication after auth changes
// security fix which correctly returns no instances for unauthenticated requests.
func testGlobalAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx := &middleware.UserContext{
			UserID:      "test-global-admin",
			Email:       "admin@test.local",
			DisplayName: "Test Global Admin",
			CasbinRoles: []string{"role:serveradmin"},
		}
		ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// setupE2ETestServer creates a test server with realistic RGD data including
// apiVersion, kind, and dependencies for e2e testing scenarios
func setupE2ETestServer(t *testing.T) (*httptest.Server, *watcher.RGDCache) {
	cache := watcher.NewRGDCache()

	// Test RGDs with full schema information including apiVersion and kind
	// All must have catalog annotation to be visible
	testRGDs := []models.CatalogRGD{
		{
			Name:        "postgres-cluster",
			Namespace:   "default",
			Description: "PostgreSQL cluster with high availability",
			Tags:        []string{"database", "production"},
			Category:    "database",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "PostgresCluster",
			Labels:      map[string]string{"tier": "backend"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "PostgresCluster",
				},
			},
		},
		{
			Name:        "fullstack-app",
			Namespace:   "default",
			Description: "Full-stack application with database dependency",
			Tags:        []string{"webapp", "production"},
			Category:    "compute",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "FullStackApp",
			Labels:      map[string]string{"tier": "frontend"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "FullStackApp",
				},
				"resources": []interface{}{
					map[string]interface{}{
						"id": "backend",
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"env": []interface{}{
									map[string]interface{}{
										"value": "${postgres-cluster.status.connectionString}",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:        "webapp",
			Namespace:   "default",
			Description: "Simple web application",
			Tags:        []string{"webapp"},
			Category:    "compute",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "WebApp",
			Labels:      map[string]string{},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-6 * time.Hour),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "WebApp",
				},
			},
		},
	}

	for i := range testRGDs {
		cache.Set(&testRGDs[i])
	}

	// Create watcher with test cache
	w := watcher.NewRGDWatcherWithCache(cache)

	// Create health checker
	checker := health.NewChecker(nil, nil, w)

	// Create router (nil for instanceTracker, schema extractor, and wsHub - we'll test without them)
	routerResult := api.NewRouterWithConfig(checker, w, nil, nil, api.RouterConfig{
		PolicyEnforcer: &testE2EAdminPolicyEnforcer{},
	})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})

	server := httptest.NewServer(routerResult.Handler)
	return server, cache
}

func TestE2E_RGDContainsAPIVersionAndKind(t *testing.T) {
	server, _ := setupE2ETestServer(t)
	defer server.Close()

	tests := []struct {
		name               string
		rgdName            string
		expectedAPIVersion string
		expectedKind       string
	}{
		{
			name:               "postgres-cluster has correct apiVersion and kind",
			rgdName:            "postgres-cluster",
			expectedAPIVersion: "kro.run/v1alpha1",
			expectedKind:       "PostgresCluster",
		},
		{
			name:               "fullstack-app has correct apiVersion and kind",
			rgdName:            "fullstack-app",
			expectedAPIVersion: "kro.run/v1alpha1",
			expectedKind:       "FullStackApp",
		},
		{
			name:               "webapp has correct apiVersion and kind",
			rgdName:            "webapp",
			expectedAPIVersion: "kro.run/v1alpha1",
			expectedKind:       "WebApp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + "/api/v1/rgds/" + tt.rgdName)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			var rgdResp services.RGDResponse
			if err := json.NewDecoder(resp.Body).Decode(&rgdResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if rgdResp.APIVersion != tt.expectedAPIVersion {
				t.Errorf("expected apiVersion %q, got %q", tt.expectedAPIVersion, rgdResp.APIVersion)
			}

			if rgdResp.Kind != tt.expectedKind {
				t.Errorf("expected kind %q, got %q", tt.expectedKind, rgdResp.Kind)
			}
		})
	}
}

func TestE2E_ListRGDsIncludesAPIVersionAndKind(t *testing.T) {
	server, _ := setupE2ETestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var listResp handlers.ListRGDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// All RGDs should have apiVersion and kind
	for _, rgd := range listResp.Items {
		if rgd.APIVersion == "" {
			t.Errorf("RGD %s has empty apiVersion", rgd.Name)
		}
		if rgd.Kind == "" {
			t.Errorf("RGD %s has empty kind", rgd.Name)
		}
	}
}

func TestE2E_CategoryBasedFiltering(t *testing.T) {
	server, _ := setupE2ETestServer(t)
	defer server.Close()

	tests := []struct {
		name          string
		category      string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "filter database category",
			category:      "database",
			expectedCount: 1,
			expectedNames: []string{"postgres-cluster"},
		},
		{
			name:          "filter compute category",
			category:      "compute",
			expectedCount: 2,
			expectedNames: []string{"fullstack-app", "webapp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + "/api/v1/rgds?category=" + tt.category)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			var listResp handlers.ListRGDsResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(listResp.Items) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(listResp.Items))
			}

			for _, expectedName := range tt.expectedNames {
				found := false
				for _, rgd := range listResp.Items {
					if rgd.Name == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %s in results", expectedName)
				}
			}
		})
	}
}

// E2E tests for Resource Graph API

// setupE2EResourceGraphTestServer creates a test server with RGDs containing
// various resource configurations for testing resource graph scenarios
func setupE2EResourceGraphTestServer(t *testing.T) (*httptest.Server, *watcher.RGDCache) {
	cache := watcher.NewRGDCache()

	// Test RGDs with full resource definitions - all must have catalog annotation to be visible
	testRGDs := []models.CatalogRGD{
		{
			Name:        "fullstack-app-with-resources",
			Namespace:   "default",
			Description: "Full-stack application with conditional ingress and externalRef",
			Tags:        []string{"webapp", "fullstack"},
			Category:    "compute",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "FullStackApp",
			Labels:      map[string]string{},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "FullStackApp",
					"spec": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"configMapName": map[string]interface{}{
								"type": "string",
							},
							"ingress": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"enabled": map[string]interface{}{
										"type": "boolean",
									},
									"host": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
				"resources": []interface{}{
					// ExternalRef to ConfigMap
					map[string]interface{}{
						"id": "configmap",
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "${schema.spec.configMapName}",
								"namespace": "${schema.metadata.namespace}",
							},
						},
					},
					// Deployment that depends on configmap
					map[string]interface{}{
						"id": "deployment",
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"envFrom": []interface{}{
													map[string]interface{}{
														"configMapRef": map[string]interface{}{
															"name": "${configmap.metadata.name}",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					// Service
					map[string]interface{}{
						"id": "service",
						"template": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
						},
					},
					// Conditional Ingress
					map[string]interface{}{
						"id": "ingress",
						"includeWhen": []interface{}{
							"${schema.spec.ingress.enabled == true}",
						},
						"template": map[string]interface{}{
							"apiVersion": "networking.k8s.io/v1",
							"kind":       "Ingress",
						},
					},
				},
			},
		},
		{
			Name:        "postgres-cluster-resources",
			Namespace:   "default",
			Description: "PostgreSQL cluster with conditional replicas",
			Tags:        []string{"database", "production"},
			Category:    "database",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "PostgresCluster",
			Labels:      map[string]string{},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "PostgresCluster",
				},
				"resources": []interface{}{
					// Primary Secret
					map[string]interface{}{
						"id": "credentials",
						"template": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
						},
					},
					// Primary StatefulSet
					map[string]interface{}{
						"id": "primary",
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "StatefulSet",
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"env": []interface{}{
													map[string]interface{}{
														"valueFrom": map[string]interface{}{
															"secretKeyRef": map[string]interface{}{
																"name": "${credentials.metadata.name}",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					// Replica StatefulSet (conditional)
					map[string]interface{}{
						"id": "replica",
						"includeWhen": []interface{}{
							"${schema.spec.replicas > 0}",
						},
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "StatefulSet",
						},
					},
					// Service
					map[string]interface{}{
						"id": "service",
						"template": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
						},
					},
				},
			},
		},
		{
			Name:        "simple-app-no-resources",
			Namespace:   "default",
			Description: "Simple app with no resources defined",
			Tags:        []string{"simple"},
			Category:    "compute",
			APIVersion:  "kro.run/v1alpha1",
			Kind:        "SimpleApp",
			Labels:      map[string]string{},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			RawSpec: map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "SimpleApp",
				},
				// No resources array
			},
		},
	}

	for i := range testRGDs {
		cache.Set(&testRGDs[i])
	}

	w := watcher.NewRGDWatcherWithCache(cache)
	checker := health.NewChecker(nil, nil, w)
	routerResult := api.NewRouterWithConfig(checker, w, nil, nil, api.RouterConfig{
		PolicyEnforcer: &testE2EAdminPolicyEnforcer{},
	})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})

	server := httptest.NewServer(routerResult.Handler)
	return server, cache
}

func TestE2E_ResourceGraph_FullStackApp(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/fullstack-app-with-resources/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var graphResp handlers.ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 4 resources
	if len(graphResp.Resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(graphResp.Resources))
	}

	// Check resource types
	resourceKinds := make(map[string]int)
	for _, res := range graphResp.Resources {
		resourceKinds[res.Kind]++
	}

	if resourceKinds["ConfigMap"] != 1 {
		t.Error("expected 1 ConfigMap resource")
	}
	if resourceKinds["Deployment"] != 1 {
		t.Error("expected 1 Deployment resource")
	}
	if resourceKinds["Service"] != 1 {
		t.Error("expected 1 Service resource")
	}
	if resourceKinds["Ingress"] != 1 {
		t.Error("expected 1 Ingress resource")
	}

	// Check for externalRef
	var configMapRes *handlers.ResourceNodeResponse
	for i := range graphResp.Resources {
		if graphResp.Resources[i].Kind == "ConfigMap" {
			configMapRes = &graphResp.Resources[i]
			break
		}
	}
	if configMapRes == nil {
		t.Fatal("ConfigMap resource not found")
	}
	if configMapRes.IsTemplate {
		t.Error("ConfigMap should not be a template (it's an externalRef)")
	}
	if configMapRes.ExternalRef == nil {
		t.Error("ConfigMap should have externalRef info")
	}
	if configMapRes.ExternalRef != nil && !configMapRes.ExternalRef.UsesSchemaSpec {
		t.Error("ConfigMap externalRef should use schema spec")
	}

	// Check for conditional Ingress
	var ingressRes *handlers.ResourceNodeResponse
	for i := range graphResp.Resources {
		if graphResp.Resources[i].Kind == "Ingress" {
			ingressRes = &graphResp.Resources[i]
			break
		}
	}
	if ingressRes == nil {
		t.Fatal("Ingress resource not found")
	}
	if !ingressRes.IsConditional {
		t.Error("Ingress should be conditional")
	}
	if ingressRes.ConditionExpr == "" {
		t.Error("Ingress should have condition expression")
	}

	// Check for edges (deployment depends on configmap)
	if len(graphResp.Edges) < 1 {
		t.Errorf("expected at least 1 edge, got %d", len(graphResp.Edges))
	}

	foundEdge := false
	for _, edge := range graphResp.Edges {
		// Looking for deployment -> configmap edge
		if edge.Type == "reference" {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		t.Error("expected to find reference edge")
	}
}

func TestE2E_ResourceGraph_PostgresCluster(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/postgres-cluster-resources/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var graphResp handlers.ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 4 resources
	if len(graphResp.Resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(graphResp.Resources))
	}

	// Check for conditional replica StatefulSet
	conditionalCount := 0
	for _, res := range graphResp.Resources {
		if res.IsConditional {
			conditionalCount++
		}
	}
	if conditionalCount != 1 {
		t.Errorf("expected 1 conditional resource, got %d", conditionalCount)
	}

	// All should be templates (no externalRef)
	for _, res := range graphResp.Resources {
		if !res.IsTemplate {
			t.Errorf("resource %s should be a template", res.Kind)
		}
	}
}

func TestE2E_ResourceGraph_NoResources(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/simple-app-no-resources/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var graphResp handlers.ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 0 resources
	if len(graphResp.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(graphResp.Resources))
	}

	// Should have 0 edges
	if len(graphResp.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(graphResp.Edges))
	}

	// RGD name should still be set
	if graphResp.RGDName != "simple-app-no-resources" {
		t.Errorf("expected RGD name 'simple-app-no-resources', got '%s'", graphResp.RGDName)
	}
}

func TestE2E_ResourceGraph_NotFound(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/nonexistent-app/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestE2E_ResourceGraph_ExternalRefDetails(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/fullstack-app-with-resources/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var graphResp handlers.ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Find ConfigMap externalRef and check details
	var configMapRes *handlers.ResourceNodeResponse
	for i := range graphResp.Resources {
		if graphResp.Resources[i].Kind == "ConfigMap" {
			configMapRes = &graphResp.Resources[i]
			break
		}
	}

	if configMapRes == nil {
		t.Fatal("ConfigMap resource not found")
	}

	if configMapRes.ExternalRef == nil {
		t.Fatal("ConfigMap should have externalRef")
	}

	if configMapRes.ExternalRef.APIVersion != "v1" {
		t.Errorf("externalRef APIVersion = %q, want %q", configMapRes.ExternalRef.APIVersion, "v1")
	}

	if configMapRes.ExternalRef.Kind != "ConfigMap" {
		t.Errorf("externalRef Kind = %q, want %q", configMapRes.ExternalRef.Kind, "ConfigMap")
	}

	if configMapRes.ExternalRef.NameExpr != "${schema.spec.configMapName}" {
		t.Errorf("externalRef NameExpr = %q, want %q", configMapRes.ExternalRef.NameExpr, "${schema.spec.configMapName}")
	}

	if !configMapRes.ExternalRef.UsesSchemaSpec {
		t.Error("externalRef should use schema spec")
	}

	if configMapRes.ExternalRef.SchemaField != "spec.configMapName" {
		t.Errorf("externalRef SchemaField = %q, want %q", configMapRes.ExternalRef.SchemaField, "spec.configMapName")
	}
}

func TestE2E_ResourceGraph_ConditionalDetails(t *testing.T) {
	server, _ := setupE2EResourceGraphTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/rgds/fullstack-app-with-resources/resources")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var graphResp handlers.ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Find Ingress and check conditional details
	var ingressRes *handlers.ResourceNodeResponse
	for i := range graphResp.Resources {
		if graphResp.Resources[i].Kind == "Ingress" {
			ingressRes = &graphResp.Resources[i]
			break
		}
	}

	if ingressRes == nil {
		t.Fatal("Ingress resource not found")
	}

	if !ingressRes.IsConditional {
		t.Error("Ingress should be conditional")
	}

	expectedExpr := "${schema.spec.ingress.enabled == true}"
	if ingressRes.ConditionExpr != expectedExpr {
		t.Errorf("ConditionExpr = %q, want %q", ingressRes.ConditionExpr, expectedExpr)
	}

	// Ingress should be a template
	if !ingressRes.IsTemplate {
		t.Error("Ingress should be a template")
	}
}

// setupE2ETestServerWithInstances creates a test server with both RGDs and instances
func setupE2ETestServerWithInstances(t *testing.T) (*httptest.Server, *watcher.RGDCache, *watcher.InstanceCache) {
	rgdCache := watcher.NewRGDCache()
	instanceCache := watcher.NewInstanceCache()

	// Create test RGD
	testRGD := &models.CatalogRGD{
		Name:        "microservices-platform",
		Namespace:   "default",
		Description: "Microservices platform with database",
		Tags:        []string{"platform", "microservices"},
		Category:    "application",
		APIVersion:  "kro.run/v1alpha1",
		Kind:        "MicroservicesPlatform",
		Labels:      map[string]string{"env": "production"},
		Annotations: map[string]string{models.CatalogAnnotation: "true"},
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "kro.run/v1alpha1",
				"kind":       "MicroservicesPlatform",
			},
		},
	}
	rgdCache.Set(testRGD)

	// Create test instances for the RGD
	instance1 := &models.Instance{
		Name:         "platform-dev",
		Namespace:    "dev",
		RGDName:      "microservices-platform",
		RGDNamespace: "default",
		APIVersion:   "kro.run/v1alpha1",
		Kind:         "MicroservicesPlatform",
		Health:       "Healthy",
		Conditions: []models.InstanceCondition{
			{
				Type:   "Ready",
				Status: "True",
				Reason: "AllResourcesReady",
			},
		},
		Spec: map[string]interface{}{
			"replicas": 3,
		},
		Status: map[string]interface{}{
			"phase": "Running",
		},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	}

	instance2 := &models.Instance{
		Name:         "platform-staging",
		Namespace:    "staging",
		RGDName:      "microservices-platform",
		RGDNamespace: "default",
		APIVersion:   "kro.run/v1alpha1",
		Kind:         "MicroservicesPlatform",
		Health:       "Degraded",
		Conditions: []models.InstanceCondition{
			{
				Type:    "Ready",
				Status:  "False",
				Reason:  "DatabaseNotReady",
				Message: "Waiting for database to be ready",
			},
		},
		Spec: map[string]interface{}{
			"replicas": 2,
		},
		Status: map[string]interface{}{
			"phase": "Pending",
		},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-2 * time.Minute),
	}

	instance3 := &models.Instance{
		Name:         "platform-prod",
		Namespace:    "prod",
		RGDName:      "microservices-platform",
		RGDNamespace: "default",
		APIVersion:   "kro.run/v1alpha1",
		Kind:         "MicroservicesPlatform",
		Health:       "Healthy",
		Conditions: []models.InstanceCondition{
			{
				Type:   "Ready",
				Status: "True",
				Reason: "AllResourcesReady",
			},
		},
		Spec: map[string]interface{}{
			"replicas": 5,
		},
		Status: map[string]interface{}{
			"phase": "Running",
		},
		CreatedAt: time.Now().Add(-48 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	}

	instanceCache.Set(instance1)
	instanceCache.Set(instance2)
	instanceCache.Set(instance3)

	// Update RGD instance count - need to fetch, update, and set again
	if rgdFromCache, found := rgdCache.Get("default", "microservices-platform"); found {
		rgdFromCache.InstanceCount = 3
		rgdCache.Set(rgdFromCache)
	}

	// Create watcher and tracker with caches
	w := watcher.NewRGDWatcherWithCache(rgdCache)
	tracker := watcher.NewInstanceTrackerWithCache(instanceCache)

	checker := health.NewChecker(nil, nil, w)

	// Create router with instance tracker
	routerResult := api.NewRouterWithConfig(checker, w, tracker, nil, api.RouterConfig{
		PolicyEnforcer: &testE2EAdminPolicyEnforcer{},
	})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})

	// Wrap with global admin middleware to provide authentication context
	// Instance filtering now requires authentication - unauthenticated
	// requests correctly see no instances (secure default)
	wrappedRouter := testGlobalAdminMiddleware(routerResult.Handler)

	server := httptest.NewServer(wrappedRouter)
	return server, rgdCache, instanceCache
}

// TestE2E_InstancesVisibleInAPI verifies that instances are visible through the API
func TestE2E_InstancesVisibleInAPI(t *testing.T) {
	server, _, instanceCache := setupE2ETestServerWithInstances(t)
	defer server.Close()

	t.Run("list all instances", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.InstanceList
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have 3 instances
		if len(listResp.Items) != 3 {
			t.Errorf("expected 3 instances, got %d", len(listResp.Items))
		}

		if listResp.TotalCount != 3 {
			t.Errorf("expected totalCount 3, got %d", listResp.TotalCount)
		}

		// Verify instance details
		instanceNames := make(map[string]bool)
		for _, inst := range listResp.Items {
			instanceNames[inst.Name] = true

			// All instances should belong to the same RGD
			if inst.RGDName != "microservices-platform" {
				t.Errorf("expected RGDName 'microservices-platform', got %q", inst.RGDName)
			}

			// All instances should have API version and kind
			if inst.APIVersion == "" {
				t.Errorf("instance %s has empty APIVersion", inst.Name)
			}
			if inst.Kind == "" {
				t.Errorf("instance %s has empty Kind", inst.Name)
			}
		}

		// Verify all expected instances are present
		expectedInstances := []string{"platform-dev", "platform-staging", "platform-prod"}
		for _, name := range expectedInstances {
			if !instanceNames[name] {
				t.Errorf("expected instance %q not found in response", name)
			}
		}
	})

	t.Run("filter instances by namespace", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances?namespace=dev")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.InstanceList
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have 1 instance in dev namespace
		if len(listResp.Items) != 1 {
			t.Errorf("expected 1 instance in dev namespace, got %d", len(listResp.Items))
		}

		if len(listResp.Items) > 0 && listResp.Items[0].Name != "platform-dev" {
			t.Errorf("expected instance 'platform-dev', got %q", listResp.Items[0].Name)
		}
	})

	t.Run("filter instances by RGD", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances?rgdName=microservices-platform")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.InstanceList
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have all 3 instances
		if len(listResp.Items) != 3 {
			t.Errorf("expected 3 instances for microservices-platform, got %d", len(listResp.Items))
		}
	})

	t.Run("filter instances by health status", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances?health=Healthy")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.InstanceList
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have 2 healthy instances (platform-dev and platform-prod)
		if len(listResp.Items) != 2 {
			t.Errorf("expected 2 healthy instances, got %d", len(listResp.Items))
		}

		for _, inst := range listResp.Items {
			if inst.Health != "Healthy" {
				t.Errorf("expected health 'Healthy', got %q for instance %s", inst.Health, inst.Name)
			}
		}
	})

	t.Run("get specific instance", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/namespaces/dev/instances/MicroservicesPlatform/platform-dev")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var inst models.Instance
		if err := json.NewDecoder(resp.Body).Decode(&inst); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify instance details
		if inst.Name != "platform-dev" {
			t.Errorf("expected name 'platform-dev', got %q", inst.Name)
		}
		if inst.Namespace != "dev" {
			t.Errorf("expected namespace 'dev', got %q", inst.Namespace)
		}
		if inst.RGDName != "microservices-platform" {
			t.Errorf("expected RGDName 'microservices-platform', got %q", inst.RGDName)
		}
		if inst.Health != "Healthy" {
			t.Errorf("expected health 'Healthy', got %q", inst.Health)
		}
		if inst.APIVersion != "kro.run/v1alpha1" {
			t.Errorf("expected APIVersion 'kro.run/v1alpha1', got %q", inst.APIVersion)
		}
		if inst.Kind != "MicroservicesPlatform" {
			t.Errorf("expected Kind 'MicroservicesPlatform', got %q", inst.Kind)
		}

		// Verify conditions
		if len(inst.Conditions) != 1 {
			t.Errorf("expected 1 condition, got %d", len(inst.Conditions))
		} else {
			if inst.Conditions[0].Type != "Ready" {
				t.Errorf("expected condition type 'Ready', got %q", inst.Conditions[0].Type)
			}
			if inst.Conditions[0].Status != "True" {
				t.Errorf("expected condition status 'True', got %q", inst.Conditions[0].Status)
			}
		}

		// Verify spec and status are present
		if inst.Spec == nil {
			t.Error("expected spec to be present")
		}
		if inst.Status == nil {
			t.Error("expected status to be present")
		}
	})

	t.Run("RGD shows correct instance count", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/rgds/microservices-platform?namespace=default")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var rgdResp services.RGDResponse
		if err := json.NewDecoder(resp.Body).Decode(&rgdResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// RGD should show 3 instances
		if rgdResp.Instances != 3 {
			t.Errorf("expected instance count 3, got %d", rgdResp.Instances)
		}
	})

	t.Run("verify cache consistency", func(t *testing.T) {
		// Verify cache has the expected instances
		cacheCount := instanceCache.Count()
		if cacheCount != 3 {
			t.Errorf("expected cache to have 3 instances, got %d", cacheCount)
		}

		// Verify we can retrieve instances from cache
		devInst, found := instanceCache.Get("dev", "MicroservicesPlatform", "platform-dev")
		if !found {
			t.Error("expected to find platform-dev in cache")
		}
		if devInst != nil && devInst.Health != "Healthy" {
			t.Errorf("expected cached instance to have health 'Healthy', got %q", devInst.Health)
		}
	})
}

// TestE2E_OldInstanceRoutes_Return404 verifies that the pre-STORY-327 route patterns
// (/{namespace}/{kind}/{name} and /-/{kind}/{name}) no longer match instance endpoints.
func TestE2E_OldInstanceRoutes_Return404(t *testing.T) {
	cache := watcher.NewRGDCache()
	w := watcher.NewRGDWatcherWithCache(cache)
	checker := health.NewChecker(nil, nil, w)

	routerResult := api.NewRouterWithConfig(checker, w, nil, nil, api.RouterConfig{
		PolicyEnforcer: &testE2EAdminPolicyEnforcer{},
	})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})
	server := httptest.NewServer(routerResult.Handler)
	defer server.Close()

	t.Run("old namespaced get returns 404", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances/default/webapp/my-app")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("old cluster-scoped sentinel get returns 404", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/instances/-/webapp/my-app")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("old POST /api/v1/instances returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/instances", nil)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 404 or 405, got %d", resp.StatusCode)
		}
	})

	t.Run("new K8s-aligned cluster-scoped get routes to handler", func(t *testing.T) {
		// Without auth middleware this returns 401/503, but NOT 404 — proving the route is registered
		resp, err := http.Get(server.URL + "/api/v1/instances/webapp/my-app")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Error("cluster-scoped instance route should be registered (got 404)")
		}
	})

	t.Run("cluster-scoped history routes not registered without HistoryService", func(t *testing.T) {
		// History routes are conditionally registered only when HistoryService != nil.
		// Without HistoryService, these should return 404 (not registered).
		paths := []string{
			"/api/v1/instances/webapp/my-app/history",
			"/api/v1/instances/webapp/my-app/history/export",
			"/api/v1/instances/webapp/my-app/timeline",
		}
		for _, path := range paths {
			resp, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatalf("request to %s failed: %v", path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("cluster-scoped history route %s should NOT be registered without HistoryService (got %d)", path, resp.StatusCode)
			}
		}
	})
}

// TestE2E_InstanceCreation_FailsClosed_NilPolicyEnforcer tests that POST instance create
// returns 503 when PolicyEnforcer is nil (AC-1: fail-closed on missing auth service)
func TestE2E_InstanceCreation_FailsClosed_NilPolicyEnforcer(t *testing.T) {
	// Create minimal test setup without PolicyEnforcer or ProjectService
	cache := watcher.NewRGDCache()
	w := watcher.NewRGDWatcherWithCache(cache)
	checker := health.NewChecker(nil, nil, w)

	// Router with no PolicyEnforcer — should register fail-closed handler
	routerResult := api.NewRouterWithConfig(checker, w, nil, nil, api.RouterConfig{
		PolicyEnforcer: &testE2EAdminPolicyEnforcer{},
	})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})
	server := httptest.NewServer(routerResult.Handler)
	defer server.Close()

	// POST to create instance (K8s-aligned route) — should get 503
	resp, err := http.Post(server.URL+"/api/v1/instances/webapp", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if errResp.Message != "instance creation temporarily unavailable" {
		t.Errorf("unexpected message: %q", errResp.Message)
	}
}
