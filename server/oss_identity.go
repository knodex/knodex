// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS identity hooks stub: OSS persists the canonical
// user roster (Story 15.2 / R5-5) but has no audit subsystem, so the hooks are
// the zero value (INFO logs from the base store only, no audit emission).
package main

import (
	"log/slog"

	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/services"
)

// InitIdentityHooks returns zero-value hooks on OSS (no audit emission).
func InitIdentityHooks(_ audit.Recorder, _ *slog.Logger) services.IdentityHooks {
	return services.IdentityHooks{}
}
