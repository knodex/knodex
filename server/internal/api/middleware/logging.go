package middleware

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements the http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Logging middleware logs HTTP requests with user context
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Build log attributes
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"request_id", r.Header.Get("X-Request-ID"),
		}

		// Add user context if available (from Auth middleware)
		// ArgoCD-aligned: log identity context only (user_id, user_email), not admin status.
		// Authorization results are logged at enforcement points, not in request logging.
		if userCtx, ok := GetUserContext(r); ok {
			attrs = append(attrs,
				"user_id", userCtx.UserID,
				"user_email", userCtx.Email,
			)
			if userCtx.DefaultProject != "" {
				attrs = append(attrs, "default_project", userCtx.DefaultProject)
			}
		}

		slog.Info("http request", attrs...)
	})
}
