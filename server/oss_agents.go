// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides OSS (non-enterprise) agent stubs.
package main

import (
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/services"
)

// InitAgentSpecValidator returns nil for OSS builds (Story 50.3): no
// Gatekeeper policy validation — the run result carries no policyValidation
// and the web renders its Enterprise notice off that absence.
func InitAgentSpecValidator(_ *config.Kubernetes, _ services.LicenseService) handlers.AgentSpecValidator {
	return nil
}
