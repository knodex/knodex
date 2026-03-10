// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestNewAuditLogger tests audit logger creation
func TestNewAuditLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	auditLogger := NewAuditLogger(logger)

	if auditLogger == nil {
		t.Fatal("Expected non-nil audit logger")
	}
	if auditLogger.logger == nil {
		t.Fatal("Expected logger to be set")
	}
}

// TestLogEvent tests the base log event function
func TestLogEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogEvent(ctx, "test.event",
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	)

	output := buf.String()
	if !strings.Contains(output, "test.event") {
		t.Errorf("Expected event_type 'test.event' in log output, got: %s", output)
	}
	if !strings.Contains(output, "key1") {
		t.Errorf("Expected 'key1' in log output, got: %s", output)
	}
	if !strings.Contains(output, "value1") {
		t.Errorf("Expected 'value1' in log output, got: %s", output)
	}
}

// TestLogProjectCreated tests project creation audit logging
func TestLogProjectCreated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogProjectCreated(ctx, "proj-123", "My Project", "my-namespace", "admin-user", 5)

	output := buf.String()

	// Check event type
	if !strings.Contains(output, AuditEventProjectCreated) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventProjectCreated, output)
	}

	// Check project details
	if !strings.Contains(output, "proj-123") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "My Project") {
		t.Errorf("Expected display_name in output, got: %s", output)
	}
	if !strings.Contains(output, "my-namespace") {
		t.Errorf("Expected namespace in output, got: %s", output)
	}
	if !strings.Contains(output, "admin-user") {
		t.Errorf("Expected created_by in output, got: %s", output)
	}
}

// TestLogProjectUpdated tests project update audit logging
func TestLogProjectUpdated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	changes := map[string]interface{}{
		"description":  "New description",
		"member_count": 10,
	}
	auditLogger.LogProjectUpdated(ctx, "proj-123", "editor-user", changes)

	output := buf.String()

	if !strings.Contains(output, AuditEventProjectUpdated) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventProjectUpdated, output)
	}
	if !strings.Contains(output, "proj-123") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "editor-user") {
		t.Errorf("Expected updated_by in output, got: %s", output)
	}
}

// TestLogProjectUpdatedEmptyChanges tests project update with empty changes map
func TestLogProjectUpdatedEmptyChanges(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogProjectUpdated(ctx, "proj-123", "editor-user", map[string]interface{}{})

	output := buf.String()

	if !strings.Contains(output, AuditEventProjectUpdated) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventProjectUpdated, output)
	}
}

// TestLogProjectDeleted tests project deletion audit logging
func TestLogProjectDeleted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogProjectDeleted(ctx, "proj-to-delete", "admin-deleter")

	output := buf.String()

	if !strings.Contains(output, AuditEventProjectDeleted) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventProjectDeleted, output)
	}
	if !strings.Contains(output, "proj-to-delete") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "admin-deleter") {
		t.Errorf("Expected deleted_by in output, got: %s", output)
	}
}

// TestLogMemberAdded tests member addition audit logging
func TestLogMemberAdded(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogMemberAdded(ctx, "proj-123", "new-user", "developer", "admin-user")

	output := buf.String()

	if !strings.Contains(output, AuditEventMemberAdded) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventMemberAdded, output)
	}
	if !strings.Contains(output, "proj-123") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "new-user") {
		t.Errorf("Expected user_id in output, got: %s", output)
	}
	if !strings.Contains(output, "developer") {
		t.Errorf("Expected role in output, got: %s", output)
	}
	if !strings.Contains(output, "admin-user") {
		t.Errorf("Expected added_by in output, got: %s", output)
	}
}

// TestLogMemberRemoved tests member removal audit logging
func TestLogMemberRemoved(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogMemberRemoved(ctx, "proj-123", "removed-user", "admin-remover")

	output := buf.String()

	if !strings.Contains(output, AuditEventMemberRemoved) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventMemberRemoved, output)
	}
	if !strings.Contains(output, "proj-123") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "removed-user") {
		t.Errorf("Expected user_id in output, got: %s", output)
	}
	if !strings.Contains(output, "admin-remover") {
		t.Errorf("Expected removed_by in output, got: %s", output)
	}
}

// TestLogMemberRoleUpdated tests member role update audit logging
func TestLogMemberRoleUpdated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogMemberRoleUpdated(ctx, "proj-123", "promoted-user", "viewer", "developer", "admin-promoter")

	output := buf.String()

	if !strings.Contains(output, AuditEventMemberRoleUpdate) {
		t.Errorf("Expected event type '%s' in output, got: %s", AuditEventMemberRoleUpdate, output)
	}
	if !strings.Contains(output, "proj-123") {
		t.Errorf("Expected project_id in output, got: %s", output)
	}
	if !strings.Contains(output, "promoted-user") {
		t.Errorf("Expected user_id in output, got: %s", output)
	}
	if !strings.Contains(output, "viewer") {
		t.Errorf("Expected old_role in output, got: %s", output)
	}
	if !strings.Contains(output, "developer") {
		t.Errorf("Expected new_role in output, got: %s", output)
	}
	if !strings.Contains(output, "admin-promoter") {
		t.Errorf("Expected updated_by in output, got: %s", output)
	}
}

// TestLogSecurityViolation tests security violation audit logging
func TestLogSecurityViolation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	details := map[string]interface{}{
		"resource":   "project/my-proj",
		"attempt":    "unauthorized_access",
		"ip_address": "192.168.1.100",
	}
	auditLogger.LogSecurityViolation(ctx, "project.delete", "insufficient_permissions", "malicious-user", details)

	output := buf.String()

	if !strings.Contains(output, "security.violation") {
		t.Errorf("Expected 'security.violation' in output, got: %s", output)
	}
	if !strings.Contains(output, "project.delete") {
		t.Errorf("Expected operation in output, got: %s", output)
	}
	if !strings.Contains(output, "insufficient_permissions") {
		t.Errorf("Expected reason in output, got: %s", output)
	}
	if !strings.Contains(output, "malicious-user") {
		t.Errorf("Expected user_id in output, got: %s", output)
	}
}

// TestLogSecurityViolationEmptyDetails tests security violation with empty details
func TestLogSecurityViolationEmptyDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	auditLogger.LogSecurityViolation(ctx, "project.delete", "unknown", "user-123", map[string]interface{}{})

	output := buf.String()

	if !strings.Contains(output, "security.violation") {
		t.Errorf("Expected 'security.violation' in output, got: %s", output)
	}
}

// TestAuditEventConstants tests that audit event constants are defined correctly
func TestAuditEventConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"ProjectCreated", AuditEventProjectCreated, "project.created"},
		{"ProjectUpdated", AuditEventProjectUpdated, "project.updated"},
		{"ProjectDeleted", AuditEventProjectDeleted, "project.deleted"},
		{"MemberAdded", AuditEventMemberAdded, "project.member.added"},
		{"MemberRemoved", AuditEventMemberRemoved, "project.member.removed"},
		{"MemberRoleUpdate", AuditEventMemberRoleUpdate, "project.member.role_updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s = %s, got %s", tt.name, tt.expected, tt.constant)
			}
		})
	}
}
