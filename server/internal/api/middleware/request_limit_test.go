// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestSizeLimit(t *testing.T) {
	// Create a test handler that echoes the request body
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	// Wrap with RequestSizeLimit middleware
	handler := RequestSizeLimit(testHandler)

	tests := []struct {
		name           string
		bodySize       int
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "request within limit (1KB)",
			bodySize:       1024,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request at limit (1MB)",
			bodySize:       MaxRequestBodySize,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request just under limit",
			bodySize:       MaxRequestBodySize - 1,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "request exceeds limit (2MB)",
			bodySize:       MaxRequestBodySize + 1,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "request far exceeds limit (10MB)",
			bodySize:       10 * MaxRequestBodySize,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body with specified size
			body := bytes.NewReader(bytes.Repeat([]byte("a"), tt.bodySize))
			req := httptest.NewRequest(http.MethodPost, "/test", body)
			rec := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rec, req)

			// Check status code
			if rec.Code != tt.expectedStatus {
				t.Errorf("status code = %d, want %d", rec.Code, tt.expectedStatus)
			}

			// For successful requests, verify body was read
			if !tt.expectError && rec.Code == http.StatusOK {
				responseBody := rec.Body.Bytes()
				if len(responseBody) != tt.bodySize {
					t.Errorf("response body size = %d, want %d", len(responseBody), tt.bodySize)
				}
			}

			// For oversized requests, verify error message
			if tt.expectError {
				responseBody := rec.Body.String()
				if !strings.Contains(responseBody, "request body too large") &&
					!strings.Contains(responseBody, "http: request body too large") {
					t.Errorf("expected error message about request size, got: %s", responseBody)
				}
			}
		})
	}
}

func TestRequestSizeLimitEmptyBody(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := RequestSizeLimit(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("response body = %q, want %q", rec.Body.String(), "OK")
	}
}
