// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"HSTS", "Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload"},
		{"CSP", "Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'none'"},
		{"X-Frame-Options", "X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "Referrer-Policy", "strict-origin-when-cross-origin"},
		{"X-Permitted-Cross-Domain-Policies", "X-Permitted-Cross-Domain-Policies", "none"},
		{"Permissions-Policy", "Permissions-Policy", "camera=(), microphone=(), geolocation=(), usb=(), payment=()"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get(tt.header); got != tt.want {
				t.Errorf("%s header = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
