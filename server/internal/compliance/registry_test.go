// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package compliance

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared package-level
// defaultChecker variable mutated by RegisterChecker() and resetDefaultChecker().
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChecker is a test implementation of ComplianceChecker
type mockChecker struct {
	enabled  bool
	passed   bool
	findings []Finding
	err      error
}

func (m *mockChecker) AuditDeployment(_ context.Context, _ *Deployment) (*AuditResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &AuditResult{
		Passed:   m.passed,
		Findings: m.findings,
	}, nil
}

func (m *mockChecker) IsEnabled() bool {
	return m.enabled
}

// Ensure mockChecker implements ComplianceChecker
var _ ComplianceChecker = (*mockChecker)(nil)

// resetDefaultChecker resets the defaultChecker to noopChecker for test isolation
func resetDefaultChecker() {
	defaultChecker = &noopChecker{}
}

// TestRegisterChecker_ReplacesDefault tests that RegisterChecker replaces the default
func TestRegisterChecker_ReplacesDefault(t *testing.T) {
	// Ensure clean state
	resetDefaultChecker()
	defer resetDefaultChecker()

	// Verify default is noop
	assert.False(t, GetChecker().IsEnabled(), "Default should be disabled")

	// Register enterprise checker
	enterpriseChecker := &mockChecker{enabled: true}
	RegisterChecker(enterpriseChecker)

	// Verify replacement
	checker := GetChecker()
	assert.True(t, checker.IsEnabled(), "Registered checker should be enabled")
	assert.Same(t, enterpriseChecker, checker, "GetChecker should return the registered checker")
}

// TestRegisterChecker_NilIgnored tests that nil registration is safely ignored
func TestRegisterChecker_NilIgnored(t *testing.T) {
	// Ensure clean state
	resetDefaultChecker()
	defer resetDefaultChecker()

	// Get reference to current checker
	originalChecker := GetChecker()
	assert.False(t, originalChecker.IsEnabled(), "Should start with noop checker")

	// Try to register nil
	RegisterChecker(nil)

	// Verify no change
	assert.Same(t, originalChecker, GetChecker(), "Nil registration should be ignored")
	assert.False(t, GetChecker().IsEnabled(), "Checker should still be disabled")
}

// TestRegisterChecker_CanRegisterMultipleTimes tests multiple registrations
func TestRegisterChecker_CanRegisterMultipleTimes(t *testing.T) {
	// Ensure clean state
	resetDefaultChecker()
	defer resetDefaultChecker()

	// First registration
	checker1 := &mockChecker{enabled: true, passed: true}
	RegisterChecker(checker1)
	assert.True(t, GetChecker().IsEnabled(), "First checker should be active")

	// Second registration replaces first
	checker2 := &mockChecker{enabled: true, passed: false}
	RegisterChecker(checker2)
	assert.Same(t, checker2, GetChecker(), "Second registration should replace first")
}

// TestRegisterChecker_WithCustomBehavior tests registering checker with custom behavior
func TestRegisterChecker_WithCustomBehavior(t *testing.T) {
	// Ensure clean state
	resetDefaultChecker()
	defer resetDefaultChecker()

	// Create checker that fails audits
	failingChecker := &mockChecker{
		enabled: true,
		passed:  false,
		findings: []Finding{
			{
				Severity:    SeverityCritical,
				Rule:        "test-rule-001",
				Description: "Test failure",
				Passed:      false,
				Remediation: "Fix the issue",
			},
		},
	}

	RegisterChecker(failingChecker)

	// Verify custom behavior
	checker := GetChecker()
	require.True(t, checker.IsEnabled(), "Custom checker should be enabled")

	result, err := checker.AuditDeployment(context.Background(), &Deployment{Name: "test"})
	require.NoError(t, err)
	assert.False(t, result.Passed, "Custom checker should fail audit")
	assert.Len(t, result.Findings, 1, "Should have one finding")
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
}

// TestRegisterChecker_PreservesCheckerState tests that registered checker maintains state
func TestRegisterChecker_PreservesCheckerState(t *testing.T) {
	// Ensure clean state
	resetDefaultChecker()
	defer resetDefaultChecker()

	// Create and register checker
	customChecker := &mockChecker{
		enabled: true,
		passed:  true,
		findings: []Finding{
			{
				Severity:    SeverityLow,
				Rule:        "info-001",
				Description: "Informational",
				Passed:      true,
			},
		},
	}

	RegisterChecker(customChecker)

	// Multiple calls should return same checker
	checker1 := GetChecker()
	checker2 := GetChecker()
	assert.Same(t, checker1, checker2, "GetChecker should return same instance")
	assert.Same(t, customChecker, checker1, "Should be the registered checker")
}

// TestDefaultCheckerAfterReset verifies resetDefaultChecker works correctly
func TestDefaultCheckerAfterReset(t *testing.T) {
	// Register custom checker
	customChecker := &mockChecker{enabled: true}
	RegisterChecker(customChecker)
	assert.True(t, GetChecker().IsEnabled(), "Custom checker should be active")

	// Reset
	resetDefaultChecker()

	// Verify back to noop
	assert.False(t, GetChecker().IsEnabled(), "Should be back to noop checker")
}
