// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides OSS (non-enterprise) license stubs.
package main

import "github.com/knodex/knodex/server/internal/services"

// InitLicenseService returns a NoopLicenseService for OSS builds.
// Enterprise license validation requires an Enterprise build.
func InitLicenseService(_, _ string) services.LicenseService {
	return &services.NoopLicenseService{}
}
