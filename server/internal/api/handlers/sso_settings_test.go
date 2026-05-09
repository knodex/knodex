// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/sso"
)

const ssoTestNamespace = "test-ns"

// mockSSOAccessChecker implements ssoAccessChecker for testing.
type mockSSOAccessChecker struct {
	allowed    bool
	err        error
	lastObject string
	lastAction string
}

func (m *mockSSOAccessChecker) CanAccessWithGroups(_ context.Context, _ string, _ []string, object, action string) (bool, error) {
	m.lastObject = object
	m.lastAction = action
	return m.allowed, m.err
}

func newSSOTestHandler() (*SSOSettingsHandler, *sso.ProviderStore) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	checker := &mockSSOAccessChecker{allowed: true}
	handler := NewSSOSettingsHandler(store, nil, checker)
	return handler, store
}

func newSSOTestHandlerWithAuthz(allowed bool) (*SSOSettingsHandler, *sso.ProviderStore, *mockSSOAccessChecker, *mockAuditRecorder) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	checker := &mockSSOAccessChecker{allowed: allowed}
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, checker)
	return handler, store, checker, recorder
}

func ssoRequestWithRole(method, path string, body interface{}, roles []string) *http.Request {
	var req *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	userCtx := &middleware.UserContext{
		UserID:      "test-user",
		Email:       "test@example.com",
		DisplayName: "Test User",
		CasbinRoles: roles,
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	req.Header.Set("X-Request-ID", "test-req-id")
	return req
}

func ssoRequest(method, path string, body interface{}) *http.Request {
	var req *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	// Add user context
	userCtx := &middleware.UserContext{
		UserID:      "admin-123",
		Email:       "admin@example.com",
		DisplayName: "Admin User",
		CasbinRoles: []string{"role:serveradmin"},
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	req.Header.Set("X-Request-ID", "test-req-id")

	return req
}

func TestSSOSettingsHandler_ListProviders_Empty(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	// Create the ConfigMap so List works (empty list)
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "temp", IssuerURL: "https://example.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "temp"); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest("GET", "/api/v1/settings/sso/providers", nil)
	w := httptest.NewRecorder()

	handler.ListProviders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 providers, got %d", len(result))
	}
}

func TestSSOSettingsHandler_CreateAndList(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "google-client-id",
		ClientSecret: "google-client-secret",
		RedirectURL:  "https://app.example.com/api/v1/auth/oidc/callback",
		Scopes:       []string{"openid", "profile", "email"},
	}

	// Create
	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if created.Name != "google" {
		t.Errorf("expected name 'google', got %q", created.Name)
	}

	// List
	req = ssoRequest("GET", "/api/v1/settings/sso/providers", nil)
	w = httptest.NewRecorder()
	handler.ListProviders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var list []SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(list))
	}
	if list[0].Name != "google" {
		t.Errorf("expected name 'google', got %q", list[0].Name)
	}
}

func TestSSOSettingsHandler_GetProvider(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest("GET", "/api/v1/settings/sso/providers/google", nil)
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.GetProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Name != "google" {
		t.Errorf("expected name 'google', got %q", got.Name)
	}
}

func TestSSOSettingsHandler_GetProvider_NotFound(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	// Create and delete to ensure ConfigMap exists
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "temp", IssuerURL: "https://example.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "temp"); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest("GET", "/api/v1/settings/sso/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetProvider(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_UpdateProvider(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "old-id", ClientSecret: "old-secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	updateReq := SSOProviderRequest{
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "new-id",
		ClientSecret: "new-secret",
		RedirectURL:  "https://new.example.com/cb",
		Scopes:       []string{"openid", "profile"},
	}

	req := ssoRequest("PUT", "/api/v1/settings/sso/providers/google", updateReq)
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.UpdateProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if updated.RedirectURL != "https://new.example.com/cb" {
		t.Errorf("expected redirectURL 'https://new.example.com/cb', got %q", updated.RedirectURL)
	}
}

func TestSSOSettingsHandler_DeleteProvider(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest("DELETE", "/api/v1/settings/sso/providers/google", nil)
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.DeleteProvider(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_DeleteProvider_NotFound(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	// Create and delete to ensure ConfigMap exists
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "temp", IssuerURL: "https://example.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "temp"); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest("DELETE", "/api/v1/settings/sso/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.DeleteProvider(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_CreateProvider_DuplicateReturns409(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_Validation_InvalidName(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	tests := []struct {
		name    string
		reqName string
	}{
		{"empty name", ""},
		{"uppercase", "GOOGLE"},
		{"spaces", "my provider"},
		{"dots", "a.b"},
		{"starts with hyphen", "-google"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			createReq := SSOProviderRequest{
				Name:         tt.reqName,
				IssuerURL:    "https://accounts.google.com",
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURL:  "https://app.example.com/cb",
				Scopes:       []string{"openid"},
			}

			req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
			w := httptest.NewRecorder()

			handler.CreateProvider(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for name %q, got %d: %s", tt.reqName, w.Code, w.Body.String())
			}
		})
	}
}

func TestSSOSettingsHandler_Validation_IssuerURL(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	tests := []struct {
		name      string
		issuerURL string
	}{
		{"empty", ""},
		{"http not https", "http://accounts.google.com"},
		{"invalid url", "not-a-url"},
		{"no scheme", "accounts.google.com"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			createReq := SSOProviderRequest{
				Name:         "google",
				IssuerURL:    tt.issuerURL,
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURL:  "https://app.example.com/cb",
				Scopes:       []string{"openid"},
			}

			req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
			w := httptest.NewRecorder()

			handler.CreateProvider(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for issuerURL %q, got %d: %s", tt.issuerURL, w.Code, w.Body.String())
			}
		})
	}
}

// TestSSOSettingsHandler_MissingSecretInfersPublicClient verifies the inference
// rule: when both tokenEndpointAuthMethod and clientSecret are omitted, the
// provider is created as a public (PKCE) client rather than rejected.
// Operators that explicitly want a confidential client must set the method
// (covered by TestCreateProvider_Confidential_RequiresSecret).
func TestSSOSettingsHandler_MissingSecretInfersPublicClient(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:        "google",
		IssuerURL:   "https://accounts.google.com",
		ClientID:    "id",
		RedirectURL: "https://app.example.com/cb",
		Scopes:      []string{"openid"},
		// ClientSecret + tokenEndpointAuthMethod intentionally empty.
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (inferred public client), got %d: %s", w.Code, w.Body.String())
	}
	var resp SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TokenEndpointAuthMethod != "none" {
		t.Errorf("expected inferred method=none, got %q", resp.TokenEndpointAuthMethod)
	}
}

func TestSSOSettingsHandler_Validation_MissingClientID(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
		// ClientID intentionally empty
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing clientID on create, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_Validation_RedirectURL(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	tests := []struct {
		name        string
		redirectURL string
	}{
		{"empty", ""},
		{"invalid", "not-a-url"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			createReq := SSOProviderRequest{
				Name:         "google",
				IssuerURL:    "https://accounts.google.com",
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURL:  tt.redirectURL,
				Scopes:       []string{"openid"},
			}

			req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
			w := httptest.NewRecorder()

			handler.CreateProvider(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for redirectURL %q, got %d: %s", tt.redirectURL, w.Code, w.Body.String())
			}
		})
	}
}

func TestSSOSettingsHandler_NoUserContext(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	handler := NewSSOSettingsHandler(store, nil, nil)

	// Request without user context
	req := httptest.NewRequest("GET", "/api/v1/settings/sso/providers", nil)
	w := httptest.NewRecorder()

	handler.ListProviders(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without user context, got %d", w.Code)
	}
}

func TestSSOSettingsHandler_ResponseExcludesClientSecret(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret-value",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	// Create
	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Check that response body does not contain the client secret
	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("super-secret-value")) {
		t.Error("response body should not contain the client secret")
	}

	// Verify the response struct has no clientSecret field
	var response SSOProviderResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// SSOProviderResponse struct has no ClientSecret field — verified by type system
}

// TestSSOSettingsHandler_Validation_AcceptsPrivateIPs locks in the ArgoCD-aligned
// posture: private/in-cluster URLs are now accepted by both the issuerURL path
// and the explicit-endpoint trio (AuthorizationURL/TokenURL/JWKSURL). Replaces
// the deleted negative test that asserted these were rejected.
//
// IPv4-mapped IPv6 (::ffff:169.254.169.254) is included because it was caught
// by the previous netutil.IsPrivateHost check. After this change it passes
// validation alongside other private addresses; the admin trust boundary
// covers it.
func TestSSOSettingsHandler_Validation_AcceptsPrivateIPs(t *testing.T) {
	t.Parallel()

	urls := []struct {
		name string
		url  string
	}{
		{"loopback IPv4", "https://127.0.0.1/"},
		{"loopback IPv6", "https://[::1]/"},
		{"private 10.x", "https://10.0.0.1/"},
		{"private 172.16.x", "https://172.16.0.1/"},
		{"private 192.168.x", "https://192.168.1.1/"},
		{"link-local", "https://169.254.169.254/"},
		{"CGNAT 100.64.x", "https://100.64.0.1/"},
		{"benchmark 198.18.x", "https://198.18.0.1/"},
		{"IPv4-mapped IPv6", "https://[::ffff:169.254.169.254]/"},
	}

	t.Run("issuerURL", func(t *testing.T) {
		t.Parallel()
		// Each subtest gets its own handler so parallel inner subtests cannot
		// collide on provider names in the shared fake k8s store.
		for i, tt := range urls {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				handler, _ := newSSOTestHandler()
				name := fmt.Sprintf("test-issuer-%d", i)
				createReq := SSOProviderRequest{
					Name:         name,
					IssuerURL:    tt.url,
					ClientID:     "id",
					ClientSecret: "secret",
					RedirectURL:  "https://app.example.com/cb",
					Scopes:       []string{"openid"},
				}
				req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
				w := httptest.NewRecorder()
				handler.CreateProvider(w, req)
				if w.Code != http.StatusCreated {
					t.Fatalf("expected 201 for issuerURL %q, got %d: %s", tt.url, w.Code, w.Body.String())
				}
				var resp SSOProviderResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.IssuerURL != tt.url {
					t.Errorf("response IssuerURL = %q, want %q (URL must round-trip unchanged)", resp.IssuerURL, tt.url)
				}
			})
		}
	})

	t.Run("explicitEndpoints", func(t *testing.T) {
		t.Parallel()
		for i, tt := range urls {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				handler, _ := newSSOTestHandler()
				name := fmt.Sprintf("test-explicit-%d", i)
				createReq := SSOProviderRequest{
					Name:             name,
					IssuerURL:        "https://accounts.google.com",
					ClientID:         "id",
					ClientSecret:     "secret",
					RedirectURL:      "https://app.example.com/cb",
					Scopes:           []string{"openid"},
					AuthorizationURL: tt.url,
					TokenURL:         tt.url,
					JWKSURL:          tt.url,
				}
				req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
				w := httptest.NewRecorder()
				handler.CreateProvider(w, req)
				if w.Code != http.StatusCreated {
					t.Fatalf("expected 201 for explicit endpoint %q, got %d: %s", tt.url, w.Code, w.Body.String())
				}
				var resp SSOProviderResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.AuthorizationURL != tt.url || resp.TokenURL != tt.url || resp.JWKSURL != tt.url {
					t.Errorf("response endpoints did not round-trip: got auth=%q token=%q jwks=%q want all %q",
						resp.AuthorizationURL, resp.TokenURL, resp.JWKSURL, tt.url)
				}
			})
		}
	})
}

func TestSSOSettingsHandler_NotFoundError_TypeChecked(t *testing.T) {
	t.Parallel()
	handler, store := newSSOTestHandler()
	ctx := context.Background()

	// Create a provider first so the ConfigMap exists
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "existing", IssuerURL: "https://example.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	// Get non-existent provider should return 404 (via typed NotFoundError)
	req := ssoRequest("GET", "/api/v1/settings/sso/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetProvider(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Authorization Tests (AC-1 through AC-5) ===

func TestSSOSettingsHandler_CreateProvider_DeniedWithoutPermission(t *testing.T) {
	t.Parallel()
	handler, _, checker, recorder := newSSOTestHandlerWithAuthz(false)

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	req := ssoRequestWithRole("POST", "/api/v1/settings/sso/providers", createReq, []string{"proj:test:viewer"})
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("AC-1: expected 403 for readonly user on CreateProvider, got %d: %s", w.Code, w.Body.String())
	}
	if checker.lastObject != "settings/*" {
		t.Errorf("expected object 'settings/*', got %q", checker.lastObject)
	}
	if checker.lastAction != "update" {
		t.Errorf("expected action 'update', got %q", checker.lastAction)
	}
	// Verify error response body matches project contract
	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %v", errResp["code"])
	}
	// Verify denied access is audit-logged with correct operation context
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for denied access, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Action != "create" {
		t.Errorf("expected audit action 'create', got %q", e.Action)
	}
	if e.Result != "denied" {
		t.Errorf("expected audit result 'denied', got %q", e.Result)
	}
}

func TestSSOSettingsHandler_UpdateProvider_DeniedWithoutPermission(t *testing.T) {
	t.Parallel()
	handler, store, checker, recorder := newSSOTestHandlerWithAuthz(false)
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	updateReq := SSOProviderRequest{
		IssuerURL:   "https://accounts.google.com",
		ClientID:    "new-id",
		RedirectURL: "https://new.example.com/cb",
	}

	req := ssoRequestWithRole("PUT", "/api/v1/settings/sso/providers/google", updateReq, []string{"proj:test:viewer"})
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.UpdateProvider(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("AC-2: expected 403 for readonly user on UpdateProvider, got %d: %s", w.Code, w.Body.String())
	}
	if checker.lastObject != "settings/*" {
		t.Errorf("expected object 'settings/*', got %q", checker.lastObject)
	}
	if checker.lastAction != "update" {
		t.Errorf("expected action 'update', got %q", checker.lastAction)
	}
	// Verify denied access is audit-logged with provider-specific context
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for denied access, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected audit action 'update', got %q", e.Action)
	}
	if e.Name != "google" {
		t.Errorf("expected audit name 'google', got %q", e.Name)
	}
	if e.Result != "denied" {
		t.Errorf("expected audit result 'denied', got %q", e.Result)
	}
}

func TestSSOSettingsHandler_DeleteProvider_DeniedWithoutPermission(t *testing.T) {
	t.Parallel()
	handler, store, checker, recorder := newSSOTestHandlerWithAuthz(false)
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequestWithRole("DELETE", "/api/v1/settings/sso/providers/google", nil, []string{"proj:test:viewer"})
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.DeleteProvider(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("AC-3: expected 403 for readonly user on DeleteProvider, got %d: %s", w.Code, w.Body.String())
	}
	if checker.lastObject != "settings/*" {
		t.Errorf("expected object 'settings/*', got %q", checker.lastObject)
	}
	if checker.lastAction != "update" {
		t.Errorf("expected action 'update', got %q", checker.lastAction)
	}
	// Verify denied access is audit-logged with provider-specific context
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for denied access, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Action != "delete" {
		t.Errorf("expected audit action 'delete', got %q", e.Action)
	}
	if e.Name != "google" {
		t.Errorf("expected audit name 'google', got %q", e.Name)
	}
	if e.Result != "denied" {
		t.Errorf("expected audit result 'denied', got %q", e.Result)
	}
}

func TestSSOSettingsHandler_AdminCanManageProviders(t *testing.T) {
	t.Parallel()
	handler, _, checker, _ := newSSOTestHandlerWithAuthz(true)

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	// Create should succeed
	req := ssoRequestWithRole("POST", "/api/v1/settings/sso/providers", createReq, []string{"role:serveradmin"})
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("AC-4: expected 201 for admin on CreateProvider, got %d: %s", w.Code, w.Body.String())
	}

	// Update should succeed
	updateReq := SSOProviderRequest{
		IssuerURL:   "https://accounts.google.com",
		ClientID:    "new-id",
		RedirectURL: "https://new.example.com/cb",
	}
	req = ssoRequestWithRole("PUT", "/api/v1/settings/sso/providers/google", updateReq, []string{"role:serveradmin"})
	req.SetPathValue("name", "google")
	w = httptest.NewRecorder()
	handler.UpdateProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("AC-4: expected 200 for admin on UpdateProvider, got %d: %s", w.Code, w.Body.String())
	}

	// Delete should succeed
	req = ssoRequestWithRole("DELETE", "/api/v1/settings/sso/providers/google", nil, []string{"role:serveradmin"})
	req.SetPathValue("name", "google")
	w = httptest.NewRecorder()
	handler.DeleteProvider(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("AC-4: expected 204 for admin on DeleteProvider, got %d: %s", w.Code, w.Body.String())
	}
	// Verify all operations checked the correct resource/action
	if checker.lastObject != "settings/*" {
		t.Errorf("expected object 'settings/*', got %q", checker.lastObject)
	}
	if checker.lastAction != "update" {
		t.Errorf("expected action 'update', got %q", checker.lastAction)
	}
}

func TestSSOSettingsHandler_ListProviders_AllowedWithoutSettingsUpdate(t *testing.T) {
	t.Parallel()
	// ListProviders requires settings:get but NOT settings:update
	handler, store, checker, _ := newSSOTestHandlerWithAuthz(true)
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequestWithRole("GET", "/api/v1/settings/sso/providers", nil, []string{"role:serveradmin"})
	w := httptest.NewRecorder()

	handler.ListProviders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("AC-5: expected 200 for admin user on ListProviders, got %d: %s", w.Code, w.Body.String())
	}
	// Verify access checker was called with get (not update) action
	if checker.lastAction != "get" {
		t.Errorf("expected action 'get', got %q", checker.lastAction)
	}
}

func TestSSOSettingsHandler_GetProvider_AllowedWithoutSettingsUpdate(t *testing.T) {
	t.Parallel()
	// GetProvider requires settings:get but NOT settings:update
	handler, store, checker, _ := newSSOTestHandlerWithAuthz(true)
	ctx := context.Background()

	if err := store.Create(ctx, sso.SSOProvider{
		Name: "google", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequestWithRole("GET", "/api/v1/settings/sso/providers/google", nil, []string{"role:serveradmin"})
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()

	handler.GetProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin user on GetProvider, got %d: %s", w.Code, w.Body.String())
	}
	// Verify access checker was called with get (not update) action
	if checker.lastAction != "get" {
		t.Errorf("expected action 'get', got %q", checker.lastAction)
	}
}

func TestSSOSettingsHandler_CreateProvider_InternalErrorOnPolicyCheck(t *testing.T) {
	t.Parallel()
	// When CanAccessWithGroups returns an error, the handler should return 500
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	checker := &mockSSOAccessChecker{allowed: false, err: errors.New("policy engine failure")}
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, checker)

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	req := ssoRequestWithRole("POST", "/api/v1/settings/sso/providers", createReq, []string{"role:serveradmin"})
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when policy check errors, got %d: %s", w.Code, w.Body.String())
	}
	// Verify error is audit-logged
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for policy check error, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Result != "error" {
		t.Errorf("expected audit result 'error', got %q", e.Result)
	}
}

func TestSSOSettingsHandler_CreateProvider_DeniedBeforeBodyParsing(t *testing.T) {
	t.Parallel()
	// Auth check should reject before body parsing — even with invalid body, should get 403 not 400
	handler, _, _, _ := newSSOTestHandlerWithAuthz(false)

	req := httptest.NewRequest("POST", "/api/v1/settings/sso/providers", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	userCtx := &middleware.UserContext{
		UserID:      "test-user",
		Email:       "test@example.com",
		DisplayName: "Test User",
		CasbinRoles: []string{"proj:test:viewer"},
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	req.Header.Set("X-Request-ID", "test-req-id")
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 before body parsing, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOSettingsHandler_WriteEndpoint_DeniedWhenAccessCheckerNil(t *testing.T) {
	t.Parallel()
	// When accessChecker is nil (OSS fallback), write endpoints should be denied
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	handler := NewSSOSettingsHandler(store, nil, nil)

	createReq := SSOProviderRequest{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()

	handler.CreateProvider(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when accessChecker is nil, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateProvider_PublicClient_NoSecret verifies AC10: a public-client SSO
// provider can be created with no client secret, and the resulting K8s Secret
// contains only the client-id key (no client-secret).
func TestCreateProvider_PublicClient_NoSecret(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	checker := &mockSSOAccessChecker{allowed: true}
	handler := NewSSOSettingsHandler(store, nil, checker)

	createReq := SSOProviderRequest{
		Name:                    "supabase",
		IssuerURL:               "https://accounts.google.com",
		ClientID:                "supabase-public-id",
		RedirectURL:             "https://app.example.com/api/v1/auth/oidc/callback",
		Scopes:                  []string{"openid", "email", "profile"},
		TokenEndpointAuthMethod: "none",
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created SSOProviderResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.TokenEndpointAuthMethod != "none" {
		t.Errorf("response method = %q, want %q", created.TokenEndpointAuthMethod, "none")
	}

	// Verify the K8s Secret contains client-id but NOT client-secret.
	secret, err := cs.CoreV1().Secrets(ssoTestNamespace).Get(context.Background(), sso.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get sso secret: %v", err)
	}
	if _, ok := secret.Data["supabase.client-id"]; !ok {
		t.Errorf("expected supabase.client-id key in secret, got keys: %v", keysOf(secret))
	}
	if _, ok := secret.Data["supabase.client-secret"]; ok {
		t.Errorf("expected NO supabase.client-secret key for public client, got keys: %v", keysOf(secret))
	}
}

// TestCreateProvider_Confidential_RequiresSecret verifies AC11: an explicit
// client_secret_basic request without a secret is rejected with a validation error.
func TestCreateProvider_Confidential_RequiresSecret(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:                    "googletest",
		IssuerURL:               "https://accounts.google.com",
		ClientID:                "google-id",
		RedirectURL:             "https://app.example.com/cb",
		Scopes:                  []string{"openid"},
		TokenEndpointAuthMethod: "client_secret_basic",
		// ClientSecret intentionally empty
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "clientSecret") {
		t.Errorf("expected validation error to mention clientSecret, got: %s", w.Body.String())
	}
}

// TestCreateProvider_InvalidAuthMethod verifies AC12: an unsupported
// tokenEndpointAuthMethod value is rejected with a validation error.
func TestCreateProvider_InvalidAuthMethod(t *testing.T) {
	t.Parallel()
	handler, _ := newSSOTestHandler()

	createReq := SSOProviderRequest{
		Name:                    "exotic",
		IssuerURL:               "https://accounts.google.com",
		ClientID:                "id",
		ClientSecret:            "secret",
		RedirectURL:             "https://app.example.com/cb",
		Scopes:                  []string{"openid"},
		TokenEndpointAuthMethod: "client_secret_jwt",
	}

	req := ssoRequest("POST", "/api/v1/settings/sso/providers", createReq)
	w := httptest.NewRecorder()
	handler.CreateProvider(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "tokenEndpointAuthMethod") {
		t.Errorf("expected validation error to mention tokenEndpointAuthMethod, got: %s", w.Body.String())
	}
}

// TestUpdateProvider_FlipToPublic verifies AC13: flipping a confidential
// provider to public removes the stored client-secret from the K8s Secret.
func TestUpdateProvider_FlipToPublic(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	checker := &mockSSOAccessChecker{allowed: true}
	handler := NewSSOSettingsHandler(store, nil, checker)

	// Seed a confidential provider directly via the store.
	if err := store.Create(context.Background(), sso.SSOProvider{
		Name:                    "google",
		IssuerURL:               "https://accounts.google.com",
		ClientID:                "google-id",
		ClientSecret:            "google-secret",
		RedirectURL:             "https://app.example.com/cb",
		Scopes:                  []string{"openid"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	// Sanity: the secret key is present.
	secret, err := cs.CoreV1().Secrets(ssoTestNamespace).Get(context.Background(), sso.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get sso secret pre-update: %v", err)
	}
	if _, ok := secret.Data["google.client-secret"]; !ok {
		t.Fatalf("seed: expected google.client-secret key, got: %v", keysOf(secret))
	}

	updateReq := SSOProviderRequest{
		IssuerURL:               "https://accounts.google.com",
		ClientID:                "google-id",
		RedirectURL:             "https://app.example.com/cb",
		Scopes:                  []string{"openid"},
		TokenEndpointAuthMethod: "none",
		// no clientSecret — flipping to public should clear it
	}
	req := ssoRequest("PUT", "/api/v1/settings/sso/providers/google", updateReq)
	req.SetPathValue("name", "google")
	w := httptest.NewRecorder()
	handler.UpdateProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// The secret should no longer contain the client-secret key.
	secret, err = cs.CoreV1().Secrets(ssoTestNamespace).Get(context.Background(), sso.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get sso secret post-update: %v", err)
	}
	if _, ok := secret.Data["google.client-secret"]; ok {
		t.Errorf("expected client-secret to be removed after flip to public, got keys: %v", keysOf(secret))
	}
	if _, ok := secret.Data["google.client-id"]; !ok {
		t.Errorf("expected client-id to remain after flip, got keys: %v", keysOf(secret))
	}
}

// keysOf returns the sorted keys of a Secret's Data map for diagnostic logging.
func keysOf(s *corev1.Secret) []string {
	out := make([]string, 0, len(s.Data))
	for k := range s.Data {
		out = append(out, k)
	}
	return out
}
