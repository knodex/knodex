// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// mockRecorder captures recorded events for testing.
type mockRecorder struct {
	events []Event
}

func (m *mockRecorder) Record(_ context.Context, event Event) {
	m.events = append(m.events, event)
}

func TestRecordEvent_NilRecorder(t *testing.T) {
	t.Parallel()

	// Must not panic when recorder is nil (OSS builds)
	RecordEvent(nil, context.Background(), Event{
		Action:   "create",
		Resource: "projects",
		Name:     "test-project",
		Result:   "success",
	})
}

func TestRecordEvent_WithRecorder(t *testing.T) {
	t.Parallel()

	mock := &mockRecorder{}
	ctx := context.Background()

	RecordEvent(mock, ctx, Event{
		UserID:    "user-1",
		UserEmail: "admin@test.local",
		SourceIP:  "10.0.0.1",
		Action:    "delete",
		Resource:  "instances",
		Name:      "my-instance",
		Project:   "alpha",
		Namespace: "alpha-ns",
		RequestID: "req-123",
		Result:    "success",
		Details:   map[string]any{"rgdName": "webapp"},
	})

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	e := mock.events[0]
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Resource != "instances" {
		t.Errorf("expected resource 'instances', got %q", e.Resource)
	}
	if e.Name != "my-instance" {
		t.Errorf("expected name 'my-instance', got %q", e.Name)
	}
	if e.UserEmail != "admin@test.local" {
		t.Errorf("expected email 'admin@test.local', got %q", e.UserEmail)
	}
	if e.Details["rgdName"] != "webapp" {
		t.Errorf("expected rgdName 'webapp' in details, got %v", e.Details["rgdName"])
	}
}

func TestSourceIP_XForwardedFor(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if got := SourceIP(r); got != "1.2.3.4" {
		t.Errorf("expected '1.2.3.4', got %q", got)
	}
}

func TestSourceIP_XForwardedForSingle(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "5.6.7.8")
	if got := SourceIP(r); got != "5.6.7.8" {
		t.Errorf("expected '5.6.7.8', got %q", got)
	}
}

func TestSourceIP_XRealIP(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "9.8.7.6")
	if got := SourceIP(r); got != "9.8.7.6" {
		t.Errorf("expected '9.8.7.6', got %q", got)
	}
}

func TestSourceIP_RemoteAddr(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:54321"
	if got := SourceIP(r); got != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got %q", got)
	}
}

func TestSourceIP_RemoteAddrNoPort(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1"
	if got := SourceIP(r); got != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got %q", got)
	}
}

// --- SanitizeDetails tests ---

func TestSanitizeDetails_NilInput(t *testing.T) {
	t.Parallel()
	if result := SanitizeDetails(nil); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestSanitizeDetails_EmptyMap(t *testing.T) {
	t.Parallel()
	result := SanitizeDetails(map[string]any{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestSanitizeDetails_RemovesAllSecretFields(t *testing.T) {
	secretKeys := []string{
		"privateKey", "password", "bearerToken", "token",
		"secret", "tlsClientCert", "tlsClientKey", "clientSecret",
	}
	withRedactFields(secretKeys, func() {
		input := map[string]any{
			"repoURL":       "https://github.com/example/repo",
			"authType":      "ssh",
			"privateKey":    "-----BEGIN RSA PRIVATE KEY-----",
			"password":      "hunter2",
			"bearerToken":   "ghp_xxxx",
			"token":         "some-token",
			"secret":        "some-secret",
			"tlsClientCert": "cert-data",
			"tlsClientKey":  "key-data",
			"clientSecret":  "oauth-secret",
		}

		result := SanitizeDetails(input)

		// Safe fields should remain
		if result["repoURL"] != "https://github.com/example/repo" {
			t.Errorf("expected repoURL preserved, got %v", result["repoURL"])
		}
		if result["authType"] != "ssh" {
			t.Errorf("expected authType preserved, got %v", result["authType"])
		}

		// Secret fields should be removed
		for _, key := range secretKeys {
			if _, exists := result[key]; exists {
				t.Errorf("expected %q to be removed", key)
			}
		}

		if len(result) != 2 {
			t.Errorf("expected 2 keys, got %d: %v", len(result), result)
		}
	})
}

func TestSanitizeDetails_PreservesAllSafeFields(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"repoURL":       "https://github.com/example/repo",
		"authType":      "https",
		"defaultBranch": "main",
		"rgdName":       "my-rgd",
		"kind":          "MyResource",
		"description":   "test project",
	}

	result := SanitizeDetails(input)

	if len(result) != len(input) {
		t.Errorf("expected %d keys, got %d", len(input), len(result))
	}
	for k, v := range input {
		if result[k] != v {
			t.Errorf("expected %q=%v, got %v", k, v, result[k])
		}
	}
}

func TestSanitizeDetails_DoesNotMutateOriginal(t *testing.T) {
	withRedactFields([]string{"privateKey"}, func() {
		input := map[string]any{
			"repoURL":    "https://github.com/example/repo",
			"privateKey": "secret-key",
		}

		_ = SanitizeDetails(input)

		if _, exists := input["privateKey"]; !exists {
			t.Error("SanitizeDetails mutated the original map")
		}
	})
}

// --- SafeChanges tests ---

func TestSafeChanges_StringValues(t *testing.T) {
	t.Parallel()
	result := SafeChanges("git@old.com:repo", "git@new.com:repo")

	if result["old"] != "git@old.com:repo" {
		t.Errorf("expected old value, got %v", result["old"])
	}
	if result["new"] != "git@new.com:repo" {
		t.Errorf("expected new value, got %v", result["new"])
	}
	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
}

func TestSafeChanges_NilOldValue(t *testing.T) {
	t.Parallel()
	result := SafeChanges(nil, "new-value")
	if result["old"] != nil {
		t.Errorf("expected nil old value, got %v", result["old"])
	}
	if result["new"] != "new-value" {
		t.Errorf("expected new-value, got %v", result["new"])
	}
}

func TestSafeChanges_IntValues(t *testing.T) {
	t.Parallel()
	result := SafeChanges(30, 90)
	if result["old"] != 30 {
		t.Errorf("expected 30, got %v", result["old"])
	}
	if result["new"] != 90 {
		t.Errorf("expected 90, got %v", result["new"])
	}
}

func TestSafeChanges_BoolValues(t *testing.T) {
	t.Parallel()
	result := SafeChanges(false, true)
	if result["old"] != false {
		t.Errorf("expected false, got %v", result["old"])
	}
	if result["new"] != true {
		t.Errorf("expected true, got %v", result["new"])
	}
}

// --- SanitizeDetails recursive tests ---

func TestSanitizeDetails_RecursiveNestedMap(t *testing.T) {
	withRedactFields([]string{"privateKey"}, func() {
		input := map[string]any{
			"changes": map[string]any{
				"repoURL":    "safe-url",
				"privateKey": "should-be-removed",
			},
			"authType": "ssh",
		}

		result := SanitizeDetails(input)

		if result["authType"] != "ssh" {
			t.Errorf("expected authType preserved, got %v", result["authType"])
		}

		nested, ok := result["changes"].(map[string]any)
		if !ok {
			t.Fatalf("expected changes to be a map, got %T", result["changes"])
		}
		if nested["repoURL"] != "safe-url" {
			t.Errorf("expected repoURL preserved in nested map, got %v", nested["repoURL"])
		}
		if _, exists := nested["privateKey"]; exists {
			t.Error("expected privateKey to be removed from nested map")
		}
	})
}

func TestSanitizeDetails_DeeplyNestedSecrets(t *testing.T) {
	withRedactFields([]string{"password"}, func() {
		input := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"safe":     "value",
					"password": "deep-secret",
				},
			},
		}

		result := SanitizeDetails(input)

		l1, _ := result["level1"].(map[string]any)
		l2, _ := l1["level2"].(map[string]any)
		if l2["safe"] != "value" {
			t.Errorf("expected safe value preserved, got %v", l2["safe"])
		}
		if _, exists := l2["password"]; exists {
			t.Error("expected password to be removed from deeply nested map")
		}
	})
}

// --- IsSecretField / SecretFieldNames tests ---
//
// The redaction set is driven entirely by AUDIT_REDACT_FIELDS.
// These tests manipulate the package-level secretFieldNames directly
// to avoid env var side-effects between parallel tests.

// withRedactFields temporarily replaces the redaction set for a single test.
// Not safe for t.Parallel() — callers must not use Parallel.
func withRedactFields(fields []string, fn func()) {
	orig := secretFieldNames
	secretFieldNames = make(map[string]bool, len(fields))
	for _, f := range fields {
		secretFieldNames[strings.ToLower(f)] = true
	}
	defer func() { secretFieldNames = orig }()
	fn()
}

func TestIsSecretField_CaseInsensitive(t *testing.T) {
	withRedactFields([]string{"privateKey", "password", "bearerToken", "clientSecret"}, func() {
		cases := []string{
			"PRIVATEKEY", "PrivateKey", "privatekey",
			"PASSWORD", "Password", "passWord",
			"BEARERTOKEN", "BearerToken", "bearertoken",
			"CLIENTSECRET", "ClientSecret", "clientsecret",
		}
		for _, key := range cases {
			if !IsSecretField(key) {
				t.Errorf("expected %q to be detected as secret (case-insensitive)", key)
			}
		}
	})
}

func TestIsSecretField_SafeFieldsNotBlocked(t *testing.T) {
	withRedactFields([]string{"password", "secret"}, func() {
		safe := []string{"repoURL", "authType", "description", "rgdName", "kind"}
		for _, key := range safe {
			if IsSecretField(key) {
				t.Errorf("expected %q to NOT be a secret field", key)
			}
		}
	})
}

func TestIsSecretField_EmptySet(t *testing.T) {
	withRedactFields(nil, func() {
		if IsSecretField("password") {
			t.Error("expected no fields redacted with empty set")
		}
	})
}

func TestSecretFieldNames_ReturnsCopy(t *testing.T) {
	withRedactFields([]string{"password", "token"}, func() {
		names := SecretFieldNames()
		if len(names) != 2 {
			t.Errorf("expected 2 fields, got %d", len(names))
		}
		// Mutating the copy must not affect the original
		names["injected"] = true
		if IsSecretField("injected") {
			t.Error("SecretFieldNames did not return an independent copy")
		}
	})
}

func TestSanitizeDetails_CaseInsensitiveRemoval(t *testing.T) {
	withRedactFields([]string{"password", "privateKey", "token"}, func() {
		input := map[string]any{
			"repoURL":    "safe",
			"PASSWORD":   "should-be-removed",
			"PrivateKey": "should-be-removed",
			"TOKEN":      "should-be-removed",
		}

		result := SanitizeDetails(input)

		if result["repoURL"] != "safe" {
			t.Errorf("expected repoURL preserved, got %v", result["repoURL"])
		}
		for _, key := range []string{"PASSWORD", "PrivateKey", "TOKEN"} {
			if _, exists := result[key]; exists {
				t.Errorf("expected %q to be removed (case-insensitive)", key)
			}
		}
		if len(result) != 1 {
			t.Errorf("expected 1 key, got %d: %v", len(result), result)
		}
	})
}

func TestSanitizeDetails_CustomOperatorFields(t *testing.T) {
	withRedactFields([]string{"apiKey", "connectionString"}, func() {
		input := map[string]any{
			"repoURL":          "safe",
			"apiKey":           "should-be-removed",
			"CONNECTIONSTRING": "should-be-removed",
			"description":      "safe",
		}

		result := SanitizeDetails(input)

		if len(result) != 2 {
			t.Errorf("expected 2 keys, got %d: %v", len(result), result)
		}
		if result["repoURL"] != "safe" || result["description"] != "safe" {
			t.Errorf("expected safe fields preserved, got %v", result)
		}
	})
}

// --- SanitizeDetails slice recursion tests (M3) ---

func TestSanitizeDetails_SliceWithNestedMaps(t *testing.T) {
	withRedactFields([]string{"password"}, func() {
		input := map[string]any{
			"items": []any{
				map[string]any{
					"name":     "safe",
					"password": "secret",
				},
				map[string]any{
					"url":      "https://example.com",
					"password": "another-secret",
				},
			},
		}

		result := SanitizeDetails(input)

		items, ok := result["items"].([]any)
		if !ok {
			t.Fatalf("expected items to be []any, got %T", result["items"])
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}

		item0, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("expected item[0] to be map, got %T", items[0])
		}
		if item0["name"] != "safe" {
			t.Errorf("expected name preserved, got %v", item0["name"])
		}
		if _, exists := item0["password"]; exists {
			t.Error("expected password removed from item[0]")
		}

		item1, ok := items[1].(map[string]any)
		if !ok {
			t.Fatalf("expected item[1] to be map, got %T", items[1])
		}
		if item1["url"] != "https://example.com" {
			t.Errorf("expected url preserved, got %v", item1["url"])
		}
		if _, exists := item1["password"]; exists {
			t.Error("expected password removed from item[1]")
		}
	})
}

func TestSanitizeDetails_SliceWithPrimitives(t *testing.T) {
	withRedactFields([]string{"password"}, func() {
		input := map[string]any{
			"tags": []any{"tag1", "tag2", "tag3"},
		}

		result := SanitizeDetails(input)

		tags, ok := result["tags"].([]any)
		if !ok {
			t.Fatalf("expected tags to be []any, got %T", result["tags"])
		}
		if len(tags) != 3 || tags[0] != "tag1" || tags[1] != "tag2" || tags[2] != "tag3" {
			t.Errorf("expected primitive slice preserved, got %v", tags)
		}
	})
}

// --- RecordEvent centralized sanitization test ---

type testRecorder struct {
	events []Event
}

func (r *testRecorder) Record(_ context.Context, event Event) {
	r.events = append(r.events, event)
}

func TestRecordEvent_SanitizesDetails(t *testing.T) {
	withRedactFields([]string{"password", "token"}, func() {
		rec := &testRecorder{}
		ctx := context.Background()

		RecordEvent(rec, ctx, Event{
			Action:   "create",
			Resource: "repositories",
			Details: map[string]any{
				"repoURL":  "https://github.com/org/repo",
				"password": "should-be-removed",
				"nested": map[string]any{
					"token": "should-be-removed",
					"safe":  "value",
				},
			},
		})

		if len(rec.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(rec.events))
		}

		details := rec.events[0].Details
		if details["repoURL"] != "https://github.com/org/repo" {
			t.Errorf("expected repoURL preserved, got %v", details["repoURL"])
		}
		if _, exists := details["password"]; exists {
			t.Error("expected password removed by RecordEvent")
		}

		nested, ok := details["nested"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested map, got %T", details["nested"])
		}
		if _, exists := nested["token"]; exists {
			t.Error("expected token removed from nested map by RecordEvent")
		}
		if nested["safe"] != "value" {
			t.Errorf("expected safe value preserved, got %v", nested["safe"])
		}
	})
}

func TestRecordEvent_NilRecorderNoOps(t *testing.T) {
	t.Parallel()
	// Should not panic with nil recorder
	RecordEvent(nil, context.Background(), Event{
		Action: "create",
		Details: map[string]any{
			"key": "value",
		},
	})
}

// --- L1: Defense-in-depth integration test ---
// Simulates handlers accidentally including secret fields in audit details.
// RecordEvent's centralized sanitization must strip them before the recorder sees them.

func TestRecordEvent_HandlerSecretLeakDefenseInDepth(t *testing.T) {
	withRedactFields([]string{"privateKey", "password", "bearerToken", "clientSecret"}, func() {
		rec := &testRecorder{}
		ctx := context.Background()

		// Simulate repository handler accidentally leaking SSH private key
		RecordEvent(rec, ctx, Event{
			Action:   "create",
			Resource: "repositories",
			Name:     "my-repo",
			Project:  "alpha",
			Result:   "success",
			Details: map[string]any{
				"repoURL":    "git@github.com:org/repo.git",
				"authType":   "ssh",
				"privateKey": "-----BEGIN RSA PRIVATE KEY-----\nMIIE...",
			},
		})

		// Simulate SSO handler accidentally leaking OAuth client secret
		RecordEvent(rec, ctx, Event{
			Action:   "update",
			Resource: "settings",
			Name:     "my-sso",
			Result:   "success",
			Details: map[string]any{
				"settingsType": "sso_provider",
				"issuerURL":    SafeChanges("old-url", "new-url"),
				"clientSecret": "oauth-secret-value",
			},
		})

		// Simulate instance handler accidentally leaking bearer token
		RecordEvent(rec, ctx, Event{
			Action:   "deploy",
			Resource: "instances",
			Name:     "my-instance",
			Result:   "success",
			Details: map[string]any{
				"rgdName":     "webapp",
				"kind":        "WebApp",
				"bearerToken": "ghp_xxxxxxxxxxxx",
			},
		})

		if len(rec.events) != 3 {
			t.Fatalf("expected 3 events, got %d", len(rec.events))
		}

		// Repository: privateKey must be stripped, safe fields preserved
		repoDetails := rec.events[0].Details
		if _, exists := repoDetails["privateKey"]; exists {
			t.Error("privateKey leaked through RecordEvent in repository create")
		}
		if repoDetails["repoURL"] != "git@github.com:org/repo.git" {
			t.Errorf("expected repoURL preserved, got %v", repoDetails["repoURL"])
		}
		if repoDetails["authType"] != "ssh" {
			t.Errorf("expected authType preserved, got %v", repoDetails["authType"])
		}

		// SSO: clientSecret must be stripped, safe fields preserved
		ssoDetails := rec.events[1].Details
		if _, exists := ssoDetails["clientSecret"]; exists {
			t.Error("clientSecret leaked through RecordEvent in SSO update")
		}
		if ssoDetails["settingsType"] != "sso_provider" {
			t.Errorf("expected settingsType preserved, got %v", ssoDetails["settingsType"])
		}

		// Instance: bearerToken must be stripped, safe fields preserved
		instanceDetails := rec.events[2].Details
		if _, exists := instanceDetails["bearerToken"]; exists {
			t.Error("bearerToken leaked through RecordEvent in instance deploy")
		}
		if instanceDetails["rgdName"] != "webapp" {
			t.Errorf("expected rgdName preserved, got %v", instanceDetails["rgdName"])
		}
	})
}
