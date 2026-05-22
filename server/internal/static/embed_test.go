// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package static

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler_ServesIndexHTML(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected Cache-Control no-cache for index.html, got %q", cacheControl)
	}
}

func TestSPAHandler_FallsBackToIndexHTML(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	// Non-existent path should fall back to index.html (SPA routing)
	req := httptest.NewRequest("GET", "/dashboard/settings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for SPA fallback, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html for SPA fallback, got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<div id=\"root\">") {
		t.Error("expected SPA fallback to serve index.html content")
	}
}

func TestSPAHandler_MissingAssetReturns404(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	// Requests for missing JS/CSS/etc. assets must NOT fall back to index.html —
	// the browser would receive HTML where it expects JS, triggering
	// "'text/html' is not a valid JavaScript MIME type" errors.
	cases := []string{
		"/assets/nonexistent-abc123.js",
		"/assets/nonexistent-abc123.css",
		"/assets/nonexistent.map",
		"/missing.svg",
	}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected status 404 for missing asset %q, got %d", path, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if strings.Contains(contentType, "text/html") && strings.Contains(w.Body.String(), "<div id=\"root\">") {
				t.Errorf("expected NOT to serve SPA index.html for missing asset %q (Content-Type=%q)", path, contentType)
			}
		})
	}
}

func TestSPAHandler_ServesRealFile(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	req := httptest.NewRequest("GET", "/favicon.svg", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "image/svg") {
		t.Errorf("expected SVG content type, got %q", contentType)
	}
}

func TestSPAHandler_CacheHeaders_HashedAssets(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	setCacheHeaders(w, "assets/main-abc123.js")

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=31536000, immutable" {
		t.Errorf("expected immutable cache for hashed assets, got %q", cacheControl)
	}
}

func TestSPAHandler_CacheHeaders_IndexHTML(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	setCacheHeaders(w, "index.html")

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected no-cache for index.html, got %q", cacheControl)
	}
}

func TestSPAHandler_CacheHeaders_OtherFiles(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	setCacheHeaders(w, "favicon.ico")

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=3600" {
		t.Errorf("expected public max-age=3600 for other files, got %q", cacheControl)
	}
}

func TestSPAHandler_PathTraversalFallsBackToIndexHTML(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	// Manually craft a request with ".." in the path
	// (normally cleaned by net/http, but defense-in-depth test)
	req := httptest.NewRequest("GET", "/foo", nil)
	req.URL.Path = "/../../../etc/passwd"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for path traversal fallback, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html for path traversal fallback, got %q", contentType)
	}
}

func TestServeIndexHTML_MissingIndexReturnsError(t *testing.T) {
	t.Parallel()

	// Create an empty FS with no index.html
	emptyFS := fstest.MapFS{}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/some-route", nil)

	serveIndexHTML(w, req, fs.FS(emptyFS))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 when index.html missing, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Errorf("expected INTERNAL_ERROR code in response, got %q", body)
	}
	if !strings.Contains(body, "embedded index.html not found") {
		t.Errorf("expected error message about missing index.html, got %q", body)
	}
}
