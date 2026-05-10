// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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

	utilenv "github.com/knodex/knodex/server/internal/util/env"
)

// secretFieldNames stores the lowercase versions of all field names that must
// never appear in audit Details. Lookups are case-insensitive.
// Populated entirely from the AUDIT_REDACT_FIELDS environment variable,
// which is set via the Helm chart with secure defaults.
var secretFieldNames map[string]bool

func init() {
	secretFieldNames = make(map[string]bool, 16)
	if fields := utilenv.GetString("AUDIT_REDACT_FIELDS", ""); fields != "" {
		for _, f := range strings.Split(fields, ",") {
			if f = strings.TrimSpace(f); f != "" {
				secretFieldNames[strings.ToLower(f)] = true
			}
		}
	}
}

// IsSecretField reports whether the given field name is in the redaction set.
// The check is case-insensitive.
func IsSecretField(name string) bool {
	return secretFieldNames[strings.ToLower(name)]
}

// SecretFieldNames returns a copy of the current redaction set (lowercase keys).
// Useful for testing and diagnostics.
func SecretFieldNames() map[string]bool {
	out := make(map[string]bool, len(secretFieldNames))
	for k, v := range secretFieldNames {
		out[k] = v
	}
	return out
}

// SanitizeDetails returns a copy of details with all keys matching the
// redaction set removed (case-insensitive), including in nested maps and slices.
// Handlers should never add secret fields, but this function provides a
// defense-in-depth safety net. Returns nil if the input is nil.
func SanitizeDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	sanitized := make(map[string]any, len(details))
	for k, v := range details {
		if IsSecretField(k) {
			continue
		}
		sanitized[k] = sanitizeValue(v)
	}
	return sanitized
}

// sanitizeValue recursively sanitizes maps and slices.
func sanitizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return SanitizeDetails(val)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = sanitizeValue(elem)
		}
		return out
	default:
		return v
	}
}

// SafeChanges returns a map representing a before/after change for a field.
// The returned map has "old" and "new" keys.
// Example: SafeChanges("git@old", "git@new") → map[string]any{"old": "git@old", "new": "git@new"}
func SafeChanges(oldVal, newVal any) map[string]any {
	return map[string]any{
		"old": oldVal,
		"new": newVal,
	}
}

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

	// Organization is the org_id scope for the event. Used by EE Postgres store
	// to enforce per-org RLS isolation. When empty, the RecorderBridge fills
	// it from the configured defaultOrg before forwarding to AuditService.
	Organization string

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
// Details are automatically sanitized (defense-in-depth) so handlers
// do not need to call SanitizeDetails themselves.
func RecordEvent(r Recorder, ctx context.Context, event Event) {
	if r != nil {
		event.Details = SanitizeDetails(event.Details)
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
