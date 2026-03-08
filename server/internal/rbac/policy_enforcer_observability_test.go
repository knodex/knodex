// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPolicyEnforcer_Metrics tests the Metrics function
func TestPolicyEnforcer_Metrics(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Get initial metrics
	metrics := pe.Metrics()
	initialReloads := metrics.PolicyReloads
	initialSyncs := metrics.BackgroundSyncs

	// Increment metrics (only interface methods)
	pe.IncrementPolicyReloads()
	pe.IncrementBackgroundSyncs()

	// Get updated metrics
	metrics = pe.Metrics()
	assert.Equal(t, initialReloads+1, metrics.PolicyReloads)
	assert.Equal(t, initialSyncs+1, metrics.BackgroundSyncs)
}

// TestPolicyEnforcer_IncrementPolicyReloads tests the IncrementPolicyReloads function
func TestPolicyEnforcer_IncrementPolicyReloads(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Get initial value
	metrics := pe.Metrics()
	initial := metrics.PolicyReloads

	// Increment multiple times
	pe.IncrementPolicyReloads()
	pe.IncrementPolicyReloads()
	pe.IncrementPolicyReloads()

	// Verify counter incremented correctly
	metrics = pe.Metrics()
	assert.Equal(t, initial+3, metrics.PolicyReloads)
}

// TestPolicyEnforcer_IncrementBackgroundSyncs tests the IncrementBackgroundSyncs function
func TestPolicyEnforcer_IncrementBackgroundSyncs(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Get initial value
	metrics := pe.Metrics()
	initial := metrics.BackgroundSyncs

	// Increment multiple times
	pe.IncrementBackgroundSyncs()
	pe.IncrementBackgroundSyncs()

	// Verify counter incremented correctly
	metrics = pe.Metrics()
	assert.Equal(t, initial+2, metrics.BackgroundSyncs)
}
