package middleware

import (
	"net/http"
)

// MaxRequestBodySize defines the maximum allowed request body size (1MB)
const MaxRequestBodySize = 1 << 20 // 1MB

// RequestSizeLimit limits the size of incoming request bodies to prevent DoS attacks
func RequestSizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the request body with MaxBytesReader to enforce size limit
		// This prevents attackers from sending arbitrarily large payloads
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
		next.ServeHTTP(w, r)
	})
}
