// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"log/slog"
	"time"
)

// AuditEvent types for RBAC operations
const (
	AuditEventProjectCreated   = "project.created"
	AuditEventProjectUpdated   = "project.updated"
	AuditEventProjectDeleted   = "project.deleted"
	AuditEventMemberAdded      = "project.member.added"
	AuditEventMemberRemoved    = "project.member.removed"
	AuditEventMemberRoleUpdate = "project.member.role_updated"
)

// AuditLogger provides structured audit logging for security operations
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
	}
}

// LogEvent logs a security audit event
func (a *AuditLogger) LogEvent(ctx context.Context, event string, attrs ...slog.Attr) {
	// Build attribute list with standard fields
	logAttrs := []slog.Attr{
		slog.String("event_type", event),
		slog.Time("timestamp", time.Now()),
	}
	logAttrs = append(logAttrs, attrs...)

	// Log at INFO level for successful operations
	a.logger.LogAttrs(ctx, slog.LevelInfo, "Security audit event", logAttrs...)
}

// LogProjectCreated logs project creation
func (a *AuditLogger) LogProjectCreated(ctx context.Context, projectID, displayName, namespace, createdBy string, memberCount int) {
	a.LogEvent(ctx, AuditEventProjectCreated,
		slog.String("project_id", projectID),
		slog.String("display_name", displayName),
		slog.String("namespace", namespace),
		slog.String("created_by", createdBy),
		slog.Int("initial_member_count", memberCount),
	)
}

// LogProjectUpdated logs project updates
func (a *AuditLogger) LogProjectUpdated(ctx context.Context, projectID, updatedBy string, changes map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("project_id", projectID),
		slog.String("updated_by", updatedBy),
	}

	// Add change details
	for key, value := range changes {
		attrs = append(attrs, slog.Any(key, value))
	}

	a.LogEvent(ctx, AuditEventProjectUpdated, attrs...)
}

// LogProjectDeleted logs project deletion
func (a *AuditLogger) LogProjectDeleted(ctx context.Context, projectID, deletedBy string) {
	a.LogEvent(ctx, AuditEventProjectDeleted,
		slog.String("project_id", projectID),
		slog.String("deleted_by", deletedBy),
	)
}

// LogMemberAdded logs member addition to project
func (a *AuditLogger) LogMemberAdded(ctx context.Context, projectID, userID, role, addedBy string) {
	a.LogEvent(ctx, AuditEventMemberAdded,
		slog.String("project_id", projectID),
		slog.String("user_id", userID),
		slog.String("role", role),
		slog.String("added_by", addedBy),
	)
}

// LogMemberRemoved logs member removal from project
func (a *AuditLogger) LogMemberRemoved(ctx context.Context, projectID, userID, removedBy string) {
	a.LogEvent(ctx, AuditEventMemberRemoved,
		slog.String("project_id", projectID),
		slog.String("user_id", userID),
		slog.String("removed_by", removedBy),
	)
}

// LogMemberRoleUpdated logs member role changes
func (a *AuditLogger) LogMemberRoleUpdated(ctx context.Context, projectID, userID, oldRole, newRole, updatedBy string) {
	a.LogEvent(ctx, AuditEventMemberRoleUpdate,
		slog.String("project_id", projectID),
		slog.String("user_id", userID),
		slog.String("old_role", oldRole),
		slog.String("new_role", newRole),
		slog.String("updated_by", updatedBy),
	)
}

// LogSecurityViolation logs security violations or suspicious activities
func (a *AuditLogger) LogSecurityViolation(ctx context.Context, operation, reason, userID string, details map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("event_type", "security.violation"),
		slog.String("operation", operation),
		slog.String("reason", reason),
		slog.String("user_id", userID),
		slog.Time("timestamp", time.Now()),
	}

	for key, value := range details {
		attrs = append(attrs, slog.Any(key, value))
	}

	a.logger.LogAttrs(ctx, slog.LevelWarn, "Security violation detected", attrs...)
}
