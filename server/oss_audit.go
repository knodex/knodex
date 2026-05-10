// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides OSS (non-enterprise) audit stubs.
package main

import (
	"context"
	"net/http"

	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
)

// InitAuditRecorder returns nil for OSS builds.
// Handlers use audit.RecordEvent() which is nil-safe.
func InitAuditRecorder(_ context.Context, _ kubernetes.Interface, _ string, _ string) audit.Recorder {
	return nil
}

// InitAuditLoginMiddleware returns nil for OSS builds.
// Login routes are not wrapped with audit middleware.
func InitAuditLoginMiddleware(_ context.Context, _ kubernetes.Interface, _ string, _ string) func(http.Handler) http.Handler {
	return nil
}

// InitAuditMiddleware returns nil for OSS builds.
// Protected routes are not wrapped with audit middleware for 401/403 events.
func InitAuditMiddleware(_ context.Context, _ kubernetes.Interface, _ string, _ string) func(http.Handler) http.Handler {
	return nil
}

// InitAuditAPIService returns nil for OSS builds.
// Routes are not registered (404 returned for audit API endpoints).
func InitAuditAPIService(_ context.Context, _ kubernetes.Interface, _ string, _ string, _ rbac.PolicyEnforcer, _ audit.Recorder) services.AuditAPIService {
	return nil
}
