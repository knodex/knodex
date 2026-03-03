package services

import (
	"context"
	"io"
	"time"
)

// ViolationHistoryService defines the interface for querying violation history.
// This is a separate interface from ComplianceService to avoid cascading changes.
// In OSS builds, a noop implementation returns "unavailable" errors.
type ViolationHistoryService interface {
	// IsAvailable returns true if the history store is ready to serve requests.
	IsAvailable() bool

	// GetRetentionDays returns the configured retention period in days.
	GetRetentionDays() int

	// ListByTimeRange returns paginated violation history records within a time range.
	ListByTimeRange(ctx context.Context, since, until time.Time, opts ViolationHistoryListOptions) ([]ViolationHistoryRecord, int, error)

	// CountByTimeRange returns the count of violation records matching filters.
	CountByTimeRange(ctx context.Context, since, until time.Time, filters ViolationHistoryExportFilters) (int, error)

	// ExportCSV streams violation history records as CSV to the provided writer.
	ExportCSV(ctx context.Context, since time.Time, filters ViolationHistoryExportFilters, w io.Writer) error
}

// ViolationHistoryRecord represents a violation history entry for API responses.
type ViolationHistoryRecord struct {
	Key               string     `json:"key"`
	ConstraintKind    string     `json:"constraintKind"`
	ConstraintName    string     `json:"constraintName"`
	ResourceKind      string     `json:"resourceKind"`
	ResourceNamespace string     `json:"resourceNamespace"`
	ResourceName      string     `json:"resourceName"`
	EnforcementAction string     `json:"enforcementAction"`
	Message           string     `json:"message"`
	FirstSeen         time.Time  `json:"firstSeen"`
	ResolvedAt        *time.Time `json:"resolvedAt"`
	Status            string     `json:"status"` // "active" or "resolved"
}

// ViolationHistoryListOptions defines pagination and filtering for history queries.
type ViolationHistoryListOptions struct {
	Page        int
	PageSize    int
	Constraint  string // filter: {kind}/{name}
	Resource    string // filter: {kind}/{namespace}/{name}
	Enforcement string // filter: deny, warn, dryrun
	Status      string // filter: active, resolved, all
}

// ViolationHistoryExportFilters defines filter criteria for CSV export and count queries.
type ViolationHistoryExportFilters struct {
	Enforcement string // filter: deny, warn, dryrun
	Constraint  string // filter: {kind}/{name}
	Resource    string // filter: {kind}/{namespace}/{name}
}

// NoopViolationHistoryService is a no-op implementation for OSS builds.
type NoopViolationHistoryService struct{}

// IsAvailable returns false.
func (s *NoopViolationHistoryService) IsAvailable() bool { return false }

// GetRetentionDays returns 0.
func (s *NoopViolationHistoryService) GetRetentionDays() int { return 0 }

// ListByTimeRange returns an error.
func (s *NoopViolationHistoryService) ListByTimeRange(_ context.Context, _, _ time.Time, _ ViolationHistoryListOptions) ([]ViolationHistoryRecord, int, error) {
	return nil, 0, ErrServiceUnavailable
}

// CountByTimeRange returns an error.
func (s *NoopViolationHistoryService) CountByTimeRange(_ context.Context, _, _ time.Time, _ ViolationHistoryExportFilters) (int, error) {
	return 0, ErrServiceUnavailable
}

// ExportCSV returns an error.
func (s *NoopViolationHistoryService) ExportCSV(_ context.Context, _ time.Time, _ ViolationHistoryExportFilters, _ io.Writer) error {
	return ErrServiceUnavailable
}
