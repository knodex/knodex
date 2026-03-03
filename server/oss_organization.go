package main

import "github.com/provops-org/knodex/server/internal/config"

// InitOrganizationFilter returns empty string for OSS builds (no org filtering).
func InitOrganizationFilter(_ *config.Config) string {
	return ""
}
