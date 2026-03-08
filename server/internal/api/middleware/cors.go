// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

// CORSConfig holds configuration for CORS middleware
type CORSConfig struct {
	// AllowedOrigins is the list of origins that are allowed to make cross-origin requests.
	// If empty, no CORS headers are returned (deny all cross-origin requests).
	AllowedOrigins []string
}

// CORS creates a CORS middleware that only allows requests from configured origins.
// If no origins are configured, no Access-Control-Allow-Origin header is set,
// effectively denying all cross-origin requests.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	// Build a lookup set for O(1) origin checks
	allowedSet := make(map[string]bool, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		allowedSet[strings.TrimSpace(origin)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// No Origin header means same-origin or non-browser request — skip CORS
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Always set Vary: Origin when Origin header is present to prevent
			// cache poisoning — ensures proxies/CDNs don't serve a cached
			// CORS-header-free response to an allowed origin.
			w.Header().Add("Vary", "Origin")

			// Check if origin is in the allowlist
			if allowedSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "3600")
			} else {
				slog.Warn("blocked cross-origin request",
					"origin", origin,
					"path", r.URL.Path,
				)
				// No CORS headers set — browser will block the response
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				if allowedSet[origin] {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusForbidden)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
