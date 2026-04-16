// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRecovery_PanicHandler(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify 500 status code
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Verify JSON content type
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	// Verify structured error response
	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if errResp.Code != response.ErrCodeInternalError {
		t.Errorf("error code = %q, want %q", errResp.Code, response.ErrCodeInternalError)
	}
	if errResp.Message != "internal server error" {
		t.Errorf("message = %q, want %q", errResp.Message, "internal server error")
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestRecovery_PanicWithError(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(fmt.Errorf("custom error"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRecovery_ErrAbortHandlerRepanics(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	defer func() {
		r := recover()
		if r != http.ErrAbortHandler {
			t.Errorf("expected re-panic with http.ErrAbortHandler, got %v", r)
		}
	}()

	handler.ServeHTTP(rec, req)
	t.Error("expected panic, but handler returned normally")
}

func TestRecovery_MetricIncremented(t *testing.T) {
	// Reset the counter for this test
	PanicsTotal.Reset()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// In tests without ServeMux routing, r.Pattern is empty so the label
	// falls back to "unmatched".
	m := &dto.Metric{}
	if err := PanicsTotal.With(prometheus.Labels{"method": "GET", "path": "unmatched"}).Write(m); err != nil {
		t.Fatalf("failed to read metric: %v", err)
	}
	if got := m.GetCounter().GetValue(); got != 1 {
		t.Errorf("panics_total = %v, want 1", got)
	}
}

func TestRecovery_PanicAfterHeadersWritten(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		panic("mid-write crash")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The handler already wrote 200 before panicking, so the recovery
	// middleware must not attempt a second WriteHeader(500).
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (headers already sent)", rec.Code, http.StatusOK)
	}
}

func TestRecovery_PanicNilValue(t *testing.T) {
	t.Parallel()

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// panic(nil) in Go 1.21+ raises a *runtime.PanicNilError which is caught by recover()
	// The middleware should handle this gracefully
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
