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
	"sync"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/models"
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

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mockNSAccessProvider implements NamespaceAccessProvider for secrets tests.
// "*" in the namespaces list signals global admin (matches any namespace via
// rbac.MatchNamespaceInList). An err is propagated to the handler unchanged so
// callers see a 500.
type mockNSAccessProvider struct {
	namespaces []string
	err        error
}

func (m *mockNSAccessProvider) GetAccessibleNamespaces(_ context.Context, _ *middleware.UserContext) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.namespaces, nil
}

// recordingAuditor captures every Event passed to RecordEvent so tests can
// inspect the audit lens (Project field) and the recorded resource details.
type recordingAuditor struct {
	mu     sync.Mutex
	events []audit.Event
}

func (r *recordingAuditor) Record(_ context.Context, e audit.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recordingAuditor) snapshot() []audit.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.Event, len(r.events))
	copy(out, r.events)
	return out
}

var _ audit.Recorder = (*recordingAuditor)(nil)

// newSecretsRequest builds an *http.Request with optional JSON body and an
// attached UserContext. Path params (namespace, name) are wired up by the
// caller via req.SetPathValue.
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

// newSecretsHandlerForTest is a small constructor that wires up sensible test
// defaults — admin NS access (so authorization always passes), a recording
// auditor for lens inspection, and an empty fake K8s client unless the caller
// supplies seed objects.
func newSecretsHandlerForTest(seeds ...runtime.Object) (*SecretsHandler, *recordingAuditor, *fake.Clientset) {
	rec := &recordingAuditor{}
	k8s := fake.NewSimpleClientset(seeds...)
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8s,
		Recorder:  rec,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})
	return h, rec, k8s
}

// makeRequestForNamespace builds the URL + path params for the namespace-keyed
// secret routes. For collection endpoints (list/create) name is empty.
func makeRequestForNamespace(method, namespace, name string, body interface{}, userCtx *middleware.UserContext) *http.Request {
	var url string
	switch {
	case name == "":
		url = "/api/v1/namespaces/" + namespace + "/secrets"
	default:
		url = "/api/v1/namespaces/" + namespace + "/secrets/" + name
	}
	req := newSecretsRequest(method, url, body, userCtx)
	if namespace != "" {
		req.SetPathValue("namespace", namespace)
	}
	if name != "" {
		req.SetPathValue("name", name)
	}
	return req
}

// ---------------------------------------------------------------------------
// CreateSecret
// ---------------------------------------------------------------------------

func TestSecretsHandler_CreateSecret_Success(t *testing.T) {
	h, rec, k8s := newSecretsHandlerForTest()

	body := CreateSecretRequest{
		Name: "my-secret",
		Data: map[string]string{"password": "s3cret", "username": "admin"},
	}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	req.Header.Set("X-Knodex-Project", "alpha") // audit lens
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var resp SecretResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "my-secret", resp.Name)
	assert.Equal(t, "default", resp.Namespace)
	assert.ElementsMatch(t, []string{"password", "username"}, resp.Keys)

	// AC5: K8s Secret carries ManagedByLabel but NOT ProjectLabel.
	created, err := k8s.CoreV1().Secrets("default").Get(req.Context(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, models.ManagedByValue, created.Labels[models.ManagedByLabel])
	_, hasProjectLabel := created.Labels[models.ProjectLabel]
	assert.False(t, hasProjectLabel, "Create must NOT stamp knodex.io/project on the Secret (TD-2)")

	// AC6: audit Event.Project reflects the caller's X-Knodex-Project lens.
	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "alpha", events[0].Project)
	assert.Equal(t, "success", events[0].Result)
}

func TestSecretsHandler_CreateSecret_MissingNamespacePathParam(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}

	// SetPathValue intentionally omitted — simulates a malformed mux pattern.
	req := newSecretsRequest("POST", "/api/v1/namespaces//secrets", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "namespace path parameter is required")
}

func TestSecretsHandler_CreateSecret_InvalidNamespace(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}

	// "UPPERCASE" violates DNS-1123 label rules.
	req := newSecretsRequest("POST", "/api/v1/namespaces/UPPERCASE/secrets", body, defaultUserCtx())
	req.SetPathValue("namespace", "UPPERCASE")
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "DNS-1123")
}

func TestSecretsHandler_CreateSecret_ValidationErrors(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()

	cases := []struct {
		name    string
		body    CreateSecretRequest
		wantSub string
	}{
		{"missing name", CreateSecretRequest{Data: map[string]string{"k": "v"}}, "name is required"},
		{"empty data", CreateSecretRequest{Name: "x", Data: map[string]string{}}, "at least one key-value pair"},
		{"empty data key", CreateSecretRequest{Name: "x", Data: map[string]string{"": "v"}}, "must not be empty"},
		{"bad key chars", CreateSecretRequest{Name: "x", Data: map[string]string{"k@y": "v"}}, "invalid characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := makeRequestForNamespace("POST", "default", "", tc.body, defaultUserCtx())
			rr := httptest.NewRecorder()
			h.CreateSecret(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tc.wantSub)
		})
	}
}

func TestSecretsHandler_CreateSecret_Duplicate(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dup", Namespace: "default"},
	}
	h, _, _ := newSecretsHandlerForTest(existing)

	body := CreateSecretRequest{Name: "dup", Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestSecretsHandler_CreateSecret_NamespaceDenied(t *testing.T) {
	rec := &recordingAuditor{}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		Recorder:  rec,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"only-shared"}},
	})

	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("POST", "kube-system", "", body, defaultUserCtx())
	req.Header.Set("X-Knodex-Project", "alpha")
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	// AC9-style: cross-namespace access without destinations → NotFound,
	// matching the instance handler's non-leak behavior.
	assert.Equal(t, http.StatusNotFound, rr.Code)

	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "denied", events[0].Result)
	assert.Equal(t, "alpha", events[0].Project, "audit lens preserved on denial")
}

func TestSecretsHandler_CreateSecret_NoUserContext(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
	req := newSecretsRequest("POST", "/api/v1/namespaces/default/secrets", body, nil)
	req.SetPathValue("namespace", "default")
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSecretsHandler_CreateSecret_AuditWithoutLensHeader(t *testing.T) {
	// AC7: when X-Knodex-Project header is absent, Event.Project is "".
	h, rec, _ := newSecretsHandlerForTest()
	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	// No X-Knodex-Project header set.
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "", events[0].Project, "absent lens header → empty audit project")
}

// TestSecretsHandler_AuditLens_Sanitization pins the F9 hardening:
// X-Knodex-Project is caller-controlled and never authorization-bearing,
// but flows verbatim into audit.Event.Project. We strip ASCII control
// characters (prevents audit-row log injection) and cap at auditLensMaxLen
// (prevents an oversized header from creating a giant audit row).
func TestSecretsHandler_AuditLens_Sanitization(t *testing.T) {
	cases := []struct {
		name       string
		headerVal  string
		wantLensEq string
	}{
		{
			name:       "control chars stripped",
			headerVal:  "alpha\nbeta\t\x00gamma",
			wantLensEq: "alphabetagamma",
		},
		{
			name:       "oversized value truncated to auditLensMaxLen",
			headerVal:  strings.Repeat("a", auditLensMaxLen+50),
			wantLensEq: strings.Repeat("a", auditLensMaxLen),
		},
		{
			name:       "exact-max value passes through unchanged",
			headerVal:  strings.Repeat("a", auditLensMaxLen),
			wantLensEq: strings.Repeat("a", auditLensMaxLen),
		},
		{
			name:       "empty header → empty lens (AC7)",
			headerVal:  "",
			wantLensEq: "",
		},
		{
			name:       "well-formed project name passes through (AC6)",
			headerVal:  "alpha",
			wantLensEq: "alpha",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec, _ := newSecretsHandlerForTest()
			body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
			req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
			if tc.headerVal != "" {
				req.Header.Set("X-Knodex-Project", tc.headerVal)
			}
			rr := httptest.NewRecorder()
			h.CreateSecret(rr, req)
			require.Equal(t, http.StatusCreated, rr.Code)

			events := rec.snapshot()
			require.Len(t, events, 1)
			assert.Equal(t, tc.wantLensEq, events[0].Project)
		})
	}
}

// ---------------------------------------------------------------------------
// ListSecrets
// ---------------------------------------------------------------------------

func TestSecretsHandler_ListSecrets_Admin_AllNamespaces(t *testing.T) {
	seed := []runtime.Object{
		mkSecret("s1", "ns-a"),
		mkSecret("s2", "ns-b"),
	}
	h, _, _ := newSecretsHandlerForTest(seed...)

	req := newSecretsRequest("GET", "/api/v1/secrets", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.PageCount)
}

func TestSecretsHandler_ListSecrets_FiltersByAccessibleNamespaces(t *testing.T) {
	// AC11: non-admin sees only secrets in their accessible namespaces.
	seed := []runtime.Object{
		mkSecret("s1", "xxx-shared"),
		mkSecret("s2", "xxx-apps"),
		mkSecret("s3", "xxx-other"),
	}
	rec := &recordingAuditor{}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed...),
		Recorder:  rec,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"xxx-shared", "xxx-apps"}},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.PageCount)
	got := make(map[string]bool)
	for _, item := range resp.Items {
		got[item.Namespace] = true
	}
	assert.True(t, got["xxx-shared"])
	assert.True(t, got["xxx-apps"])
	assert.False(t, got["xxx-other"], "xxx-other must not appear — not in user's accessible namespaces")
}

func TestSecretsHandler_ListSecrets_NamespaceFilter_Authorized(t *testing.T) {
	// AC12: ?namespace= narrows the list when the user has access to it.
	seed := []runtime.Object{
		mkSecret("s1", "xxx-shared"),
		mkSecret("s2", "xxx-apps"),
	}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed...),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"xxx-shared", "xxx-apps"}},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?namespace=xxx-shared", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.PageCount)
	assert.Equal(t, "xxx-shared", resp.Items[0].Namespace)
}

func TestSecretsHandler_ListSecrets_NamespaceFilter_Unauthorized_EmptyList(t *testing.T) {
	// AC13: ?namespace=<no-access> → empty list, not a 4xx (mirrors instances).
	seed := []runtime.Object{mkSecret("s", "xxx-other")}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed...),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"xxx-shared"}},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets?namespace=xxx-other", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.PageCount)
}

// TestSecretsHandler_ListSecrets_WildcardDestinationsSkipped pins the
// KNOWN LIMITATION documented in secrets_handler.go ListSecrets: when a
// user's destinations include a pattern (e.g., "staging-*"), the List
// path silently skips it — pattern destinations cannot be K8s LIST'd
// directly without listing every namespace in the cluster. The
// asymmetry: GET/PUT/DELETE on concrete matching namespaces still work
// (authorizeSecretAccess uses rbac.MatchNamespaceInList). This test
// pins the silent-skip behavior so a future change either preserves it
// (no regression) or removes the limitation (failing test → flip the
// assertion).
func TestSecretsHandler_ListSecrets_WildcardDestinationsSkipped(t *testing.T) {
	seed := []runtime.Object{
		mkSecret("s1", "staging-team-a"),
		mkSecret("s2", "staging-team-b"),
	}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed...),
		Recorder:  &recordingAuditor{},
		// Pattern destination: matches both seeded namespaces in principle,
		// but cannot be issued as a K8s LIST directly.
		NSAccess: &mockNSAccessProvider{namespaces: []string{"staging-*"}},
	})

	req := newSecretsRequest("GET", "/api/v1/secrets", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equalf(t, 0, resp.PageCount,
		"pattern destinations contribute 0 list results (known limitation); "+
			"single-resource verbs still succeed via MatchNamespaceInList")
}

func TestSecretsHandler_ListSecrets_NoUserContext(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := newSecretsRequest("GET", "/api/v1/secrets", nil, nil)
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSecretsHandler_ListSecrets_InvalidNamespaceFilter(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := newSecretsRequest("GET", "/api/v1/secrets?namespace=BAD_NS", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.ListSecrets(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// GetSecret
// ---------------------------------------------------------------------------

func TestSecretsHandler_GetSecret_Success(t *testing.T) {
	seed := mkSecretWithData("api-key", "xxx-shared", map[string][]byte{"token": []byte("hunter2")})
	h, rec, _ := newSecretsHandlerForTest(seed)

	req := makeRequestForNamespace("GET", "xxx-shared", "api-key", nil, defaultUserCtx())
	req.Header.Set("X-Knodex-Project", "alpha")
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp SecretDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "api-key", resp.Name)
	assert.Equal(t, "hunter2", resp.Data["token"])

	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "alpha", events[0].Project)
}

func TestSecretsHandler_GetSecret_AC1_ProjectScopedRead(t *testing.T) {
	// AC1: a single user with destinations matching the secret's namespace
	// can read it. (The actual Casbin enforcement is middleware-level; this
	// test pins the handler's defense-in-depth + audit shape.)
	seed := mkSecret("my-secret", "xxx-shared")
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"xxx-shared"}},
	})

	req := makeRequestForNamespace("GET", "xxx-shared", "my-secret", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecretsHandler_GetSecret_AC2_SharedNamespaceRead(t *testing.T) {
	// AC2: two distinct users (different project roles) BOTH succeed when
	// their destinations list the same namespace. The handler does not know
	// or care which project a user "belongs to" — the namespace alone gates
	// access.
	seed := mkSecret("shared-secret", "xxx-shared")
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"xxx-shared"}},
	})

	for _, lens := range []string{"alpha", "beta"} {
		req := makeRequestForNamespace("GET", "xxx-shared", "shared-secret", nil, defaultUserCtx())
		req.Header.Set("X-Knodex-Project", lens)
		rr := httptest.NewRecorder()
		h.GetSecret(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "lens=%s should succeed", lens)
	}
}

func TestSecretsHandler_GetSecret_AC9_CrossNamespaceDenied(t *testing.T) {
	// AC9: user without destination access to xxx-shared → NotFound, not 403.
	seed := mkSecret("private", "xxx-shared")
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"gamma-apps"}},
	})

	req := makeRequestForNamespace("GET", "xxx-shared", "private", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_GetSecret_AC10_ServerAdminBypass(t *testing.T) {
	// AC10: admin (NSAccess returns ["*"]) reads from any namespace.
	seed := mkSecret("any", "anywhere")
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	req := makeRequestForNamespace("GET", "anywhere", "any", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecretsHandler_GetSecret_NotFound(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := makeRequestForNamespace("GET", "default", "missing", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_GetSecret_MissingPathParams(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()

	cases := []struct {
		name string
		path string // unmounted path values
	}{
		{"empty namespace", "/api/v1/namespaces//secrets/x"},
		{"empty name", "/api/v1/namespaces/default/secrets/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newSecretsRequest("GET", tc.path, nil, defaultUserCtx())
			if tc.name == "empty name" {
				req.SetPathValue("namespace", "default")
			}
			rr := httptest.NewRecorder()
			h.GetSecret(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestSecretsHandler_GetSecret_NSAccessError_FailsClosedAs500(t *testing.T) {
	rec := &recordingAuditor{}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(mkSecret("x", "ns")),
		Recorder:  rec,
		NSAccess:  &mockNSAccessProvider{err: errors.New("provider unavailable")},
	})

	req := makeRequestForNamespace("GET", "ns", "x", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)

	// Matches instance_crud.go behavior: 500 when accessible-namespaces
	// determination fails (caller can retry; we don't leak "denied" vs
	// "broken").
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSecretsHandler_GetSecret_NoUserContext(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := makeRequestForNamespace("GET", "default", "x", nil, nil)
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ---------------------------------------------------------------------------
// CheckSecretExists (HEAD)
// ---------------------------------------------------------------------------

func TestSecretsHandler_CheckSecretExists_Found(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest(mkSecret("x", "default"))
	req := makeRequestForNamespace("HEAD", "default", "x", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_NotFound(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := makeRequestForNamespace("HEAD", "default", "missing", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_NamespaceDenied(t *testing.T) {
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(mkSecret("x", "xxx-shared")),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"only-mine"}},
	})

	req := makeRequestForNamespace("HEAD", "xxx-shared", "x", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CheckSecretExists(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_CheckSecretExists_MissingParams(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := newSecretsRequest("HEAD", "/api/v1/namespaces//secrets/", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CheckSecretExists(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// UpdateSecret
// ---------------------------------------------------------------------------

func TestSecretsHandler_UpdateSecret_Success(t *testing.T) {
	seed := mkSecretWithData("api-key", "default", map[string][]byte{"token": []byte("old")})
	h, rec, k8s := newSecretsHandlerForTest(seed)

	body := UpdateSecretRequest{Data: map[string]string{"token": "new"}}
	req := makeRequestForNamespace("PUT", "default", "api-key", body, defaultUserCtx())
	req.Header.Set("X-Knodex-Project", "alpha")
	rr := httptest.NewRecorder()
	h.UpdateSecret(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	updated, err := k8s.CoreV1().Secrets("default").Get(req.Context(), "api-key", metav1.GetOptions{})
	require.NoError(t, err)
	// fake K8s preserves StringData rather than collapsing it into Data —
	// the handler unions them when computing keys, mirroring real K8s.
	assert.Equal(t, "new", updated.StringData["token"])
	assert.NotEmpty(t, updated.Annotations[updatedAtAnnotation], "updatedAt annotation stamped")

	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "alpha", events[0].Project)
}

func TestSecretsHandler_UpdateSecret_ValidationErrors(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest(mkSecret("x", "default"))

	cases := []struct {
		name string
		body UpdateSecretRequest
		want string
	}{
		{"empty data", UpdateSecretRequest{Data: map[string]string{}}, "at least one key-value"},
		{"bad key", UpdateSecretRequest{Data: map[string]string{"$": "v"}}, "invalid characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := makeRequestForNamespace("PUT", "default", "x", tc.body, defaultUserCtx())
			rr := httptest.NewRecorder()
			h.UpdateSecret(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), tc.want)
		})
	}
}

func TestSecretsHandler_UpdateSecret_NamespaceDenied(t *testing.T) {
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(mkSecret("x", "xxx-shared")),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"only-mine"}},
	})

	body := UpdateSecretRequest{Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("PUT", "xxx-shared", "x", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.UpdateSecret(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_UpdateSecret_NotFound(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	body := UpdateSecretRequest{Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("PUT", "default", "ghost", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.UpdateSecret(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_UpdateSecret_MissingPathParams(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	body := UpdateSecretRequest{Data: map[string]string{"k": "v"}}

	req := newSecretsRequest("PUT", "/api/v1/namespaces//secrets/x", body, defaultUserCtx())
	req.SetPathValue("name", "x")
	rr := httptest.NewRecorder()
	h.UpdateSecret(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// DeleteSecret
// ---------------------------------------------------------------------------

func TestSecretsHandler_DeleteSecret_Success_NoReferences(t *testing.T) {
	seed := mkSecret("doomed", "default")
	rec := &recordingAuditor{}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(seed),
		Recorder:  rec,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	req := makeRequestForNamespace("DELETE", "default", "doomed", nil, defaultUserCtx())
	req.Header.Set("X-Knodex-Project", "alpha")
	rr := httptest.NewRecorder()
	h.DeleteSecret(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp DeleteSecretResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Deleted)
	assert.Empty(t, resp.Warnings)

	events := rec.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, "alpha", events[0].Project)
}

func TestSecretsHandler_DeleteSecret_WithReferences(t *testing.T) {
	seed := mkSecret("doomed", "default")

	// Build a fake dynamic client with one Instance that references "doomed".
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "kro.run", Version: "v1alpha1", Kind: "Instance"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "kro.run", Version: "v1alpha1", Kind: "InstanceList"}, &unstructured.UnstructuredList{})

	instance := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kro.run/v1alpha1",
		"kind":       "Instance",
		"metadata":   map[string]interface{}{"name": "uses-secret", "namespace": "default"},
		"spec": map[string]interface{}{
			"secretRef": "doomed",
		},
	}}
	gvrToListKind := map[schema.GroupVersionResource]string{
		kroInstanceGVR: "InstanceList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, instance)

	rec := &recordingAuditor{}
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient:     fake.NewSimpleClientset(seed),
		DynamicClient: dyn,
		Recorder:      rec,
		NSAccess:      &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	req := makeRequestForNamespace("DELETE", "default", "doomed", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.DeleteSecret(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp DeleteSecretResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Deleted)
	require.NotEmpty(t, resp.Warnings, "instance reference should produce warning")
	assert.Contains(t, resp.Warnings[0], "uses-secret")
}

func TestSecretsHandler_DeleteSecret_NotFound(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := makeRequestForNamespace("DELETE", "default", "ghost", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_DeleteSecret_NamespaceDenied(t *testing.T) {
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(mkSecret("x", "prod")),
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"staging"}},
	})

	req := makeRequestForNamespace("DELETE", "prod", "x", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSecretsHandler_DeleteSecret_MissingPathParams(t *testing.T) {
	h, _, _ := newSecretsHandlerForTest()
	req := newSecretsRequest("DELETE", "/api/v1/namespaces//secrets/", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.DeleteSecret(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// K8s error mapping (subset — exhaustive coverage of all 4 verbs would
// duplicate the same shape with no new signal).
// ---------------------------------------------------------------------------

func TestSecretsHandler_K8sForbidden_Returns403(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	k8s.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "x", errors.New("forbidden"))
	})
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8s,
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSecretsHandler_K8sTimeout_Returns503(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	k8s.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, context.DeadlineExceeded
	})
	h := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8s,
		Recorder:  &recordingAuditor{},
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	body := CreateSecretRequest{Name: "x", Data: map[string]string{"k": "v"}}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// ---------------------------------------------------------------------------
// Response invariants
// ---------------------------------------------------------------------------

func TestSecretsHandler_ResponseNeverContainsValues(t *testing.T) {
	// Create path: response carries only keys; values must never leak.
	h, _, _ := newSecretsHandlerForTest()
	body := CreateSecretRequest{
		Name: "x",
		Data: map[string]string{"password": "TOPSECRET123", "username": "alice"},
	}
	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.CreateSecret(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)
	assert.NotContains(t, rr.Body.String(), "TOPSECRET123")

	// List path: same invariant for the SecretResponse items.
	rrList := httptest.NewRecorder()
	h.ListSecrets(rrList, newSecretsRequest("GET", "/api/v1/secrets", nil, defaultUserCtx()))
	require.Equal(t, http.StatusOK, rrList.Code)
	assert.NotContains(t, rrList.Body.String(), "TOPSECRET123")
}

func TestSecretsHandler_GetSecret_UpdatedAtAbsentWhenNeverUpdated(t *testing.T) {
	seed := mkSecret("never-updated", "default")
	seed.CreationTimestamp = metav1.Time{Time: time.Now().Add(-time.Hour)}
	h, _, _ := newSecretsHandlerForTest(seed)

	req := makeRequestForNamespace("GET", "default", "never-updated", nil, defaultUserCtx())
	rr := httptest.NewRecorder()
	h.GetSecret(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp SecretDetailResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Nil(t, resp.UpdatedAt)
	assert.NotContains(t, rr.Body.String(), "updatedAt")
}

// ---------------------------------------------------------------------------
// Reference scanning (containsSecretReference)
// ---------------------------------------------------------------------------

func TestContainsSecretReference(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]interface{}
		want bool
	}{
		{
			"direct secretRef match",
			map[string]interface{}{"secretRef": "my-secret"},
			true,
		},
		{
			"nested envFromSecretRef",
			map[string]interface{}{
				"envFromSecretRef": map[string]interface{}{"name": "my-secret"},
			},
			true,
		},
		{
			"unrelated description with same string is not a hit",
			map[string]interface{}{"description": "my-secret"},
			false,
		},
		{
			"deep nesting in arrays",
			map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"envSecretRef": []interface{}{
							map[string]interface{}{"name": "my-secret"},
						},
					},
				},
			},
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containsSecretReference(tc.spec, "my-secret")
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Validator helpers
// ---------------------------------------------------------------------------

func TestValidateCreateSecretRequest(t *testing.T) {
	tests := []struct {
		name string
		req  CreateSecretRequest
		want []string // substrings expected to appear in errors
	}{
		{
			"all valid",
			CreateSecretRequest{Name: "my-secret", Data: map[string]string{"k": "v"}},
			nil,
		},
		{
			"missing name",
			CreateSecretRequest{Data: map[string]string{"k": "v"}},
			[]string{"name is required"},
		},
		{
			"invalid name",
			CreateSecretRequest{Name: "Invalid_Name", Data: map[string]string{"k": "v"}},
			[]string{"DNS-1123"},
		},
		{
			"missing data",
			CreateSecretRequest{Name: "ok"},
			[]string{"at least one"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateCreateSecretRequest(&tc.req)
			if len(tc.want) == 0 {
				assert.Empty(t, errs)
				return
			}
			combined := strings.Join(collectMapValues(errs), "|")
			for _, sub := range tc.want {
				assert.Contains(t, combined, sub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Small fixture helpers
// ---------------------------------------------------------------------------

func mkSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{models.ManagedByLabel: models.ManagedByValue},
		},
	}
}

func mkSecretWithData(name, namespace string, data map[string][]byte) *corev1.Secret {
	s := mkSecret(name, namespace)
	s.Data = data
	return s
}

func collectMapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
