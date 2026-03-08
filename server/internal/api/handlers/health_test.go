package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/health"
)

func TestHealthHandler_Healthz(t *testing.T) {
	t.Parallel()
	// Create checker without clients (always healthy for liveness)
	checker := health.NewChecker(nil, nil, nil)
	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.Healthz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var status health.HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status.Status != health.StatusHealthy {
		t.Errorf("expected status %s, got %s", health.StatusHealthy, status.Status)
	}
}

func TestHealthHandler_Readyz_NoClients(t *testing.T) {
	t.Parallel()
	// Create checker without clients
	checker := health.NewChecker(nil, nil, nil)
	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.Readyz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var status health.HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status.Status != health.StatusHealthy {
		t.Errorf("expected status %s, got %s", health.StatusHealthy, status.Status)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	t.Parallel()
	checker := health.NewChecker(nil, nil, nil)
	handler := NewHealthHandler(checker)

	tests := []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"healthz", "/healthz", handler.Healthz},
		{"readyz", "/readyz", handler.Readyz},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected content-type application/json, got %s", contentType)
			}
		})
	}
}
