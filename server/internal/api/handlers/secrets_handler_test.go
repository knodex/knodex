// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// mockSecretsEnforcer implements SecretsHandlerEnforcer for testing
type mockSecretsEnforcer struct {
	canAccess    bool
	canAccessErr error
	canAccessMap map[string]bool // "object:action" -> result
}

func (m *mockSecretsEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessErr != nil {
		return false, m.canAccessErr
	}
	if m.canAccessMap != nil {
		if result, ok := m.canAccessMap[object+":"+action]; ok {
			return result, nil
		}
	}
	return m.canAccess, nil
}

func (m *mockSecretsEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

func (m *mockSecretsEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	allowed, err := m.CanAccess(ctx, user, "projects/"+projectName, action)
	if err != nil {
		return err
	}
	if !allowed {
		return rbac.ErrAccessDenied
	}
	return nil
}

func (m *mockSecretsEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	return nil, nil
}

func (m *mockSecretsEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	return false, nil
}

func newSecretsRequest(method, url string, body interface{}, userCtx *middleware.UserContext) *http.Request {
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, url, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	req.Header.Set("X-Request-ID", "test-request-id")
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

func defaultUserCtx() *middleware.UserContext {
	return &middleware.UserContext{
		UserID: "user@test.local",
		Email:  "user@test.local",
		Groups: []string{"developers"},
	}
}

func TestSecretsHandler_CreateSecret_Success(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	enforcer := &mockSecretsEnforcer{canAccess: true}
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  enforcer,
	})

	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"password": "s3cret", "username": "admin"},
	}

	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp SecretResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "my-secret", resp.Name)
	assert.Equal(t, "default", resp.Namespace)
	assert.ElementsMatch(t, []string{"password", "username"}, resp.Keys)
	// Note: fake K8s client doesn't populate CreationTimestamp, so we skip that check
	assert.Equal(t, "demo", resp.Labels["knodex.io/project"])
	assert.Equal(t, "knodex", resp.Labels["knodex.io/managed-by"])

	// Verify secret values are NOT in the response
	responseBody := rr.Body.String()
	assert.NotContains(t, responseBody, "s3cret")
	assert.NotContains(t, responseBody, `"data"`)

	// Verify K8s secret was actually created
	secret, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "demo", secret.Labels["knodex.io/project"])
	assert.Equal(t, "knodex", secret.Labels["knodex.io/managed-by"])
}

func TestSecretsHandler_CreateSecret_MissingProject(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{Name: "test", Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "project query parameter is required")
}

func TestSecretsHandler_CreateSecret_Duplicate(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-secret",
			Namespace: "default",
		},
	})
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{
		Name:      "existing-secret",
		Namespace: "default",
		Data:      map[string]string{"k": "v"},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "already exists")
}

func TestSecretsHandler_CreateSecret_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: false},
	})

	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"k": "v"},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_CreateSecret_ValidationErrors(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name   string
		body   CreateSecretRequest
		errKey string
	}{
		{
			name:   "empty name",
			body:   CreateSecretRequest{Name: "", Namespace: "default", Data: map[string]string{"k": "v"}},
			errKey: "name",
		},
		{
			name:   "invalid name",
			body:   CreateSecretRequest{Name: "INVALID_NAME", Namespace: "default", Data: map[string]string{"k": "v"}},
			errKey: "name",
		},
		{
			name:   "empty namespace",
			body:   CreateSecretRequest{Name: "my-secret", Namespace: "", Data: map[string]string{"k": "v"}},
			errKey: "namespace",
		},
		{
			name:   "empty data",
			body:   CreateSecretRequest{Name: "my-secret", Namespace: "default", Data: map[string]string{}},
			errKey: "data",
		},
		{
			name:   "nil data",
			body:   CreateSecretRequest{Name: "my-secret", Namespace: "default", Data: nil},
			errKey: "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", tt.body, defaultUserCtx())
			rr := httptest.NewRecorder()
			handler.CreateSecret(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.errKey)
		})
	}
}

func TestSecretsHandler_CreateSecret_NoUserContext(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{Name: "test", Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, nil)
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSecretsHandler_ListSecrets_Success(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-1",
				Namespace: "ns-1",
				Labels: map[string]string{
					"knodex.io/project":    "demo",
					"knodex.io/managed-by": "knodex",
				},
			},
			Data: map[string][]byte{
				"password": []byte("s3cret"),
				"username": []byte("admin"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-2",
				Namespace: "ns-2",
				Labels: map[string]string{
					"knodex.io/project":    "demo",
					"knodex.io/managed-by": "knodex",
				},
			},
			Data: map[string][]byte{
				"api-key": []byte("abc123"),
			},
		},
	)

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SecretListResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 2, resp.TotalCount)
	assert.Len(t, resp.Items, 2)

	// Verify no secret values in response
	responseBody := rr.Body.String()
	assert.NotContains(t, responseBody, "s3cret")
	assert.NotContains(t, responseBody, "admin")
	assert.NotContains(t, responseBody, "abc123")
}

func TestSecretsHandler_ListSecrets_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: false},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_ListSecrets_MissingProject(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "project query parameter is required")
}

func TestSecretsHandler_ListSecrets_NoUserContext(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, nil)
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSecretsHandler_ResponseNeverContainsValues(t *testing.T) {
	// NFR-S1/NFR-S2: Verify that secret values never appear in any response

	sensitiveValues := []string{"super-secret-password", "api-key-12345", "database-connection-string"}

	k8sClient := fake.NewSimpleClientset()
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// Create a secret with sensitive values
	body := CreateSecretRequest{
		Name:      "sensitive-secret",
		Namespace: "default",
		Data: map[string]string{
			"password":    sensitiveValues[0],
			"api-key":     sensitiveValues[1],
			"db-conn-str": sensitiveValues[2],
		},
	}

	// Check create response
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	for _, val := range sensitiveValues {
		assert.NotContains(t, rr.Body.String(), val, "create response must not contain secret value: %s", val)
	}

	// Check list response
	req = newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	for _, val := range sensitiveValues {
		assert.NotContains(t, rr.Body.String(), val, "list response must not contain secret value: %s", val)
	}
}

func TestSecretsHandler_CreateSecret_EnforcerError(t *testing.T) {
	// Simulate an infrastructure failure in the authorization system (e.g., Casbin storage unavailable).
	// ErrAccessDenied is a business error and would not be returned by CanAccessWithGroups as an error;
	// use a generic infrastructure error to accurately model this path.
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccessErr: errors.New("casbin: storage unavailable")},
	})

	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"k": "v"},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSecretsHandler_ListSecrets_EnforcerError(t *testing.T) {
	// Mirror of TestSecretsHandler_CreateSecret_EnforcerError for the list path.
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccessErr: errors.New("casbin: storage unavailable")},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSecretsHandler_ListSecrets_CrossProjectIsolation(t *testing.T) {
	// Verify that secrets from a different project are NOT returned (tenant isolation).
	k8sClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-secret",
				Namespace: "ns-1",
				Labels: map[string]string{
					"knodex.io/project":    "demo",
					"knodex.io/managed-by": "knodex",
				},
			},
			Data: map[string][]byte{"key": []byte("val")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-secret",
				Namespace: "ns-2",
				Labels: map[string]string{
					"knodex.io/project":    "other",
					"knodex.io/managed-by": "knodex",
				},
			},
			Data: map[string][]byte{"key": []byte("val")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unlabeled-secret",
				Namespace: "ns-3",
				// No knodex labels — should never appear
			},
			Data: map[string][]byte{"key": []byte("val")},
		},
	)

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	// Only demo-secret should be returned
	assert.Equal(t, 1, resp.TotalCount)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "demo-secret", resp.Items[0].Name)

	// other-secret and unlabeled-secret must NOT appear
	responseBody := rr.Body.String()
	assert.NotContains(t, responseBody, "other-secret")
	assert.NotContains(t, responseBody, "unlabeled-secret")
}

func TestSecretsHandler_InvalidProjectParam(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// Project names follow K8s DNS-1123 naming (lowercase only, no injection chars).
	// Note: spaces must be URL-encoded (%20) since httptest.NewRequest requires valid URLs.
	invalidProjects := []struct {
		label string
		query string // URL-safe representation
	}{
		{"label=injection", "demo%3Dinjected"},
		{"comma-separator injection", "demo%2Cother"},
		{"uppercase not allowed", "UPPERCASE"},
		{"space not allowed", "has%20space"},
		{"too long (>63 chars)", "toolong-toolong-toolong-toolong-toolong-toolong-toolong-toolong-toolong"},
	}

	for _, proj := range invalidProjects {
		t.Run("invalid project: "+proj.label, func(t *testing.T) {
			// Test both endpoints
			for _, method := range []string{"GET", "POST"} {
				var reqBody interface{}
				if method == "POST" {
					reqBody = CreateSecretRequest{Name: "s", Namespace: "default", Data: map[string]string{"k": "v"}}
				}
				req := newSecretsRequest(method, "/api/v1/secrets?project="+proj.query, reqBody, defaultUserCtx())
				rr := httptest.NewRecorder()
				if method == "POST" {
					handler.CreateSecret(rr, req)
				} else {
					handler.ListSecrets(rr, req)
				}
				assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 for project=%q method=%s", proj.label, method)
			}
		})
	}
}

func TestSecretsHandler_CreateSecret_InvalidNamespace(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name      string
		namespace string
	}{
		{"uppercase", "DefaultNamespace"},
		{"has space", "my namespace"},
		{"starts with hyphen", "-invalid"},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := CreateSecretRequest{
				Name:      "my-secret",
				Namespace: tt.namespace,
				Data:      map[string]string{"k": "v"},
			}
			req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
			rr := httptest.NewRecorder()
			handler.CreateSecret(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "namespace")
		})
	}
}

func TestSecretsHandler_CreateSecret_NameWithDots(t *testing.T) {
	// K8s allows dots in DNS-1123 subdomain names — Knodex must accept them too.
	k8sClient := fake.NewSimpleClientset()
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{
		Name:      "tls.cert",
		Namespace: "default",
		Data:      map[string]string{"cert": "value"},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "names with dots must be accepted")
}

// ============================================================
// GetSecret tests
// ============================================================

func TestSecretsHandler_GetSecret_Success(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{
			"password": []byte("s3cret"),
			"username": []byte("admin"),
		},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.GetSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SecretDetailResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "my-secret", resp.Name)
	assert.Equal(t, "default", resp.Namespace)
	assert.Equal(t, "s3cret", resp.Data["password"])
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "demo", resp.Labels["knodex.io/project"])
}

func TestSecretsHandler_GetSecret_NotFound(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets/nonexistent?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	handler.GetSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "NOT_FOUND")
}

func TestSecretsHandler_GetSecret_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: false},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.GetSecret(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_GetSecret_MissingParams(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name string
		url  string
		msg  string
	}{
		{"missing project", "/api/v1/secrets/my-secret?namespace=default", "project"},
		{"missing namespace", "/api/v1/secrets/my-secret?project=demo", "namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecretsRequest("GET", tt.url, nil, defaultUserCtx())
			req.SetPathValue("name", "my-secret")
			rr := httptest.NewRecorder()
			handler.GetSecret(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.msg)
		})
	}
}

func TestSecretsHandler_GetSecret_CrossProjectDenied(t *testing.T) {
	// C-1: Verify that GetSecret denies access to secrets belonging to a different project
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-project-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "sensitive-project",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("top-secret")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// User has access to "demo" project but tries to read a "sensitive-project" secret
	req := newSecretsRequest("GET", "/api/v1/secrets/other-project-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "other-project-secret")
	rr := httptest.NewRecorder()
	handler.GetSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "cross-project access must return 404")
	assert.NotContains(t, rr.Body.String(), "top-secret", "secret values must not leak")
}

func TestSecretsHandler_UpdateSecret_CrossProjectDenied(t *testing.T) {
	// C-1: Verify that UpdateSecret denies modification of secrets belonging to a different project
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-project-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "sensitive-project",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("top-secret")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := UpdateSecretRequest{
		Namespace: "default",
		Data:      map[string]string{"password": "hacked"},
	}
	req := newSecretsRequest("PUT", "/api/v1/secrets/other-project-secret?project=demo", body, defaultUserCtx())
	req.SetPathValue("name", "other-project-secret")
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "cross-project update must return 404")

	// Verify the secret was NOT modified
	secret, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), "other-project-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "top-secret", string(secret.Data["password"]), "secret must remain unchanged")
}

func TestSecretsHandler_DeleteSecret_CrossProjectDenied(t *testing.T) {
	// C-1: Verify that DeleteSecret denies deletion of secrets belonging to a different project
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-project-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "sensitive-project",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("top-secret")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/other-project-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "other-project-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "cross-project delete must return 404")

	// Verify the secret was NOT deleted
	_, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), "other-project-secret", metav1.GetOptions{})
	assert.NoError(t, err, "secret must still exist after cross-project delete attempt")
}

func TestSecretsHandler_GetSecret_NoUserContext(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, nil)
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.GetSecret(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ============================================================
// CheckSecretExists tests
// ============================================================

func TestSecretsHandler_CheckSecretExists_Found(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("s3cret")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("HEAD", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Body.String(), "HEAD response must have no body")
}

func TestSecretsHandler_CheckSecretExists_NotFound(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("HEAD", "/api/v1/secrets/nonexistent?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_CrossProjectDenied(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "sensitive-project",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"key": []byte("value")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// Request "demo" project but secret belongs to "sensitive-project" → 404
	req := newSecretsRequest("HEAD", "/api/v1/secrets/other-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "other-secret")
	rr := httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: false},
	})

	req := newSecretsRequest("HEAD", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_MissingParams(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name string
		url  string
	}{
		{"missing project", "/api/v1/secrets/my-secret?namespace=default"},
		{"missing namespace", "/api/v1/secrets/my-secret?project=demo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecretsRequest("HEAD", tt.url, nil, defaultUserCtx())
			req.SetPathValue("name", "my-secret")
			rr := httptest.NewRecorder()
			handler.CheckSecretExists(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

// ============================================================
// UpdateSecret tests
// ============================================================

func TestSecretsHandler_UpdateSecret_Success(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{
			"password": []byte("old-password"),
		},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := UpdateSecretRequest{
		Namespace: "default",
		Data:      map[string]string{"password": "new-password", "api-key": "abc123"},
	}

	req := newSecretsRequest("PUT", "/api/v1/secrets/my-secret?project=demo", body, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SecretResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "my-secret", resp.Name)
	assert.Equal(t, "default", resp.Namespace)
	assert.ElementsMatch(t, []string{"api-key", "password"}, resp.Keys)

	// Verify no secret values in response
	responseBody := rr.Body.String()
	assert.NotContains(t, responseBody, "new-password")
	assert.NotContains(t, responseBody, "abc123")
}

func TestSecretsHandler_UpdateSecret_NotFound(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := UpdateSecretRequest{
		Namespace: "default",
		Data:      map[string]string{"key": "value"},
	}

	req := newSecretsRequest("PUT", "/api/v1/secrets/nonexistent?project=demo", body, defaultUserCtx())
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_UpdateSecret_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: false},
	})

	body := UpdateSecretRequest{
		Namespace: "default",
		Data:      map[string]string{"key": "value"},
	}

	req := newSecretsRequest("PUT", "/api/v1/secrets/my-secret?project=demo", body, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_UpdateSecret_ValidationErrors(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		}),
		Enforcer: &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name string
		body UpdateSecretRequest
		msg  string
	}{
		{"empty namespace", UpdateSecretRequest{Namespace: "", Data: map[string]string{"k": "v"}}, "namespace"},
		{"empty data", UpdateSecretRequest{Namespace: "default", Data: map[string]string{}}, "data"},
		{"nil data", UpdateSecretRequest{Namespace: "default", Data: nil}, "data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecretsRequest("PUT", "/api/v1/secrets/my-secret?project=demo", tt.body, defaultUserCtx())
			req.SetPathValue("name", "my-secret")
			rr := httptest.NewRecorder()
			handler.UpdateSecret(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.msg)
		})
	}
}

func TestSecretsHandler_UpdateSecret_MissingProject(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := UpdateSecretRequest{Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("PUT", "/api/v1/secrets/my-secret", body, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "project")
}

// ============================================================
// DeleteSecret tests
// ============================================================

func TestSecretsHandler_DeleteSecret_Success_NoReferences(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("s3cret")},
	})

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp DeleteSecretResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Deleted)
	assert.Empty(t, resp.Warnings)

	// Verify secret was actually deleted
	_, err = k8sClient.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestSecretsHandler_DeleteSecret_NotFound(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/nonexistent?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_DeleteSecret_Unauthorized(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		}),
		Enforcer: &mockSecretsEnforcer{canAccess: false},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_DeleteSecret_MissingParams(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name string
		url  string
		msg  string
	}{
		{"missing project", "/api/v1/secrets/my-secret?namespace=default", "project"},
		{"missing namespace", "/api/v1/secrets/my-secret?project=demo", "namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecretsRequest("DELETE", tt.url, nil, defaultUserCtx())
			req.SetPathValue("name", "my-secret")
			rr := httptest.NewRecorder()
			handler.DeleteSecret(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.msg)
		})
	}
}

func TestSecretsHandler_DeleteSecret_NoUserContext(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, nil)
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSecretsHandler_DeleteSecret_WithReferences(t *testing.T) {
	// Verify AC#3: delete with Instance references returns warnings but still deletes
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("s3cret")},
	})

	// Create a fake kro.run Instance that references the secret via externalRef
	instance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Instance",
			"metadata": map[string]interface{}{
				"name":      "my-instance",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"externalRef": map[string]interface{}{
					"name": "my-secret",
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{Group: "kro.run", Version: "v1alpha1", Resource: "instances"}
	fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "InstanceList"},
		instance,
	)

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient:     k8sClient,
		DynamicClient: fakeDynClient,
		Enforcer:      &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp DeleteSecretResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Secret is deleted despite reference (non-blocking)
	assert.True(t, resp.Deleted)
	assert.Len(t, resp.Warnings, 1)
	assert.Contains(t, resp.Warnings[0], "my-instance")

	// Verify K8s secret was actually deleted
	_, err = k8sClient.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestContainsSecretReference(t *testing.T) {
	tests := []struct {
		name       string
		val        interface{}
		secretName string
		expected   bool
	}{
		// Bare strings outside a ref context are NOT matched (avoids false positives)
		{"bare string - no ref context - no match", "my-secret", "my-secret", false},
		{"no match string", "other-secret", "my-secret", false},
		// Nested ref patterns ARE matched (the intended use case)
		{"secretRef.name match", map[string]interface{}{
			"secretRef": map[string]interface{}{
				"name": "my-secret",
			},
		}, "my-secret", true},
		{"externalRef.name match", map[string]interface{}{
			"externalRef": map[string]interface{}{
				"name": "my-secret",
			},
		}, "my-secret", true},
		{"secretRef.name no match", map[string]interface{}{
			"secretRef": map[string]interface{}{
				"name": "other",
			},
		}, "my-secret", false},
		// Non-ref keys do NOT match even if value equals the secret name
		{"non-ref key no match", map[string]interface{}{
			"displayName": "my-secret",
		}, "my-secret", false},
		// Slices without ref context do NOT match (individual string items require key context)
		{"slice without ref context - no match", []interface{}{"other", "my-secret"}, "my-secret", false},
		{"nil value", nil, "my-secret", false},
		{"number value", 42, "my-secret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSecretReference(tt.val, tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateCreateSecretRequest(t *testing.T) {
	tests := []struct {
		name       string
		req        *CreateSecretRequest
		wantErrors []string
	}{
		{
			name: "valid request",
			req: &CreateSecretRequest{
				Name:      "my-secret",
				Namespace: "default",
				Data:      map[string]string{"key": "value"},
			},
			wantErrors: nil,
		},
		{
			name: "all fields empty",
			req: &CreateSecretRequest{
				Name:      "",
				Namespace: "",
				Data:      nil,
			},
			wantErrors: []string{"name", "namespace", "data"},
		},
		{
			name: "invalid DNS name with uppercase",
			req: &CreateSecretRequest{
				Name:      "Invalid-Name",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"name"},
		},
		{
			name: "name starting with hyphen",
			req: &CreateSecretRequest{
				Name:      "-invalid",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"name"},
		},
		{
			name: "valid name with dots (DNS-1123 subdomain)",
			req: &CreateSecretRequest{
				Name:      "tls.cert",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: nil,
		},
		{
			name: "consecutive dots rejected (K8s rejects a..b)",
			req: &CreateSecretRequest{
				Name:      "a..b",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"name"},
		},
		{
			name: "trailing hyphen before dot rejected (K8s rejects a-.b)",
			req: &CreateSecretRequest{
				Name:      "a-.b",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"name"},
		},
		{
			name: "leading hyphen after dot rejected (K8s rejects a.-b)",
			req: &CreateSecretRequest{
				Name:      "a.-b",
				Namespace: "default",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"name"},
		},
		{
			name: "invalid namespace with uppercase",
			req: &CreateSecretRequest{
				Name:      "my-secret",
				Namespace: "MyNamespace",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"namespace"},
		},
		{
			name: "namespace too long",
			req: &CreateSecretRequest{
				Name:      "my-secret",
				Namespace: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Data:      map[string]string{"k": "v"},
			},
			wantErrors: []string{"namespace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCreateSecretRequest(tt.req)
			if tt.wantErrors == nil {
				assert.Empty(t, errs)
			} else {
				for _, key := range tt.wantErrors {
					assert.Contains(t, errs, key, "expected validation error for key: %s", key)
				}
			}
		})
	}
}

// ============================================================
// K8s error type handling tests (AC#1)
// ============================================================

// newForbiddenError creates a K8s Forbidden status error
func newForbiddenError() *k8serrors.StatusError {
	return &k8serrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusForbidden,
		Reason:  metav1.StatusReasonForbidden,
		Message: "secrets is forbidden: User \"system:serviceaccount:knodex:knodex\" cannot create resource \"secrets\"",
	}}
}

// newUnauthorizedError creates a K8s Unauthorized status error
func newUnauthorizedError() *k8serrors.StatusError {
	return &k8serrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusUnauthorized,
		Reason:  metav1.StatusReasonUnauthorized,
		Message: "Unauthorized",
	}}
}

// fakeClientWithReactor creates a fake K8s client that returns the given error for a verb+resource.
func fakeClientWithReactor(verb, resource string, err error) *fake.Clientset {
	client := fake.NewSimpleClientset()
	client.PrependReactor(verb, resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return client
}

func TestSecretsHandler_K8sForbidden_Returns403(t *testing.T) {
	tests := []struct {
		name      string
		verb      string
		method    string
		url       string
		pathValue string
		body      interface{}
		handler   func(h *SecretsHandler) http.HandlerFunc
	}{
		{
			name:   "CreateSecret forbidden",
			verb:   "create",
			method: "POST",
			url:    "/api/v1/secrets?project=demo",
			body:   CreateSecretRequest{Name: "s", Namespace: "default", Data: map[string]string{"k": "v"}},
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.CreateSecret
			},
		},
		{
			name:   "ListSecrets forbidden",
			verb:   "list",
			method: "GET",
			url:    "/api/v1/secrets?project=demo",
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.ListSecrets
			},
		},
		{
			name:      "GetSecret forbidden",
			verb:      "get",
			method:    "GET",
			url:       "/api/v1/secrets/my-secret?project=demo&namespace=default",
			pathValue: "my-secret",
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.GetSecret
			},
		},
		{
			name:      "CheckSecretExists forbidden",
			verb:      "get",
			method:    "HEAD",
			url:       "/api/v1/secrets/my-secret?project=demo&namespace=default",
			pathValue: "my-secret",
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.CheckSecretExists
			},
		},
		{
			name:      "UpdateSecret forbidden (get phase)",
			verb:      "get",
			method:    "PUT",
			url:       "/api/v1/secrets/my-secret?project=demo",
			pathValue: "my-secret",
			body:      UpdateSecretRequest{Namespace: "default", Data: map[string]string{"k": "v"}},
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.UpdateSecret
			},
		},
		{
			name:      "DeleteSecret forbidden (get phase)",
			verb:      "get",
			method:    "DELETE",
			url:       "/api/v1/secrets/my-secret?project=demo&namespace=default",
			pathValue: "my-secret",
			handler: func(h *SecretsHandler) http.HandlerFunc {
				return h.DeleteSecret
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fakeClientWithReactor(tt.verb, "secrets", newForbiddenError())
			h := NewSecretsHandler(SecretsHandlerConfig{
				K8sClient: k8sClient,
				Enforcer:  &mockSecretsEnforcer{canAccess: true},
			})

			req := newSecretsRequest(tt.method, tt.url, tt.body, defaultUserCtx())
			if tt.pathValue != "" {
				req.SetPathValue("name", tt.pathValue)
			}
			rr := httptest.NewRecorder()
			tt.handler(h)(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code, "K8s Forbidden should return 403")
			if tt.method != "HEAD" {
				assert.Contains(t, rr.Body.String(), "service account lacks permission")
			}
		})
	}
}

func TestSecretsHandler_K8sUnauthorized_Returns403(t *testing.T) {
	// K8s Unauthorized (401) should also map to our 403 Forbidden response
	k8sClient := fakeClientWithReactor("create", "secrets", newUnauthorizedError())
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{Name: "s", Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "K8s Unauthorized should return 403")
	assert.Contains(t, rr.Body.String(), "service account lacks permission")
}

func TestSecretsHandler_CreateSecret_NamespaceNotFound(t *testing.T) {
	// When namespace doesn't exist, K8s returns NotFound for the create operation
	nsNotFoundErr := &k8serrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusNotFound,
		Reason:  metav1.StatusReasonNotFound,
		Message: "namespaces \"nonexistent\" not found",
	}}
	k8sClient := fakeClientWithReactor("create", "secrets", nsNotFoundErr)
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{Name: "s", Namespace: "nonexistent", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "namespace does not exist")
}

// ============================================================
// Timeout tests (AC#2)
// ============================================================

func TestSecretsHandler_TimeoutConstant(t *testing.T) {
	// Verify the timeout constant is set to 15 seconds as specified in AC#2
	assert.Equal(t, 15*time.Second, secretsOperationTimeout)
}

func TestSecretsHandler_ContextTimeout_Returns503(t *testing.T) {
	// Verify that a K8s server-side timeout (StatusReasonTimeout) returns 503
	timeoutErr := &k8serrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusGatewayTimeout,
		Reason:  metav1.StatusReasonTimeout,
		Message: "request timeout",
	}}
	k8sClient := fakeClientWithReactor("list", "secrets", timeoutErr)
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "timed out")
}

func TestSecretsHandler_ContextDeadlineExceeded_Returns503(t *testing.T) {
	// Verify that a context deadline exceeded (client-side 15s timeout) returns 503
	k8sClient := fakeClientWithReactor("list", "secrets", context.DeadlineExceeded)
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "timed out")
}

// ============================================================
// Depth limit and reference scan tests (AC#4)
// ============================================================

func TestSearchSecretRef_DepthLimit(t *testing.T) {
	// Build a deeply nested structure (100+ levels) that would match at the bottom
	var val interface{} = map[string]interface{}{
		"secretRef": map[string]interface{}{
			"name": "my-secret",
		},
	}
	// Wrap it in 100 layers of nesting
	for i := 0; i < 100; i++ {
		val = map[string]interface{}{
			"level": val,
		}
	}

	// The match is at depth ~102, which exceeds maxSearchDepth (50)
	assert.False(t, containsSecretReference(val, "my-secret"),
		"deeply nested reference beyond maxSearchDepth should not be found")

	// But a shallow reference should still work
	shallow := map[string]interface{}{
		"secretRef": map[string]interface{}{
			"name": "my-secret",
		},
	}
	assert.True(t, containsSecretReference(shallow, "my-secret"),
		"shallow reference within depth limit should be found")
}

func TestSecretsHandler_DeleteSecret_ReferenceScanTimeout(t *testing.T) {
	// Verify that reference scan timeout produces a warning but doesn't block deletion
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				"knodex.io/project":    "demo",
				"knodex.io/managed-by": "knodex",
			},
		},
		Data: map[string][]byte{"password": []byte("s3cret")},
	})

	// Create a dynamic client that simulates timeout by returning many instances
	// We use a canceled context to simulate timeout
	gvr := schema.GroupVersionResource{Group: "kro.run", Version: "v1alpha1", Resource: "instances"}
	instances := make([]runtime.Object, 0, 10)
	for i := 0; i < 10; i++ {
		instances = append(instances, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kro.run/v1alpha1",
				"kind":       "Instance",
				"metadata": map[string]interface{}{
					"name":      "instance-" + string(rune('a'+i)),
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"key": "value",
				},
			},
		})
	}
	fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "InstanceList"},
		instances...,
	)

	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient:     k8sClient,
		DynamicClient: fakeDynClient,
		Enforcer:      &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("DELETE", "/api/v1/secrets/my-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "my-secret")
	rr := httptest.NewRecorder()
	handler.DeleteSecret(rr, req)

	// Secret should still be deleted regardless of scan result
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp DeleteSecretResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Deleted)
}

// ============================================================
// Pagination tests (AC#5)
// ============================================================

func TestSecretsHandler_ListSecrets_DefaultPageSize(t *testing.T) {
	assert.Equal(t, 100, defaultSecretPageSize, "default page size must be 100")
	assert.Equal(t, 500, maxSecretPageSize, "max page size must be 500")
}

func TestSecretsHandler_ListSecrets_CustomLimit(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s1", Namespace: "ns",
				Labels: map[string]string{"knodex.io/project": "demo", "knodex.io/managed-by": "knodex"},
			},
			Data: map[string][]byte{"k": []byte("v")},
		},
	)
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// Request with custom limit
	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo&limit=10", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
	assert.False(t, resp.HasMore)
	assert.Empty(t, resp.Continue)
}

func TestSecretsHandler_ListSecrets_InvalidLimitIgnored(t *testing.T) {
	// Invalid limit values should be silently ignored (uses default)
	k8sClient := fake.NewSimpleClientset()
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	for _, limitStr := range []string{"abc", "-5", "0"} {
		req := newSecretsRequest("GET", "/api/v1/secrets?project=demo&limit="+limitStr, nil, defaultUserCtx())
		rr := httptest.NewRecorder()
		handler.ListSecrets(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "invalid limit=%q should not cause error", limitStr)
	}
}

func TestSecretsHandler_ListSecrets_PaginationResponseFields(t *testing.T) {
	// Verify the response includes the new pagination fields
	k8sClient := fake.NewSimpleClientset()
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.HasMore, "empty list should have hasMore=false")
	assert.Empty(t, resp.Continue, "empty list should have no continue token")
}

// ============================================================
// Secret data size validation tests (AC#6, AC#7)
// ============================================================

func TestSecretsHandler_CreateSecret_EmptyKey(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"": "value"},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "secret keys must not be empty")
}

func TestSecretsHandler_CreateSecret_OversizedValue(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	largeValue := strings.Repeat("x", MaxSecretValueSize+1)
	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"key": largeValue},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "256KB")
}

func TestSecretsHandler_CreateSecret_OversizedTotal(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// 3 values each under 256KB but totaling > 512KB
	value := strings.Repeat("x", 200*1024) // 200KB each, 600KB total
	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"a": value, "b": value, "c": value},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "512KB")
}

func TestSecretsHandler_CreateSecret_ValidSizes(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	// Value exactly at limit should pass
	value := strings.Repeat("x", MaxSecretValueSize)
	body := CreateSecretRequest{
		Name:      "my-secret",
		Namespace: "default",
		Data:      map[string]string{"key": value},
	}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSecretsHandler_UpdateSecret_SizeValidation(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-secret", Namespace: "default",
			Labels: map[string]string{"knodex.io/project": "demo", "knodex.io/managed-by": "knodex"},
		},
		Data: map[string][]byte{"old": []byte("val")},
	})
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})

	tests := []struct {
		name string
		data map[string]string
		code int
		msg  string
	}{
		{"empty key", map[string]string{"": "v"}, http.StatusBadRequest, "secret keys must not be empty"},
		{"oversized value", map[string]string{"k": strings.Repeat("x", MaxSecretValueSize+1)}, http.StatusBadRequest, "256KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := UpdateSecretRequest{Namespace: "default", Data: tt.data}
			req := newSecretsRequest("PUT", "/api/v1/secrets/my-secret?project=demo", body, defaultUserCtx())
			req.SetPathValue("name", "my-secret")
			rr := httptest.NewRecorder()
			handler.UpdateSecret(rr, req)

			assert.Equal(t, tt.code, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.msg)
		})
	}
}

// --- License Service Integration Tests (STORY-289) ---

func newLicensedSecretsHandler(ls services.LicenseService) *SecretsHandler {
	k8sClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-secret",
				Namespace: "default",
				Labels:    map[string]string{"knodex.io/project": "demo", "knodex.io/managed-by": "knodex"},
			},
			Data: map[string][]byte{"key1": []byte("val1")},
		},
	)
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		Enforcer:  &mockSecretsEnforcer{canAccess: true},
	})
	if ls != nil {
		handler.SetLicenseService(ls)
	}
	return handler
}

func TestSecretsHandler_License_ValidLicense_AllOperationsSucceed(t *testing.T) {
	ls := &mockLicenseService{
		licensed: true,
		features: map[string]bool{services.FeatureSecrets: true},
	}
	handler := newLicensedSecretsHandler(ls)

	// CREATE (write)
	body := CreateSecretRequest{Name: "new-secret", Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// LIST (read)
	req = newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// GET (read)
	req = newSecretsRequest("GET", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.GetSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// DELETE (write)
	req = newSecretsRequest("DELETE", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecretsHandler_License_GracePeriod_SucceedsWithWarningHeader(t *testing.T) {
	ls := &mockLicenseService{
		licensed:    true,
		gracePeriod: true,
		features:    map[string]bool{services.FeatureSecrets: true},
	}
	handler := newLicensedSecretsHandler(ls)

	// READ operation
	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "expired", rr.Header().Get("X-License-Warning"))

	// WRITE operation
	body := CreateSecretRequest{Name: "grace-secret", Namespace: "default", Data: map[string]string{"k": "v"}}
	req = newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "expired", rr.Header().Get("X-License-Warning"))
}

func TestSecretsHandler_License_ExpiredReadOnly_ReadsSucceedWritesFail(t *testing.T) {
	ls := &mockLicenseService{
		licensed:    false,
		readOnly:    true,
		features:    map[string]bool{}, // IsFeatureEnabled returns false (expired)
		hasFeatures: map[string]bool{services.FeatureSecrets: true},
	}
	handler := newLicensedSecretsHandler(ls)

	// READ: ListSecrets should succeed
	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// READ: GetSecret should succeed
	req = newSecretsRequest("GET", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.GetSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// READ: CheckSecretExists should succeed
	req = newSecretsRequest("HEAD", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// WRITE: CreateSecret should return 402
	body := CreateSecretRequest{Name: "new-secret", Namespace: "default", Data: map[string]string{"k": "v"}}
	req = newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")
	assert.Contains(t, rr.Body.String(), "license_expired")

	// WRITE: UpdateSecret should return 402
	updateBody := UpdateSecretRequest{Namespace: "default", Data: map[string]string{"k": "v2"}}
	req = newSecretsRequest("PUT", "/api/v1/secrets/existing-secret?project=demo", updateBody, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.UpdateSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "license_expired")

	// WRITE: DeleteSecret should return 402
	req = newSecretsRequest("DELETE", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "license_expired")
}

func TestSecretsHandler_License_NoLicense_AllOperationsReturn402(t *testing.T) {
	ls := &mockLicenseService{
		licensed:    false,
		features:    map[string]bool{},
		hasFeatures: map[string]bool{},
	}
	handler := newLicensedSecretsHandler(ls)

	// READ: ListSecrets should return 402
	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")
	assert.Contains(t, rr.Body.String(), "valid enterprise license")

	// READ: GetSecret should return 402
	req = newSecretsRequest("GET", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.GetSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")

	// READ: CheckSecretExists should return 402
	req = newSecretsRequest("HEAD", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)

	// WRITE: CreateSecret should return 402
	body := CreateSecretRequest{Name: "new-secret", Namespace: "default", Data: map[string]string{"k": "v"}}
	req = newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")

	// WRITE: UpdateSecret should return 402
	updateBody := UpdateSecretRequest{Namespace: "default", Data: map[string]string{"k": "v2"}}
	req = newSecretsRequest("PUT", "/api/v1/secrets/existing-secret?project=demo", updateBody, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.UpdateSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")

	// WRITE: DeleteSecret should return 402
	req = newSecretsRequest("DELETE", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")
}

func TestSecretsHandler_License_FeatureNotIncluded_Returns402(t *testing.T) {
	ls := &mockLicenseService{
		licensed:    true,
		features:    map[string]bool{services.FeatureCompliance: true}, // has compliance, NOT secrets
		hasFeatures: map[string]bool{services.FeatureCompliance: true},
	}
	handler := newLicensedSecretsHandler(ls)

	// READ: should return 402 with "not included in your license"
	req := newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "LICENSE_REQUIRED")
	assert.Contains(t, rr.Body.String(), "not included in your license")
}

func TestSecretsHandler_License_NilLicenseService_BackwardCompat(t *testing.T) {
	// When licenseService is nil (no license file configured), all operations should succeed
	handler := newLicensedSecretsHandler(nil) // nil license service

	// CREATE (write) - should succeed
	body := CreateSecretRequest{Name: "new-secret", Namespace: "default", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/secrets?project=demo", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// LIST (read) - should succeed
	req = newSecretsRequest("GET", "/api/v1/secrets?project=demo", nil, defaultUserCtx())
	rr = httptest.NewRecorder()
	handler.ListSecrets(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// GET (read) - should succeed
	req = newSecretsRequest("GET", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.GetSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// CHECK (read) - should succeed
	req = newSecretsRequest("HEAD", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// UPDATE (write) - should succeed
	updateBody := UpdateSecretRequest{Namespace: "default", Data: map[string]string{"k": "v2"}}
	req = newSecretsRequest("PUT", "/api/v1/secrets/existing-secret?project=demo", updateBody, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.UpdateSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// DELETE (write) - should succeed
	req = newSecretsRequest("DELETE", "/api/v1/secrets/existing-secret?project=demo&namespace=default", nil, defaultUserCtx())
	req.SetPathValue("name", "existing-secret")
	rr = httptest.NewRecorder()
	handler.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
