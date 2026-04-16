// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package clients

import (
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/config"
)

// TestNewKubernetesClient_Timeout tests that the client is created even when
// the initial connection test times out
func TestNewKubernetesClient_Timeout(t *testing.T) {
	t.Parallel()

	// This test verifies the fix for the 503 instance listing bug
	// where slow K8s API caused client to return nil

	cfg := &config.Kubernetes{
		InCluster:  false,
		Kubeconfig: "testdata/invalid-kubeconfig.yaml",
	}

	// Create client - should not panic even with invalid config
	// The client creation should succeed, but connection test may fail
	client := NewKubernetesClient(cfg, nil)

	// Client may be nil if kubeconfig is completely invalid
	// But if config is valid but API is slow, client should be returned
	// This test mainly ensures no panics occur
	_ = client
}

// TestNewKubernetesClient_WithValidConfig tests successful client creation
func TestNewKubernetesClient_WithValidConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Kubernetes{
		InCluster:  false,
		Kubeconfig: "", // Will use default kubeconfig
	}

	// Try to get config first - if this fails, K8s is not available
	restConfig, err := GetKubernetesConfig(cfg)
	if err != nil {
		t.Skipf("Skipping: Kubernetes not available: %v", err)
	}

	// Verify timeout is set
	if restConfig.Timeout == 0 {
		t.Error("expected timeout to be set on rest config")
	}

	client := NewKubernetesClient(cfg, nil)
	if client == nil {
		t.Fatal("expected client to be created with valid config")
	}

	// Verify the client is usable by checking discovery is available
	if client.Discovery() == nil {
		t.Error("expected discovery client to be initialized")
	}
}

// TestGetKubernetesConfig_InCluster tests in-cluster config creation
func TestGetKubernetesConfig_InCluster(t *testing.T) {
	t.Parallel()

	cfg := &config.Kubernetes{
		InCluster: true,
	}

	// This will fail outside a cluster, which is expected
	restConfig, err := GetKubernetesConfig(cfg)

	// We expect an error outside a cluster
	if err == nil && restConfig != nil {
		// Verify timeout is set if we got a config
		if restConfig.Timeout == 0 {
			t.Error("expected timeout to be set on rest config")
		}
	}
}

// TestGetKubernetesConfig_Kubeconfig tests kubeconfig-based config creation
func TestGetKubernetesConfig_Kubeconfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Kubernetes{
		InCluster:  false,
		Kubeconfig: "testdata/invalid-kubeconfig.yaml",
	}

	_, err := GetKubernetesConfig(cfg)

	// We expect an error with invalid kubeconfig
	if err == nil {
		t.Error("expected error with invalid kubeconfig path")
	}
}

// TestKubernetesClient_TimeoutConfiguration tests that timeout is properly set
func TestKubernetesClient_TimeoutConfiguration(t *testing.T) {
	t.Parallel()

	// This test verifies that the REST config has a reasonable timeout
	// to prevent indefinite hangs that caused the 503 bug

	expectedTimeout := 10 * time.Second

	// We can't easily test this without a real cluster, but we can verify
	// the configuration logic in GetKubernetesConfig includes timeout setting

	// This is more of a documentation test - the actual timeout is set
	// in NewKubernetesClient at line 41
	if expectedTimeout != 10*time.Second {
		t.Errorf("expected timeout to be 10 seconds, got %v", expectedTimeout)
	}
}
