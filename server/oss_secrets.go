// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

// InitSecretsEnabled returns false for OSS builds.
// Secrets management requires an Enterprise license.
func InitSecretsEnabled() bool {
	return false
}
