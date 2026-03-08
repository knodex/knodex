package main

import "github.com/knodex/knodex/server/internal/config"

// InitOrganizationFilter returns empty string for OSS builds (no org filtering).
func InitOrganizationFilter(_ *config.Config) string {
	return ""
}
