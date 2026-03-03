package services

import (
	"context"
	"net/http"
	"time"
)

// AuditEvent is the API representation of an audit event.
type AuditEvent struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	UserID    string         `json:"userId"`
	UserEmail string         `json:"userEmail"`
	SourceIP  string         `json:"sourceIP"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Name      string         `json:"name"`
	Project   string         `json:"project,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	RequestID string         `json:"requestId"`
	Result    string         `json:"result"`
	Details   map[string]any `json:"details,omitempty"`
}

// AuditEventList is the paginated response for listing audit events.
type AuditEventList struct {
	Events   []AuditEvent `json:"events"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

// AuditEventFilter defines query parameters for listing audit events.
type AuditEventFilter struct {
	UserID   string
	Action   string
	Resource string
	Project  string
	Result   string
	From     time.Time
	To       time.Time
	Page     int
	PageSize int
}

// UserActivity represents a user's event count for top-users aggregation.
type UserActivity struct {
	UserID string `json:"userId"`
	Count  int64  `json:"count"`
}

// AuditStats is the API representation of audit statistics.
type AuditStats struct {
	TotalEvents    int64            `json:"totalEvents"`
	EventsToday    int64            `json:"eventsToday"`
	TopUsers       []UserActivity   `json:"topUsers"`
	DeniedAttempts int64            `json:"deniedAttempts"`
	ByActionToday  map[string]int64 `json:"byActionToday"`
	ByResultToday  map[string]int64 `json:"byResultToday"`
}

// AuditConfig is the API representation of audit configuration.
type AuditConfig struct {
	Enabled          bool     `json:"enabled"`
	RetentionDays    int      `json:"retentionDays"`
	MaxStreamLength  int      `json:"maxStreamLength"`
	ExcludeActions   []string `json:"excludeActions"`
	ExcludeResources []string `json:"excludeResources"`
}

// AuditAPIService defines the interface for audit trail API operations.
// In OSS builds, this is nil (routes not registered, 404 returned).
// In EE builds, this provides full audit query and config management.
type AuditAPIService interface {
	// ListEvents returns paginated audit events matching the filter.
	ListEvents(ctx context.Context, filter AuditEventFilter) (*AuditEventList, error)

	// GetEvent returns a single audit event by its ULID.
	GetEvent(ctx context.Context, eventID string) (*AuditEvent, error)

	// GetStats returns aggregate audit statistics.
	GetStats(ctx context.Context) (*AuditStats, error)

	// GetConfig returns the current audit configuration.
	GetConfig() *AuditConfig

	// UpdateConfig updates the audit configuration by writing to the ConfigMap.
	UpdateConfig(ctx context.Context, config AuditConfig) error

	// RegisterRoutes registers audit API routes on the given mux.
	RegisterRoutes(mux *http.ServeMux)
}
