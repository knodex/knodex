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
