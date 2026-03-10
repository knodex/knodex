// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_SwaggerUIPage(t *testing.T) {
	t.Parallel()

	handler := Handler("/swagger/")

	req := httptest.NewRequest("GET", "/swagger/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /swagger/ returned status %d, want 200", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "swagger-ui") {
		t.Error("response body should contain 'swagger-ui'")
	}
	if !strings.Contains(body, "Knodex API Documentation") {
		t.Error("response body should contain page title")
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", contentType)
	}

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header should be set for Swagger UI page")
	}
	if !strings.Contains(csp, "cdn.jsdelivr.net") {
		t.Error("CSP should allow cdn.jsdelivr.net for Swagger UI assets")
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("CSP should include frame-ancestors 'none'")
	}
}

func TestHandler_SwaggerUIPageIndexHTML(t *testing.T) {
	t.Parallel()

	handler := Handler("/swagger/")

	req := httptest.NewRequest("GET", "/swagger/index.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /swagger/index.html returned status %d, want 200", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "swagger-ui") {
		t.Error("index.html should serve Swagger UI page")
	}
}

func TestHandler_OpenAPIYAML(t *testing.T) {
	t.Parallel()

	handler := Handler("/swagger/")

	req := httptest.NewRequest("GET", "/swagger/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /swagger/openapi.yaml returned status %d, want 200", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/yaml; charset=utf-8", contentType)
	}

	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=300" {
		t.Errorf("Cache-Control = %q, want 'public, max-age=300'", cacheControl)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "openapi:") {
		t.Error("response body should contain valid OpenAPI YAML content")
	}
}

func TestHandler_UnknownPath_Returns404(t *testing.T) {
	t.Parallel()

	handler := Handler("/swagger/")

	req := httptest.NewRequest("GET", "/swagger/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /swagger/nonexistent returned status %d, want 404", rr.Code)
	}
}

func TestHandler_CSPNotSetOnYAML(t *testing.T) {
	t.Parallel()

	handler := Handler("/swagger/")

	req := httptest.NewRequest("GET", "/swagger/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp != "" {
		t.Errorf("CSP should not be set on YAML endpoint, got %q", csp)
	}
}
