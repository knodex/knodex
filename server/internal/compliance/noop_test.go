// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package compliance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoopChecker_IsEnabled tests that noopChecker reports as disabled
func TestNoopChecker_IsEnabled(t *testing.T) {
	checker := &noopChecker{}

	enabled := checker.IsEnabled()

	assert.False(t, enabled, "noopChecker should report as disabled in OSS builds")
}

// TestNoopChecker_AuditDeployment tests that noopChecker always passes
func TestNoopChecker_AuditDeployment(t *testing.T) {
	checker := &noopChecker{}
	ctx := context.Background()

	tests := []struct {
		name       string
		deployment *Deployment
	}{
		{
			name:       "nil deployment",
			deployment: nil,
		},
		{
			name:       "empty deployment",
			deployment: &Deployment{},
		},
		{
			name: "deployment with name only",
			deployment: &Deployment{
				Name: "test-deployment",
			},
		},
		{
			name: "full deployment",
			deployment: &Deployment{
				Name:      "my-app",
				Namespace: "production",
				ProjectID: "project-1",
				RGDName:   "webapp-rgd",
				Inputs: map[string]interface{}{
					"replicas": 3,
					"image":    "nginx:latest",
				},
				Labels: map[string]string{
					"app": "my-app",
					"env": "prod",
				},
				Annotations: map[string]string{
					"description": "Production deployment",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.AuditDeployment(ctx, tt.deployment)

			require.NoError(t, err, "noopChecker should never return an error")
			require.NotNil(t, result, "AuditResult should not be nil")
			assert.True(t, result.Passed, "noopChecker should always pass")
			assert.Empty(t, result.Findings, "noopChecker should have no findings")
		})
	}
}

// TestNoopChecker_AuditDeployment_ContextCancellation tests context handling
func TestNoopChecker_AuditDeployment_ContextCancellation(t *testing.T) {
	checker := &noopChecker{}

	// Even with cancelled context, noopChecker should pass
	// (it doesn't do any actual work that could be cancelled)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := checker.AuditDeployment(ctx, &Deployment{Name: "test"})

	require.NoError(t, err, "noopChecker ignores context cancellation")
	assert.True(t, result.Passed, "noopChecker should still pass with cancelled context")
}

// TestNoopChecker_ImplementsInterface verifies noopChecker implements ComplianceChecker
func TestNoopChecker_ImplementsInterface(t *testing.T) {
	var checker ComplianceChecker = &noopChecker{}
	require.NotNil(t, checker, "noopChecker should implement ComplianceChecker")
}

// TestGetChecker_ReturnsDefaultChecker tests that GetChecker returns the default checker
func TestGetChecker_ReturnsDefaultChecker(t *testing.T) {
	// Reset to default state
	defaultChecker = &noopChecker{}

	checker := GetChecker()

	require.NotNil(t, checker, "GetChecker should never return nil")
	assert.False(t, checker.IsEnabled(), "Default checker should report as disabled")
}

// TestGetChecker_DefaultBehavior tests the default checker behavior
func TestGetChecker_DefaultBehavior(t *testing.T) {
	// Reset to default state
	defaultChecker = &noopChecker{}

	checker := GetChecker()
	ctx := context.Background()

	// Test AuditDeployment
	result, err := checker.AuditDeployment(ctx, &Deployment{
		Name:      "test",
		Namespace: "default",
	})

	require.NoError(t, err, "Default checker should not return errors")
	assert.True(t, result.Passed, "Default checker should always pass")
	assert.Empty(t, result.Findings, "Default checker should have no findings")

	// Test IsEnabled
	assert.False(t, checker.IsEnabled(), "Default checker should be disabled")
}
