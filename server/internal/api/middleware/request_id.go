package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// fallbackCounter is used when crypto/rand fails
var fallbackCounter atomic.Uint64

// RequestID middleware adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}

func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp + counter if crypto/rand fails (extremely rare)
		slog.Warn("crypto/rand.Read failed, using fallback", "error", err)
		counter := fallbackCounter.Add(1)
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), counter)
	}
	return hex.EncodeToString(b)
}
