// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com", "http://localhost:3000"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/rgds", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://app.example.com', got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods header, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Request-ID" {
		t.Errorf("expected Access-Control-Allow-Headers header, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("expected Access-Control-Max-Age '3600', got %q", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestCORS_BlockedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/rgds", nil)
	req.Header.Set("Origin", "https://attacker.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header for blocked origin, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Methods header for blocked origin, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Headers header for blocked origin, got %q", got)
	}
	// Vary: Origin must always be set when Origin header is present (cache poisoning prevention)
	if rr.Header().Get("Vary") == "" {
		t.Error("expected Vary header to include Origin for blocked origin (cache safety)")
	}
	// Request still succeeds (browser enforces CORS, not server)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/rgds", nil)
	// No Origin header
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS headers for same-origin request, got %q", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestCORS_PreflightAllowed(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/rgds", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("expected CORS headers on preflight, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods on preflight, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Request-ID" {
		t.Errorf("expected Access-Control-Allow-Headers on preflight, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("expected Access-Control-Max-Age on preflight, got %q", got)
	}
}

func TestCORS_PreflightBlocked(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/rgds", nil)
	req.Header.Set("Origin", "https://attacker.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for blocked preflight, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS headers for blocked preflight, got %q", got)
	}
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: nil,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/rgds", nil)
	req.Header.Set("Origin", "https://any-origin.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS headers when no origins configured, got %q", got)
	}
}
