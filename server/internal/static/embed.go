package static

import (
	"embed"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/knodex/knodex/server/internal/api/response"
)

//go:embed all:dist
var staticFS embed.FS

// SPAHandler returns an http.Handler that serves the embedded frontend files
// with SPA (Single Page Application) fallback. Static assets are served directly;
// all other routes fall back to index.html for client-side routing.
//
// Caching strategy:
//   - Hashed assets (/assets/*): Cache-Control: public, max-age=31536000, immutable
//   - index.html: Cache-Control: no-cache (revalidate every time)
//   - Other files: Cache-Control: public, max-age=3600
func SPAHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem for embedded dist: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		// Defense-in-depth: reject paths with ".." components
		// (embed.FS also rejects these, but explicit is better)
		if strings.Contains(path, "..") {
			serveIndexHTML(w, r, sub)
			return
		}

		// Try to open the file from the embedded filesystem
		f, err := sub.Open(path)
		if err != nil {
			// File not found — serve index.html for SPA client-side routing
			serveIndexHTML(w, r, sub)
			return
		}
		f.Close()

		// File exists — set appropriate cache headers
		setCacheHeaders(w, path)

		// Set correct MIME type based on file extension
		ext := filepath.Ext(path)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}

		fileServer.ServeHTTP(w, r)
	})
}

// serveIndexHTML serves the SPA index.html with no-cache headers.
func serveIndexHTML(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		slog.Error("embedded index.html not found", "error", err)
		response.InternalError(w, "embedded index.html not found")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// setCacheHeaders sets Cache-Control headers based on the file path.
// Vite hashed assets get long-lived immutable caching; index.html gets no-cache.
func setCacheHeaders(w http.ResponseWriter, path string) {
	switch {
	case path == "index.html":
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasPrefix(path, "assets/"):
		// Vite hashed assets — immutable, cache for 1 year
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		// Other static files (favicon.ico, robots.txt, etc.)
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
}
