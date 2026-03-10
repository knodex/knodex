// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"testing"

	"github.com/knodex/knodex/server/internal/config"
)

func TestInitOrganizationFilter_OSS_ReturnsEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.Organization = "my-org" // Even if org is set in config, OSS should return ""

	result := InitOrganizationFilter(cfg)
	if result != "" {
		t.Errorf("OSS build should return empty string, got %q", result)
	}
}
