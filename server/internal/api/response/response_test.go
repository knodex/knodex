// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMethodNotAllowed(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	MethodNotAllowed(w, "method not allowed")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeMethodNotAllowed {
		t.Errorf("expected code %s, got %s", ErrCodeMethodNotAllowed, resp.Code)
	}

	if resp.Message != "method not allowed" {
		t.Errorf("expected message 'method not allowed', got '%s'", resp.Message)
	}
}

func TestUnauthorized(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	Unauthorized(w, "missing Authorization header")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("expected code %s, got %s", ErrCodeUnauthorized, resp.Code)
	}
}

func TestForbidden(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	Forbidden(w, "permission denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeForbidden {
		t.Errorf("expected code %s, got %s", ErrCodeForbidden, resp.Code)
	}

	if resp.Message != "permission denied" {
		t.Errorf("expected message 'permission denied', got '%s'", resp.Message)
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	NotFound(w, "dashboard", "test-slug")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeNotFound, resp.Code)
	}

	if resp.Details["resource"] != "dashboard" {
		t.Errorf("expected resource 'dashboard', got '%s'", resp.Details["resource"])
	}
}

func TestBadRequest(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	BadRequest(w, "invalid input", map[string]string{"field": "name"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeBadRequest {
		t.Errorf("expected code %s, got %s", ErrCodeBadRequest, resp.Code)
	}

	if resp.Details["field"] != "name" {
		t.Errorf("expected details field 'name', got '%s'", resp.Details["field"])
	}
}

func TestServiceUnavailable(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	ServiceUnavailable(w, "service down")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeServiceUnavailable {
		t.Errorf("expected code %s, got %s", ErrCodeServiceUnavailable, resp.Code)
	}
}

func TestInternalError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	InternalError(w, "something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeInternalError {
		t.Errorf("expected code %s, got %s", ErrCodeInternalError, resp.Code)
	}
}

// assertNoCacheHeaders verifies all three no-cache headers are set correctly.
func assertNoCacheHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store, no-cache, must-revalidate")
	}
	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want %q", got, "no-cache")
	}
	if got := w.Header().Get("Expires"); got != "0" {
		t.Errorf("Expires = %q, want %q", got, "0")
	}
}

// assertNoNoCacheHeaders verifies no-cache headers are NOT set.
func assertNoNoCacheHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control should not be set, got %q", got)
	}
	if got := w.Header().Get("Pragma"); got != "" {
		t.Errorf("Pragma should not be set, got %q", got)
	}
	if got := w.Header().Get("Expires"); got != "" {
		t.Errorf("Expires should not be set, got %q", got)
	}
}

func TestWriteAuthJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteAuthJSON(w, http.StatusOK, map[string]string{"token": "test"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	assertNoCacheHeaders(t, w)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["token"] != "test" {
		t.Errorf("expected token 'test', got %q", resp["token"])
	}
}

func TestWriteAuthError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteAuthError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "invalid creds", nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	assertNoCacheHeaders(t, w)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("expected code %s, got %s", ErrCodeUnauthorized, resp.Code)
	}
}

func TestAuthBadRequest(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	AuthBadRequest(w, "bad input", map[string]string{"field": "code"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	assertNoCacheHeaders(t, w)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != ErrCodeBadRequest {
		t.Errorf("expected code %s, got %s", ErrCodeBadRequest, resp.Code)
	}
}

func TestAuthUnauthorized(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	AuthUnauthorized(w, "invalid credentials")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	assertNoCacheHeaders(t, w)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("expected code %s, got %s", ErrCodeUnauthorized, resp.Code)
	}
}

func TestAuthInternalError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	AuthInternalError(w, "something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	assertNoCacheHeaders(t, w)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != ErrCodeInternalError {
		t.Errorf("expected code %s, got %s", ErrCodeInternalError, resp.Code)
	}
}

func TestWriteJSON_DoesNotSetNoCacheHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteJSON(w, http.StatusOK, map[string]string{"data": "value"})

	assertNoNoCacheHeaders(t, w)
}

func TestWriteError_DoesNotSetNoCacheHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusNotFound, ErrCodeNotFound, "not found", nil)

	assertNoNoCacheHeaders(t, w)
}

func TestWriteError_HTMLEncodesMessage(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "<script>alert(1)</script>", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	expected := "&lt;script&gt;alert(1)&lt;/script&gt;"
	if resp.Message != expected {
		t.Errorf("expected message %q, got %q", expected, resp.Message)
	}
}

func TestWriteError_HTMLEncodesDetails(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "validation failed",
		map[string]string{"field": `<img src=x onerror=alert(1)>`})

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	expected := "&lt;img src=x onerror=alert(1)&gt;"
	if resp.Details["field"] != expected {
		t.Errorf("expected details field %q, got %q", expected, resp.Details["field"])
	}
}

func TestWriteError_PlainTextUnchanged(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusNotFound, ErrCodeNotFound, "instance not found", nil)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Message != "instance not found" {
		t.Errorf("expected message %q, got %q", "instance not found", resp.Message)
	}
}

func TestWriteError_PreEncodedEntitiesAreDoubleEncoded(t *testing.T) {
	t.Parallel()

	// html.EscapeString intentionally double-encodes pre-escaped entities.
	// This is the correct behavior: callers should pass raw text, not pre-escaped HTML.
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "query error: a &amp; b", nil)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The & in &amp; is escaped again, producing &amp;amp;
	expected := "query error: a &amp;amp; b"
	if resp.Message != expected {
		t.Errorf("expected message %q, got %q", expected, resp.Message)
	}
}

func TestWriteError_AmpersandAndQuotes(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, `a & b > c < d "e" 'f'`, nil)

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	expected := "a &amp; b &gt; c &lt; d &#34;e&#34; &#39;f&#39;"
	if resp.Message != expected {
		t.Errorf("expected message %q, got %q", expected, resp.Message)
	}
}

func TestWriteError_RateLimit(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	WriteError(w, http.StatusTooManyRequests, ErrCodeRateLimit, "too many connections", nil)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != ErrCodeRateLimit {
		t.Errorf("expected code %s, got %s", ErrCodeRateLimit, resp.Code)
	}
}
