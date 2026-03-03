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

func TestSPAHandler_ServesGitkeep(t *testing.T) {
	t.Parallel()

	handler := SPAHandler()

	// .gitkeep exists as a real file in the dist directory
	req := httptest.NewRequest("GET", "/.gitkeep", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
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
