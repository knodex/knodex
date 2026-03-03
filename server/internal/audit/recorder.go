// Package audit provides a shared audit recording interface for handler instrumentation.
// This package is NOT gated behind enterprise build tags, so both OSS and EE builds
// can reference the Recorder interface and Event type. In OSS builds, the recorder
// is nil; in EE builds, it is backed by the AuditService via RecorderBridge.
package audit

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// Event represents an auditable operation recorded by handlers after
// a successful business operation. Fields map to the enterprise AuditEvent.
type Event struct {
	// Who
	UserID    string
	UserEmail string
	SourceIP  string

	// What
	Action   string // e.g. "create", "delete", "member_add", "enforcement_change"
	Resource string // e.g. "projects", "instances", "repositories", "compliance", "settings"
	Name     string // resource identifier (project name, instance name, etc.)

	// Where
	Project   string // project scope (empty for global actions)
	Namespace string // K8s namespace (empty for non-namespaced resources)

	// Context
	RequestID string

	// Result
	Result string // "success", "denied", "error"

	// Details contains action-specific metadata.
	Details map[string]any
}

// Recorder is the interface handlers use to record audit events.
// In OSS builds, the recorder is nil. In EE builds, it wraps AuditService.
// Implementations must be safe for concurrent use.
type Recorder interface {
	Record(ctx context.Context, event Event)
}

// RecordEvent safely records an audit event if the recorder is not nil.
// This is the primary entry point for handler instrumentation.
func RecordEvent(r Recorder, ctx context.Context, event Event) {
	if r != nil {
		r.Record(ctx, event)
	}
}

// SourceIP extracts the client's IP address from the request.
// Checks X-Forwarded-For (first entry), then X-Real-IP, then falls back to
// RemoteAddr (with port stripped).
//
// IMPORTANT: This function trusts proxy headers. Deploy behind a trusted
// reverse proxy that overwrites X-Forwarded-For and X-Real-IP.
func SourceIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
