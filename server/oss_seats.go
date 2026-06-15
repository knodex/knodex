// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS no-op for the EE license seat reconciler. OSS
// has no license to enforce, so the function returns nil and app.Run starts no
// goroutine — the license service returns the cold-start sentinel.
package main

import (
	"context"
	"log/slog"

	"github.com/knodex/knodex/server/internal/services"
)

// InitSeatReconciler is a no-op in OSS builds.
func InitSeatReconciler(_ services.LicenseService, _ services.IdentityService, _ string, _ *slog.Logger) func(context.Context) {
	return nil
}
