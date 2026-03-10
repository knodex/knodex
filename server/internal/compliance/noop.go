// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package compliance

import "context"

// noopChecker is the default compliance checker for OSS builds.
// It always passes compliance checks and reports as disabled.
type noopChecker struct{}

// defaultChecker is the package-level compliance checker instance.
// In OSS builds, this is a noopChecker.
// In Enterprise builds, this is replaced via RegisterChecker() in init().
var defaultChecker ComplianceChecker = &noopChecker{}

// GetChecker returns the registered compliance checker.
// Returns the default noopChecker in OSS builds.
func GetChecker() ComplianceChecker {
	return defaultChecker
}

// AuditDeployment always returns a passing result for OSS builds.
// Enterprise builds provide actual compliance auditing.
func (n *noopChecker) AuditDeployment(_ context.Context, _ *Deployment) (*AuditResult, error) {
	return &AuditResult{
		Passed:   true,
		Findings: []Finding{},
	}, nil
}

// IsEnabled returns false for OSS builds.
// Compliance checking is only enabled in Enterprise builds.
func (n *noopChecker) IsEnabled() bool {
	return false
}

// Ensure noopChecker implements ComplianceChecker
var _ ComplianceChecker = (*noopChecker)(nil)
