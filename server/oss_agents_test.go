// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"testing"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/services"
)

// TestInitAgentSpecValidator_OSS pins the OSS guarantee (Story 50.3 AC #3):
// the dispatch returns nil regardless of inputs — no Gatekeeper validation,
// no EE code linked, the result carries no policyValidation.
func TestInitAgentSpecValidator_OSS(t *testing.T) {
	if got := InitAgentSpecValidator(&config.Kubernetes{}, nil); got != nil {
		t.Errorf("OSS init must return nil, got %T", got)
	}
	if got := InitAgentSpecValidator(nil, &services.NoopLicenseService{}); got != nil {
		t.Errorf("OSS init must return nil regardless of arguments, got %T", got)
	}
}
