// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/knodex/knodex/server/internal/api/response"
)

// PanicsTotal counts the number of panics recovered by the middleware.
var PanicsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "knodex_http_panics_total",
		Help: "Total number of panics recovered by the HTTP recovery middleware",
	},
	[]string{"method", "path"},
)

// responseTracker wraps http.ResponseWriter to track whether headers have
// been sent, so the recovery handler can avoid a superfluous WriteHeader call
// when a handler panics after partially writing a response.
type responseTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rt *responseTracker) WriteHeader(code int) {
	rt.wroteHeader = true
	rt.ResponseWriter.WriteHeader(code)
}

func (rt *responseTracker) Write(b []byte) (int, error) {
	rt.wroteHeader = true
	return rt.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (rt *responseTracker) Unwrap() http.ResponseWriter {
	return rt.ResponseWriter
}

// Recovery middleware catches panics from downstream handlers, logs the stack
// trace via structured slog, returns a JSON 500 response, and increments the
// knodex_http_panics_total Prometheus counter.
//
// It must be the outermost middleware so that panics in any layer are caught.
// On the success path no recover() is called — the deferred function checks the
// return value of recover() which is nil when no panic occurred, so the overhead
// is a single deferred call.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &responseTracker{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				// http.ErrAbortHandler is a sentinel used by net/http to
				// abort the connection. Re-panic so the server tears down
				// the connection as intended.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				stack := debug.Stack()

				// Build structured log attributes
				attrs := []any{
					"error", fmt.Sprint(rec),
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(stack),
				}

				if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
					attrs = append(attrs, "request_id", reqID)
				}

				if userCtx, ok := GetUserContext(r); ok {
					attrs = append(attrs, "user_id", userCtx.UserID, "user_email", userCtx.Email)
				}

				slog.Error("panic recovered in HTTP handler", attrs...)

				// Use the matched route pattern for the label to avoid
				// unbounded cardinality from dynamic path segments.
				routePattern := r.Pattern
				if routePattern == "" {
					routePattern = "unmatched"
				}

				PanicsTotal.With(prometheus.Labels{
					"method": r.Method,
					"path":   routePattern,
				}).Inc()

				// Only write the error response if headers haven't been
				// sent yet. If the handler already wrote a partial response,
				// the connection is compromised and we can only log.
				if !tw.wroteHeader {
					response.InternalError(w, "internal server error")
				}
			}
		}()

		next.ServeHTTP(tw, r)
	})
}
