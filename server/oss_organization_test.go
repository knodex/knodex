package main

import (
	"testing"

	"github.com/provops-org/knodex/server/internal/config"
)

func TestInitOrganizationFilter_OSS_ReturnsEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.Organization = "my-org" // Even if org is set in config, OSS should return ""

	result := InitOrganizationFilter(cfg)
	if result != "" {
		t.Errorf("OSS build should return empty string, got %q", result)
	}
}
