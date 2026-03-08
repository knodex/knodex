// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package health

import (
	"context"
	"testing"
)

func TestChecker_CheckLiveness(t *testing.T) {
	t.Parallel()

	// Create checker without any clients
	checker := NewChecker(nil, nil, nil)

	status := checker.CheckLiveness(context.Background())

	if status.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, status.Status)
	}
}

func TestChecker_CheckReadiness_NoClients(t *testing.T) {
	t.Parallel()

	// Create checker without any clients
	checker := NewChecker(nil, nil, nil)

	status := checker.CheckReadiness(context.Background())

	// Should be healthy when no clients are configured
	if status.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, status.Status)
	}

	// Should have no components when no clients are configured
	if len(status.Components) != 0 {
		t.Errorf("expected 0 components, got %d", len(status.Components))
	}
}

func TestHealthStatus_JSON(t *testing.T) {
	t.Parallel()

	status := &HealthStatus{
		Status: StatusHealthy,
		Components: []ComponentHealth{
			{Name: "redis", Status: StatusHealthy},
			{Name: "kubernetes", Status: StatusHealthy},
		},
	}

	if status.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, status.Status)
	}

	if len(status.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(status.Components))
	}
}

// mockRBACHealth implements RBACHealth for testing
type mockRBACHealth struct {
	synced bool
}

func (m *mockRBACHealth) IsPolicySynced() bool {
	return m.synced
}

func TestChecker_CheckReadiness_RBACNotSynced(t *testing.T) {
	t.Parallel()

	checker := NewChecker(nil, nil, nil)
	checker.SetRBACHealth(&mockRBACHealth{synced: false})

	status := checker.CheckReadiness(context.Background())

	if status.Status != StatusUnhealthy {
		t.Errorf("expected status %s, got %s", StatusUnhealthy, status.Status)
	}

	// Should have RBAC component
	found := false
	for _, c := range status.Components {
		if c.Name == "rbac" {
			found = true
			if c.Status != StatusUnhealthy {
				t.Errorf("expected rbac component status %s, got %s", StatusUnhealthy, c.Status)
			}
			if c.Message != "initial policy sync in progress" {
				t.Errorf("expected message 'initial policy sync in progress', got %s", c.Message)
			}
		}
	}
	if !found {
		t.Error("expected rbac component in readiness check")
	}
}

func TestChecker_CheckReadiness_RBACSynced(t *testing.T) {
	t.Parallel()

	checker := NewChecker(nil, nil, nil)
	checker.SetRBACHealth(&mockRBACHealth{synced: true})

	status := checker.CheckReadiness(context.Background())

	if status.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, status.Status)
	}

	// Should have RBAC component marked healthy
	found := false
	for _, c := range status.Components {
		if c.Name == "rbac" {
			found = true
			if c.Status != StatusHealthy {
				t.Errorf("expected rbac component status %s, got %s", StatusHealthy, c.Status)
			}
		}
	}
	if !found {
		t.Error("expected rbac component in readiness check")
	}
}

func TestChecker_CheckReadiness_NilRBACHealth(t *testing.T) {
	t.Parallel()

	checker := NewChecker(nil, nil, nil)
	// Don't set RBAC health — should still be healthy with no rbac component

	status := checker.CheckReadiness(context.Background())

	if status.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, status.Status)
	}

	// Should have no RBAC component
	for _, c := range status.Components {
		if c.Name == "rbac" {
			t.Error("unexpected rbac component when RBACHealth is nil")
		}
	}
}

func TestComponentHealth_WithMessage(t *testing.T) {
	t.Parallel()

	component := ComponentHealth{
		Name:    "redis",
		Status:  StatusUnhealthy,
		Message: "connection refused",
	}

	if component.Status != StatusUnhealthy {
		t.Errorf("expected status %s, got %s", StatusUnhealthy, component.Status)
	}

	if component.Message != "connection refused" {
		t.Errorf("expected message 'connection refused', got %s", component.Message)
	}
}
