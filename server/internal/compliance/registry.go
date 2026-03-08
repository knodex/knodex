// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package compliance

// RegisterChecker allows EE code to register a custom compliance checker.
// This function is called by Enterprise Edition code via init() to replace
// the default no-op checker with a full compliance implementation.
//
// Thread-safety: This function should only be called during package init(),
// before any concurrent access to the checker. After init() completes,
// the checker is read-only.
//
// Usage (in ee/compliance/init.go):
//
//	//go:build enterprise
//
//	package compliance
//
//	import compliance "github.com/knodex/knodex/server/internal/compliance"
//
//	func init() {
//	    compliance.RegisterChecker(&enterpriseChecker{})
//	}
func RegisterChecker(checker ComplianceChecker) {
	if checker == nil {
		// Silently ignore nil registration to prevent panics.
		// This allows defensive coding in EE init() functions.
		return
	}
	defaultChecker = checker
}
