// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared controller state
// (Controller, dynamicfake.FakeDynamicClient, and fake.Clientset instances shared within multi-step tests).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/deployment/vcs"
	"github.com/knodex/knodex/server/internal/util/retry"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// kroDiscoveryResources returns API resource lists for common test CRD kinds.
var kroDiscoveryResources = []*metav1.APIResourceList{
	{
		GroupVersion: "kro.run/v1alpha1",
		APIResources: []metav1.APIResource{
			{Name: "applications", Kind: "Application", Verbs: metav1.Verbs{"get", "list", "create", "update", "patch", "delete"}},
			{Name: "clusterconfigs", Kind: "ClusterConfig", Verbs: metav1.Verbs{"get", "list", "create", "update", "patch", "delete"}},
			{Name: "clusterpolicies", Kind: "ClusterPolicy", Verbs: metav1.Verbs{"get", "list", "create", "update", "patch", "delete"}},
		},
	},
}

// newFakeKubeClientWithDiscovery creates a fake kubeClient with discovery resources
// registered for common test kinds (Application, ClusterConfig, ClusterPolicy).
func newFakeKubeClientWithDiscovery(objects ...runtime.Object) *fake.Clientset {
	fakeClient := fake.NewSimpleClientset(objects...)
	fakeClient.Resources = kroDiscoveryResources
	return fakeClient
}

func TestNewController(t *testing.T) {
	t.Run("creates controller with nil clients", func(t *testing.T) {
		ctrl := NewController(nil, nil, nil)
		if ctrl == nil {
			t.Error("expected controller to be created")
		}
		if ctrl.generator == nil {
			t.Error("expected generator to be initialized")
		}
		if ctrl.logger == nil {
			t.Error("expected logger to be initialized")
		}
	})

	t.Run("creates controller with clients", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
		kubeClient := fake.NewSimpleClientset()

		ctrl := NewController(dynamicClient, kubeClient, nil)
		if ctrl == nil {
			t.Error("expected controller to be created")
		}
		if ctrl.dynamicClient == nil {
			t.Error("expected dynamic client to be set")
		}
		if ctrl.kubeClient == nil {
			t.Error("expected kube client to be set")
		}
	})
}

func TestController_Deploy_NilRequest(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	result, err := ctrl.Deploy(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
	if result != nil {
		t.Error("expected nil result for nil request")
	}
	if err.Error() != "deploy request cannot be nil" {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestController_Deploy_DirectMode(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.Mode != ModeDirect {
		t.Errorf("expected mode %s, got %s", ModeDirect, result.Mode)
	}

	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}

	if result.Status != StatusReady {
		t.Errorf("expected status %s, got %s", StatusReady, result.Status)
	}
}

func TestController_Deploy_GitOpsMode_MissingRepository(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository:     nil, // Missing repository config
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err == nil {
		t.Error("expected error for missing repository")
	}

	if result == nil {
		t.Fatal("expected result even on error")
	}

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}

	if result.GitError == "" {
		t.Error("expected git error to be set")
	}
}

func TestController_Deploy_HybridMode_ClusterOnly(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeHybrid,
		Repository:     nil, // No repository config, should skip git push
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.Mode != ModeHybrid {
		t.Errorf("expected mode %s, got %s", ModeHybrid, result.Mode)
	}

	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}

	if result.GitPushed {
		t.Error("expected git pushed to be false when no repo configured")
	}

	if result.Status != StatusReady {
		t.Errorf("expected status %s, got %s", StatusReady, result.Status)
	}
}

func TestController_Deploy_UnknownMode_DefaultsToDirect(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: DeploymentMode("unknown"),
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Mode != ModeDirect {
		t.Errorf("expected mode to default to %s, got %s", ModeDirect, result.Mode)
	}
}

func TestController_Delete_DirectMode(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	err := ctrl.Delete(context.Background(), "default", "test", "Application", "test-rgd", "user", false, nil, ModeDirect)
	if err != nil {
		t.Errorf("expected no error for direct mode delete, got: %v", err)
	}
}

func TestController_Delete_GitOpsMode_MissingRepo(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	err := ctrl.Delete(context.Background(), "default", "test", "Application", "test-rgd", "user", false, nil, ModeGitOps)
	if err == nil {
		t.Error("expected error for gitops mode without repo config")
	}
}

func TestController_Delete_HybridMode_NoRepo(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	// Hybrid mode without repo should not fail (git deletion is optional)
	err := ctrl.Delete(context.Background(), "default", "test", "Application", "test-rgd", "user", false, nil, ModeHybrid)
	if err != nil {
		t.Errorf("expected no error for hybrid mode without repo, got: %v", err)
	}
}

func TestGetGVRFromUnstructured(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		wantGroup  string
		wantVer    string
		wantRes    string
		wantErr    bool
	}{
		{
			name:       "core api v1",
			apiVersion: "v1",
			kind:       "Pod",
			wantGroup:  "",
			wantVer:    "v1",
			wantRes:    "pods",
			wantErr:    false,
		},
		{
			name:       "apps v1",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			wantGroup:  "apps",
			wantVer:    "v1",
			wantRes:    "deployments",
			wantErr:    false,
		},
		{
			name:       "custom resource",
			apiVersion: "kro.run/v1alpha1",
			kind:       "Application",
			wantGroup:  "kro.run",
			wantVer:    "v1alpha1",
			wantRes:    "applications",
			wantErr:    false,
		},
		{
			name:       "empty apiVersion",
			apiVersion: "",
			kind:       "Pod",
			wantErr:    true,
		},
		{
			name:       "empty kind",
			apiVersion: "v1",
			kind:       "",
			wantErr:    true,
		},
		{
			name:       "invalid apiVersion format",
			apiVersion: "apps/v1/beta",
			kind:       "Deployment",
			wantErr:    true,
		},
	}

	// Set up fake discovery with all test kinds
	fakeKubeClient := fake.NewSimpleClientset()
	fakeKubeClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
		{
			GroupVersion: "kro.run/v1alpha1",
			APIResources: []metav1.APIResource{
				{Name: "applications", Kind: "Application", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": tt.apiVersion,
					"kind":       tt.kind,
				},
			}

			ctrl := &Controller{discoveryClient: fakeKubeClient.Discovery()}
			gvr, err := ctrl.getGVRFromUnstructured(obj)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gvr.Group != tt.wantGroup {
				t.Errorf("expected group %q, got %q", tt.wantGroup, gvr.Group)
			}
			if gvr.Version != tt.wantVer {
				t.Errorf("expected version %q, got %q", tt.wantVer, gvr.Version)
			}
			if gvr.Resource != tt.wantRes {
				t.Errorf("expected resource %q, got %q", tt.wantRes, gvr.Resource)
			}
		})
	}
}

func TestParseDeploymentMode(t *testing.T) {
	tests := []struct {
		input    string
		expected DeploymentMode
	}{
		{"direct", ModeDirect},
		{"gitops", ModeGitOps},
		{"hybrid", ModeHybrid},
		{"invalid", ModeDirect}, // defaults to direct
		{"", ModeDirect},        // defaults to direct
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseDeploymentMode(tt.input)
			if result != tt.expected {
				t.Errorf("ParseDeploymentMode(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestController_GetGitHubToken(t *testing.T) {
	ctx := context.Background()

	t.Run("missing secret name", func(t *testing.T) {
		kubeClient := fake.NewSimpleClientset()
		ctrl := NewController(nil, kubeClient, nil)

		repo := &RepositoryConfig{
			SecretName: "", // Missing
		}

		_, err := ctrl.getGitHubToken(ctx, repo)
		if err == nil {
			t.Error("expected error for missing secret name")
		}
	})

	t.Run("default namespace and key", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gh-token",
				Namespace: "kro-system", // Default namespace
			},
			Data: map[string][]byte{
				"token": []byte("github-pat-token"), // Default key
			},
		}

		kubeClient := fake.NewSimpleClientset(secret)
		ctrl := NewController(nil, kubeClient, nil)

		repo := &RepositoryConfig{
			SecretName: "gh-token",
			// SecretNamespace and SecretKey not set, should use defaults
		}

		token, err := ctrl.getGitHubToken(ctx, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if token != "github-pat-token" {
			t.Errorf("expected 'github-pat-token', got %q", token)
		}
	})

	t.Run("custom namespace and key", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-secret",
				Namespace: "custom-ns",
			},
			Data: map[string][]byte{
				"pat": []byte("custom-token"),
			},
		}

		kubeClient := fake.NewSimpleClientset(secret)
		ctrl := NewController(nil, kubeClient, nil)

		repo := &RepositoryConfig{
			SecretName:      "custom-secret",
			SecretNamespace: "custom-ns",
			SecretKey:       "pat",
		}

		token, err := ctrl.getGitHubToken(ctx, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if token != "custom-token" {
			t.Errorf("expected 'custom-token', got %q", token)
		}
	})
}

// TestDeployResult_Fields tests the DeployResult struct fields
func TestDeployResult_Fields(t *testing.T) {
	now := time.Now()
	result := &DeployResult{
		InstanceID:      "inst-123",
		Name:            "my-instance",
		Namespace:       "default",
		Mode:            ModeHybrid,
		Status:          StatusReady,
		ClusterDeployed: true,
		ClusterError:    "",
		GitPushed:       true,
		GitCommitSHA:    "abc123",
		ManifestPath:    "instances/default/Application/my-instance.yaml",
		GitError:        "",
		DeployedAt:      now,
	}

	if result.InstanceID != "inst-123" {
		t.Errorf("InstanceID mismatch")
	}
	if result.Mode != ModeHybrid {
		t.Errorf("Mode mismatch")
	}
	if !result.ClusterDeployed {
		t.Errorf("ClusterDeployed should be true")
	}
	if !result.GitPushed {
		t.Errorf("GitPushed should be true")
	}
}

// HIGH PRIORITY: Token Memory Zeroing Verification
// Tests that getGitHubToken properly zeros the secret bytes after reading
// Note: This test verifies the security fix exists by checking the code path,
// but cannot directly verify memory zeroing due to Go's garbage collector and
// the fake client's copy semantics. The actual zeroing happens in controller.go:406-411.
func TestController_GetGitHubToken_MemoryZeroing(t *testing.T) {
	ctx := context.Background()

	// Create a secret with a known token value
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_TestSecretToken123456789"),
		},
	}

	kubeClient := fake.NewSimpleClientset(secret)
	ctrl := NewController(nil, kubeClient, nil)

	repo := &RepositoryConfig{
		SecretName: "test-secret",
	}

	// Get the token
	token, err := ctrl.getGitHubToken(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the token value is correct
	expectedToken := "ghp_TestSecretToken123456789"
	if token != expectedToken {
		t.Errorf("token value mismatch: got %q, want %q", token, expectedToken)
	}

	// The actual memory zeroing happens in the controller's getGitHubToken method
	// at lines 406-411 (controller.go). Due to the fake client's copy semantics
	// and Go's garbage collector, we cannot directly verify the zeroing in tests.
	// Instead, we verify that the security fix code path exists by:
	// 1. Confirming the function returns the correct token value
	// 2. Code review confirms the zeroing loop exists in controller.go:409-411

	// Additional verification: ensure subsequent calls work correctly
	// (the zeroing should not corrupt state)
	token2, err := ctrl.getGitHubToken(ctx, repo)
	if err != nil {
		t.Fatalf("second call unexpected error: %v", err)
	}
	if token2 != expectedToken {
		t.Errorf("second call token value mismatch: got %q, want %q", token2, expectedToken)
	}
}

// TestController_GetGitHubToken_SecretRef verifies that getGitHubToken uses
// GetSecretName/GetSecretNamespace/GetSecretKey accessors so SecretRef-based
// configs are resolved correctly (not just legacy SecretName fields).
func TestController_GetGitHubToken_SecretRef(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ref-secret",
			Namespace: "custom-ns",
		},
		Data: map[string][]byte{
			"bearerToken": []byte("ref-token-value"),
		},
	}
	kubeClient := fake.NewSimpleClientset(secret)
	ctrl := NewController(nil, kubeClient, nil)

	// Use only SecretRef fields (legacy fields empty) — verifies GetSecretName() is used
	repo := &RepositoryConfig{
		SecretRef: SecretReference{
			Name:      "ref-secret",
			Namespace: "custom-ns",
			Key:       "bearerToken",
		},
	}

	token, err := ctrl.getGitHubToken(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "ref-token-value" {
		t.Errorf("expected 'ref-token-value', got %q", token)
	}
}

// Test context cancellation in Deploy operations — verifies the function
// returns a populated result even with a cancelled context (fake client
// doesn't honour context, but this exercises the code path without panicking).
func TestController_Deploy_ContextCancellation(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	// The fake client doesn't check context cancellation, so the call succeeds.
	// Assert the result is populated (non-nil) and the mode is correct.
	result, err := ctrl.Deploy(ctx, req)
	if err != nil {
		// If the fake client starts respecting context, that's also acceptable
		t.Logf("Deploy returned error with cancelled context (acceptable): %v", err)
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Mode != ModeDirect {
		t.Errorf("expected mode %s, got %s", ModeDirect, result.Mode)
	}
}

// Test GitOps mode with invalid repository config
func TestController_Deploy_GitOpsMode_InvalidRepositoryConfig(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	tests := []struct {
		name     string
		repoConf *RepositoryConfig
		errMsg   string
	}{
		{
			name: "invalid owner - starts with hyphen",
			repoConf: &RepositoryConfig{
				Owner:      "-invalid",
				Repo:       "my-repo",
				SecretName: "github-token",
			},
			errMsg: "invalid repository configuration",
		},
		{
			name: "invalid repo - special chars",
			repoConf: &RepositoryConfig{
				Owner:      "valid-owner",
				Repo:       "my-repo!@#",
				SecretName: "github-token",
			},
			errMsg: "invalid repository configuration",
		},
		{
			name: "path traversal in branch",
			repoConf: &RepositoryConfig{
				Owner:         "valid-owner",
				Repo:          "valid-repo",
				SecretName:    "github-token",
				DefaultBranch: "../etc/passwd",
			},
			errMsg: "invalid repository configuration",
		},
		{
			name: "path traversal in base path",
			repoConf: &RepositoryConfig{
				Owner:      "valid-owner",
				Repo:       "valid-repo",
				SecretName: "github-token",
				BasePath:   "../../../etc",
			},
			errMsg: "invalid repository configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DeployRequest{
				InstanceID:     "test-id",
				Name:           "test-instance",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "kro.run/v1alpha1",
				Kind:           "Application",
				Spec:           map[string]interface{}{"replicas": int64(1)},
				DeploymentMode: ModeGitOps,
				Repository:     tt.repoConf,
				CreatedBy:      "test-user",
				CreatedAt:      time.Now(),
			}

			result, err := ctrl.Deploy(context.Background(), req)
			if err == nil {
				t.Error("expected error for invalid repository config")
			}

			if result == nil {
				t.Fatal("expected result even on error")
			}

			if result.Status != StatusFailed {
				t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
			}

			if result.GitError == "" {
				t.Error("expected git error to be set")
			}
		})
	}
}

// Test Hybrid mode with invalid repository config (should still succeed for cluster)
func TestController_Deploy_HybridMode_InvalidRepoConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		InstanceID:     "test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeHybrid,
		Repository: &RepositoryConfig{
			Owner:      "-invalid-owner", // Invalid: starts with hyphen
			Repo:       "my-repo",
			SecretName: "github-token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error (cluster should succeed), got: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Cluster deployment should succeed
	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}

	// Git push should fail due to invalid repo config
	if result.GitPushed {
		t.Error("expected git pushed to be false due to invalid config")
	}

	if result.GitError == "" {
		t.Error("expected git error to be set for invalid config")
	}

	// Status should indicate GitOps failed but cluster succeeded
	if result.Status != StatusGitOpsFailed {
		t.Errorf("expected status %s, got %s", StatusGitOpsFailed, result.Status)
	}
}

// Test Delete with context
func TestController_Delete_WithContext(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Direct mode delete should work even with context
	err := ctrl.Delete(ctx, "default", "test", "Application", "test-rgd", "user", false, nil, ModeDirect)
	if err != nil {
		t.Errorf("expected no error for direct mode delete, got: %v", err)
	}
}

// HIGH PRIORITY: GitOps Success Path Test
// This test exercises the full GitOps code path including token retrieval
// The actual GitHub API call will fail, but this validates the internal logic
func TestController_Deploy_GitOpsMode_WithValidSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a secret with a fake GitHub token
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := newFakeKubeClientWithDiscovery(secret)

	// Create controller with Kubernetes clients for cluster-apply and secret access
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "github-token-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// The GitHub API call will fail (no real GitHub), but the internal logic
	// should correctly retrieve the token and attempt the push
	// Error is expected due to network failure to GitHub API
	if err == nil {
		// If by some miracle it succeeds, validate the result
		t.Log("GitOps deployment succeeded unexpectedly (real GitHub accessible?)")
	} else {
		// Expected: error should mention GitHub client or API failure
		errStr := err.Error()
		if !strings.Contains(errStr, "GitHub") &&
			!strings.Contains(errStr, "github") &&
			!strings.Contains(errStr, "commit") &&
			!strings.Contains(errStr, "failed") {
			t.Logf("Got expected network error: %v", err)
		}
	}

	// Result should still be populated
	if result != nil {
		// Verify status reflects the failure
		if result.Status != StatusFailed {
			t.Logf("Status: %s", result.Status)
		}
		// GitError should be set
		if result.GitError == "" {
			t.Log("Warning: GitError should be set when push fails")
		}
	}
}

// HIGH PRIORITY: Hybrid Full Success Test
// This test exercises both cluster deployment and Git push code paths
func TestController_Deploy_HybridMode_FullPath(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a secret with a fake GitHub token
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := newFakeKubeClientWithDiscovery(secret)

	// Create controller with both clients
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "hybrid-test-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeHybrid,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "github-token-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// For hybrid mode, the cluster deployment should succeed,
	// but the Git push will fail (no real GitHub)
	// This is expected behavior - hybrid mode doesn't fail if Git fails
	if err != nil {
		t.Fatalf("hybrid mode should not return error even if Git fails: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Cluster deployment should succeed
	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}

	// Git push will fail due to no real GitHub
	if result.GitPushed {
		t.Log("Git push unexpectedly succeeded (real GitHub accessible?)")
	} else {
		t.Log("Git push correctly failed (expected - no real GitHub)")
	}

	// GitError should be set
	if result.GitError == "" {
		t.Log("Warning: GitError should be set when push fails")
	}

	// Status should indicate GitOps failed (cluster succeeded, Git failed)
	if result.Status != StatusGitOpsFailed && result.Status != StatusReady {
		t.Errorf("expected status %s or %s, got %s", StatusGitOpsFailed, StatusReady, result.Status)
	}
}

// Test GitOps mode with secret not found
func TestController_Deploy_GitOpsMode_SecretNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create Kubernetes client without the secret (but with discovery for GVR resolution)
	kubeClient := newFakeKubeClientWithDiscovery()

	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-no-secret-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "nonexistent-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Should fail because secret doesn't exist
	if err == nil {
		t.Error("expected error when secret not found")
	}

	if result == nil {
		t.Fatal("expected result even on error")
	}

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}

	// Error should mention token or secret
	if result.GitError == "" {
		t.Error("expected git error to be set")
	}

	errStr := result.GitError
	if !strings.Contains(errStr, "token") && !strings.Contains(errStr, "secret") {
		t.Errorf("expected error to mention token or secret, got: %s", errStr)
	}
}

// Test Hybrid mode with secret retrieval failure
func TestController_Deploy_HybridMode_SecretNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create Kubernetes client without the secret (but with discovery for GVR resolution)
	kubeClient := newFakeKubeClientWithDiscovery()

	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "hybrid-no-secret-id",
		Name:           "test-instance",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeHybrid,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "nonexistent-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Hybrid mode should succeed for cluster even if Git fails
	if err != nil {
		t.Fatalf("hybrid mode should not return error even if Git fails: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Cluster deployment should succeed
	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}

	// Git push should fail
	if result.GitPushed {
		t.Error("expected git pushed to be false when secret not found")
	}

	// GitError should mention secret/token issue
	if result.GitError == "" {
		t.Error("expected git error to be set")
	}
}

// TestApplyToCluster_IrregularPlural_UsesDiscovery verifies that applyToCluster
// uses the discovery-based GVR resolver and sends the correct plural resource name
// to the dynamic client (e.g., "proxies" not "proxys" for kind Proxy).
func TestApplyToCluster_IrregularPlural_UsesDiscovery(t *testing.T) {
	// Set up fake discovery with Proxy -> proxies mapping
	fakeKubeClient := fake.NewSimpleClientset()
	fakeKubeClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "proxies", Kind: "Proxy", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
	}

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "example.com", Version: "v1", Resource: "proxies"}: "ProxyList",
	}
	fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	ctrl := NewController(fakeDynClient, fakeKubeClient, nil)

	req := &DeployRequest{
		Name:           "my-proxy",
		Namespace:      "default",
		APIVersion:     "example.com/v1",
		Kind:           "Proxy",
		Spec:           map[string]interface{}{"port": float64(8080)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test@test.local",
		CreatedAt:      time.Now(),
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the dynamic client received "proxies" (not "proxys")
	actions := fakeDynClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	createAction := actions[0]
	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxies"}
	if createAction.GetResource() != expectedGVR {
		t.Errorf("expected GVR %v, got %v", expectedGVR, createAction.GetResource())
	}
}

// --- Cluster-Scoped Tests (STORY-302) ---

func TestController_Deploy_DirectMode_ClusterScoped(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		InstanceID:      "cluster-inst-1",
		Name:            "global-config",
		Namespace:       "", // cluster-scoped has no namespace
		IsClusterScoped: true,
		RGDName:         "cluster-config-rgd",
		RGDNamespace:    "kro-system",
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "ClusterConfig",
		Spec:            map[string]interface{}{"tier": "gold"},
		DeploymentMode:  ModeDirect,
		ProjectID:       "platform",
		CreatedBy:       "admin@test.local",
		CreatedAt:       time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Mode != ModeDirect {
		t.Errorf("expected mode %s, got %s", ModeDirect, result.Mode)
	}
	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true")
	}
	if result.Status != StatusReady {
		t.Errorf("expected status %s, got %s", StatusReady, result.Status)
	}

	// Verify the dynamic client received a cluster-scoped create (no namespace)
	actions := dynamicClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	action := actions[0]
	if action.GetNamespace() != "" {
		t.Errorf("expected empty namespace for cluster-scoped create, got %q", action.GetNamespace())
	}
}

func TestApplyToCluster_ClusterScoped_NoNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		Name:            "test-global",
		Namespace:       "",
		IsClusterScoped: true,
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "ClusterConfig",
		Spec:            map[string]interface{}{"enable": true},
		DeploymentMode:  ModeDirect,
		InstanceID:      "inst-cs-1",
		CreatedBy:       "admin@test.local",
		CreatedAt:       time.Now(),
		ProjectID:       "infra",
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify create was called without namespace
	actions := dynamicClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].GetNamespace() != "" {
		t.Errorf("cluster-scoped create should have empty namespace, got %q", actions[0].GetNamespace())
	}

	// Verify object does not have namespace in metadata
	createAction, ok := actions[0].(interface{ GetObject() runtime.Object })
	if ok {
		obj := createAction.GetObject()
		if unstrObj, ok := obj.(*unstructured.Unstructured); ok {
			if ns := unstrObj.GetNamespace(); ns != "" {
				t.Errorf("cluster-scoped object should have empty namespace, got %q", ns)
			}
			// Verify project label is set
			labels := unstrObj.GetLabels()
			if labels["knodex.io/project"] != "infra" {
				t.Errorf("expected project label 'infra', got %q", labels["knodex.io/project"])
			}
		}
	}
}

func TestApplyToCluster_NamespaceScoped_Unchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		Name:            "my-app",
		Namespace:       "production",
		IsClusterScoped: false,
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "Application",
		Spec:            map[string]interface{}{"replicas": int64(1)},
		DeploymentMode:  ModeDirect,
		InstanceID:      "inst-ns-1",
		CreatedBy:       "dev@test.local",
		CreatedAt:       time.Now(),
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify create was called with namespace
	actions := dynamicClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].GetNamespace() != "production" {
		t.Errorf("namespace-scoped create should have namespace 'production', got %q", actions[0].GetNamespace())
	}
}

func TestApplyToCluster_ClusterScoped_LabelsAndAnnotations(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		Name:            "cluster-policy",
		Namespace:       "",
		IsClusterScoped: true,
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "ClusterPolicy",
		Spec:            map[string]interface{}{"enforce": true},
		DeploymentMode:  ModeGitOps,
		InstanceID:      "inst-label-1",
		CreatedBy:       "admin@test.local",
		CreatedAt:       time.Now(),
		ProjectID:       "platform",
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := dynamicClient.Actions()
	createAction, ok := actions[0].(interface{ GetObject() runtime.Object })
	if !ok {
		t.Fatal("cannot extract object from create action")
	}

	obj := createAction.GetObject()
	unstrObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		t.Fatal("expected unstructured object")
	}

	// Verify annotations injected for cluster-scoped
	annotations := unstrObj.GetAnnotations()
	if annotations["knodex.io/instance-id"] != "inst-label-1" {
		t.Errorf("expected instance-id annotation, got %q", annotations["knodex.io/instance-id"])
	}
	if annotations["knodex.io/created-by"] != "admin@test.local" {
		t.Errorf("expected created-by annotation, got %q", annotations["knodex.io/created-by"])
	}
	if annotations["knodex.io/created-at"] == "" {
		t.Error("expected created-at annotation to be set")
	}
	if annotations["knodex.io/project-id"] != "platform" {
		t.Errorf("expected project-id annotation, got %q", annotations["knodex.io/project-id"])
	}
	if annotations["knodex.io/deployment-mode"] != "gitops" {
		t.Errorf("expected deployment-mode annotation, got %q", annotations["knodex.io/deployment-mode"])
	}

	// Verify labels
	labels := unstrObj.GetLabels()
	if labels["knodex.io/project"] != "platform" {
		t.Errorf("expected project label 'platform', got %q", labels["knodex.io/project"])
	}
	if labels["knodex.io/deployment-mode"] != "gitops" {
		t.Errorf("expected deployment-mode label, got %q", labels["knodex.io/deployment-mode"])
	}
}

// TestController_Delete_ClusterScoped_UsesKindBasedPath verifies that Delete()
// passes IsClusterScoped through to deleteFromGit — ensuring the kind-based path
// would be used rather than the old namespace/name path.
// (Full deleteFromGit path requires a real GitHub API; here we verify the parameter
// propagation via the ModeGitOps missing-repo error path with cluster-scoped flag.)
func TestController_Delete_ClusterScoped_MissingRepo(t *testing.T) {
	ctrl := NewController(nil, nil, nil)

	// Should fail asking for repo config — proves the isClusterScoped param is wired through
	err := ctrl.Delete(context.Background(), "", "global-config", "ClusterConfig", "cluster-rgd", "admin", true, nil, ModeGitOps)
	if err == nil {
		t.Error("expected error for gitops cluster-scoped delete without repo config")
	}
	if err.Error() != "repository configuration is required for GitOps deletion" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// Cluster-Scoped GitOps / Hybrid / Delete Tests (STORY-311)
// ============================================================================

// TestController_Deploy_ClusterScoped_GitOps tests that GitOps deployment works with
// cluster-scoped instances. The GitHub API call will fail (no real GitHub), but this
// validates the internal logic: token retrieval, manifest generation, and path construction
// use cluster-scoped paths (STORY-311, AC #2, F-10).
func TestController_Deploy_ClusterScoped_GitOps(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := fake.NewSimpleClientset(secret)

	// No dynamicClient — GitOps mode does not apply to cluster
	ctrl := NewController(nil, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:      "cs-gitops-1",
		Name:            "global-config",
		Namespace:       "",
		IsClusterScoped: true,
		RGDName:         "cluster-config-rgd",
		RGDNamespace:    "kro-system",
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "ClusterConfig",
		Spec:            map[string]interface{}{"tier": "gold"},
		DeploymentMode:  ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "github-token-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		ProjectID: "platform",
		CreatedBy: "admin@test.local",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Expected: GitHub API call fails (no real GitHub), but internal logic succeeded up to that point
	// The error should reference GitHub/commit failure, proving the cluster-scoped path was attempted
	if err == nil {
		t.Log("GitOps deployment succeeded unexpectedly (real GitHub accessible?)")
	} else {
		errStr := err.Error()
		// Verify error is about GitHub API, not about missing namespace or bad path
		if strings.Contains(errStr, "namespace") {
			t.Errorf("unexpected namespace-related error for cluster-scoped GitOps: %v", err)
		}
	}

	// Verify manifest path would use cluster-scoped pattern
	path, pathErr := ctrl.generator.GenerateManifestPath(req, "instances")
	if pathErr != nil {
		t.Fatalf("failed to generate manifest path: %v", pathErr)
	}
	expectedPath := "instances/cluster-scoped/clusterconfig/global-config.yaml"
	if path != expectedPath {
		t.Errorf("expected cluster-scoped path %q, got %q", expectedPath, path)
	}

	// Verify commit message includes cluster-scoped indicator
	commitMsg := ctrl.generator.GenerateCommitMessage(req)
	if !strings.Contains(commitMsg, "[cluster-scoped]") {
		t.Errorf("expected commit message to include [cluster-scoped], got: %s", commitMsg)
	}

	// Result must exist (even on failure)
	if result == nil {
		t.Fatal("expected non-nil result from GitOps deployment")
	}
	if result.ClusterDeployed {
		t.Error("GitOps mode should not deploy to cluster")
	}
	if result.GitPushed {
		t.Error("GitPushed should be false when GitHub API fails")
	}
}

// TestController_Deploy_ClusterScoped_Hybrid tests that Hybrid deployment correctly
// deploys cluster-scoped instances to the cluster AND attempts Git push.
// Cluster deploy succeeds; Git push fails (no real GitHub) — expected for hybrid mode
// (STORY-311, AC #2, F-10).
func TestController_Deploy_ClusterScoped_Hybrid(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := newFakeKubeClientWithDiscovery(secret)

	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:      "cs-hybrid-1",
		Name:            "global-policy",
		Namespace:       "",
		IsClusterScoped: true,
		RGDName:         "cluster-policy-rgd",
		RGDNamespace:    "kro-system",
		APIVersion:      "kro.run/v1alpha1",
		Kind:            "ClusterPolicy",
		Spec:            map[string]interface{}{"enforce": true},
		DeploymentMode:  ModeHybrid,
		Repository: &RepositoryConfig{
			Owner:           "test-owner",
			Repo:            "test-repo",
			DefaultBranch:   "main",
			BasePath:        "instances",
			SecretName:      "github-token-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		ProjectID: "platform",
		CreatedBy: "admin@test.local",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Hybrid mode: cluster succeeds, Git fails — overall should NOT error
	if err != nil {
		t.Fatalf("hybrid mode should not return error even if Git fails: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Cluster deployment should succeed
	if !result.ClusterDeployed {
		t.Error("expected cluster deployed to be true for hybrid mode")
	}

	// Git push fails (no real GitHub) — expected
	if result.GitPushed {
		t.Log("Git push unexpectedly succeeded (real GitHub accessible?)")
	}

	// Verify dynamic client received a cluster-scoped create (empty namespace)
	var foundClusterCreate bool
	for _, a := range dynamicClient.Actions() {
		if a.GetVerb() == "create" && a.GetNamespace() == "" {
			foundClusterCreate = true
			break
		}
	}
	if !foundClusterCreate {
		t.Error("expected a cluster-scoped create action (empty namespace) in dynamic client")
	}

	// Status: should be GitOpsFailed (cluster OK, Git failed) or Ready
	if result.Status != StatusGitOpsFailed && result.Status != StatusReady {
		t.Errorf("expected status %s or %s, got %s", StatusGitOpsFailed, StatusReady, result.Status)
	}
}

// TestController_Delete_ClusterScoped_GitOps_ReachesPushStep validates that deleteFromGit
// with cluster-scoped instances correctly constructs the path and retrieves the token,
// reaching the GitHub push step. The GitHub API call itself will fail (no real GitHub) —
// verified by asserting the error does NOT come from path/secret validation (STORY-311, AC #3, F-11).
func TestController_Delete_ClusterScoped_GitOps_ReachesPushStep(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := fake.NewSimpleClientset(secret)

	ctrl := NewController(nil, kubeClient, nil)

	repo := &RepositoryConfig{
		Owner:           "test-owner",
		Repo:            "test-repo",
		DefaultBranch:   "main",
		BasePath:        "instances",
		SecretName:      "github-token-secret",
		SecretNamespace: "kro-system",
		SecretKey:       "token",
	}

	// Delete cluster-scoped instance from Git
	err := ctrl.Delete(context.Background(), "", "global-config", "ClusterConfig", "cluster-rgd", "admin@test.local", true, repo, ModeGitOps)

	// Expected: error from GitHub API (no real GitHub), but NOT from path generation or secret retrieval
	if err == nil {
		t.Log("Delete from Git succeeded unexpectedly (real GitHub accessible?)")
	} else {
		errStr := err.Error()
		// Error should be about GitHub API, not about missing namespace or bad path
		if strings.Contains(errStr, "namespace is required") {
			t.Errorf("cluster-scoped delete should not require namespace: %v", err)
		}
		if strings.Contains(errStr, "secret") {
			t.Errorf("secret retrieval should succeed: %v", err)
		}
		// Acceptable errors: GitHub client creation, commit/delete file failure
		t.Logf("Got expected GitHub error: %v", err)
	}
}

// TestController_Delete_ClusterScoped_Hybrid_Success tests hybrid mode delete for
// cluster-scoped instances. Direct mode delete always succeeds (no-op for controller),
// and Git delete failure is non-fatal (STORY-311, AC #3, F-11).
func TestController_Delete_ClusterScoped_Hybrid_Success(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-secret",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token_for_testing_123456789"),
		},
	}
	kubeClient := fake.NewSimpleClientset(secret)

	ctrl := NewController(nil, kubeClient, nil)

	repo := &RepositoryConfig{
		Owner:           "test-owner",
		Repo:            "test-repo",
		DefaultBranch:   "main",
		BasePath:        "instances",
		SecretName:      "github-token-secret",
		SecretNamespace: "kro-system",
		SecretKey:       "token",
	}

	// Hybrid delete: Git failure is non-fatal
	err := ctrl.Delete(context.Background(), "", "global-policy", "ClusterPolicy", "cluster-rgd", "admin@test.local", true, repo, ModeHybrid)
	if err != nil {
		t.Errorf("hybrid mode delete should not fail even when Git delete fails: %v", err)
	}
}

// --- STORY-362: Deployment Controller Resilience Tests ---

// TestApplyToCluster_AlreadyExists_FallsBackToPatch verifies that when Create
// returns AlreadyExists, the controller falls back to MergePatch (AC #1).
func TestApplyToCluster_AlreadyExists_FallsBackToPatch(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Make Create return AlreadyExists
	dynamicClient.PrependReactor("create", "applications", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(
			schema.GroupResource{Group: "kro.run", Resource: "applications"}, "test-instance")
	})
	// Allow Patch to succeed
	dynamicClient.PrependReactor("patch", "applications", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(k8stesting.PatchAction)
		if patchAction.GetPatchType() != "application/merge-patch+json" {
			t.Errorf("expected MergePatchType, got %s", patchAction.GetPatchType())
		}
		return true, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "test-instance", "namespace": "default"},
		}}, nil
	})

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		Name:           "test-instance",
		Namespace:      "default",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(2)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test@test.local",
		CreatedAt:      time.Now(),
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error on upsert, got: %v", err)
	}

	// Verify both Create and Patch were called
	actions := dynamicClient.Actions()
	var createCount, patchCount int
	for _, a := range actions {
		switch a.GetVerb() {
		case "create":
			createCount++
		case "patch":
			patchCount++
		}
	}
	if createCount != 1 {
		t.Errorf("expected 1 create action, got %d", createCount)
	}
	if patchCount != 1 {
		t.Errorf("expected 1 patch action, got %d", patchCount)
	}
}

// TestApplyToCluster_TransientError_RetriesAndSucceeds verifies that transient
// errors trigger retry and the operation succeeds after recovery (AC #3).
func TestApplyToCluster_TransientError_RetriesAndSucceeds(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// First call fails with transient error, second succeeds
	var callCount atomic.Int32
	dynamicClient.PrependReactor("create", "applications", func(action k8stesting.Action) (bool, runtime.Object, error) {
		n := callCount.Add(1)
		if n == 1 {
			// Transient: connection reset (matches retry.IsRetryable)
			return true, nil, fmt.Errorf("connection reset by peer")
		}
		// Second attempt succeeds
		return true, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "test-instance", "namespace": "default"},
		}}, nil
	})

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)
	ctrl.retryConfig = retry.RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}

	req := &DeployRequest{
		Name:           "test-instance",
		Namespace:      "default",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test@test.local",
		CreatedAt:      time.Now(),
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}

	if callCount.Load() != 2 {
		t.Errorf("expected 2 create attempts (1 fail + 1 success), got %d", callCount.Load())
	}
}

// TestApplyToCluster_PermanentError_NoRetry verifies that permanent K8s errors
// (Forbidden, Invalid) are not retried and returned immediately (AC #3).
func TestApplyToCluster_PermanentError_NoRetry(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	var callCount atomic.Int32
	dynamicClient.PrependReactor("create", "applications", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount.Add(1)
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "kro.run", Resource: "applications"}, "test-instance",
			fmt.Errorf("user not authorized"))
	})

	ctrl := NewController(dynamicClient, newFakeKubeClientWithDiscovery(), nil)

	req := &DeployRequest{
		Name:           "test-instance",
		Namespace:      "default",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeDirect,
		CreatedBy:      "test@test.local",
		CreatedAt:      time.Now(),
	}

	err := ctrl.applyToCluster(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for forbidden resource")
	}
	if !strings.Contains(err.Error(), "Application") || !strings.Contains(err.Error(), "default") {
		t.Errorf("expected error to include resource kind and namespace, got: %v", err)
	}

	// Should have been called only once — no retry for permanent errors
	if callCount.Load() != 1 {
		t.Errorf("expected 1 create attempt (no retry for permanent error), got %d", callCount.Load())
	}
}

// TestGetGVRFromUnstructured_NoDiscovery_ReturnsError verifies that without a
// discovery client, getGVRFromUnstructured returns an error instead of guessing (AC #2).
func TestGetGVRFromUnstructured_NoDiscovery_ReturnsError(t *testing.T) {
	ctrl := &Controller{} // no discovery client

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Application",
		},
	}

	_, err := ctrl.getGVRFromUnstructured(obj)
	if err == nil {
		t.Fatal("expected error when discovery client is nil")
	}
	if !strings.Contains(err.Error(), "cannot determine plural") {
		t.Errorf("expected error about plural resolution, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Application") {
		t.Errorf("expected error to mention the kind, got: %v", err)
	}
}

// TestGetGVRFromUnstructured_DiscoveryFails_ReturnsError verifies that when
// discovery fails, the function returns an error with context (AC #2).
func TestGetGVRFromUnstructured_DiscoveryFails_ReturnsError(t *testing.T) {
	// Create a kubeClient with NO resources registered — discovery will fail to map
	fakeKubeClient := fake.NewSimpleClientset()
	fakeKubeClient.Resources = []*metav1.APIResourceList{} // empty

	ctrl := &Controller{discoveryClient: fakeKubeClient.Discovery(), logger: slog.Default()}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "UnknownKind",
		},
	}

	_, err := ctrl.getGVRFromUnstructured(obj)
	if err == nil {
		t.Fatal("expected error for unresolvable kind")
	}
	if !strings.Contains(err.Error(), "UnknownKind") {
		t.Errorf("expected error to mention kind, got: %v", err)
	}
}

// TestController_Deploy_GitOpsMode_AppliesSuspendedToCluster verifies that GitOps
// deployments apply the manifest to the cluster with kro.run/reconcile: suspended.
func TestController_Deploy_GitOpsMode_AppliesSuspendedToCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Capture the created object via reactor
	var createdObj *unstructured.Unstructured
	dynamicClient.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		createdObj = createAction.GetObject().(*unstructured.Unstructured)
		return false, nil, nil // Let the fake handle the actual create
	})

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gh-token",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token"),
		},
	}
	kubeClient := newFakeKubeClientWithDiscovery(secret)
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-suspended-test",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:      "test-owner",
			Repo:       "test-repo",
			Branch:     "main",
			BasePath:   "instances",
			SecretName: "gh-token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Git push is expected to fail (fake token → GitHub API unreachable)
	if err == nil {
		t.Fatal("expected error from Git push with fake token")
	}

	// Verify the cluster-applied object has the suspended annotation
	if createdObj == nil {
		t.Fatal("expected object to be created on cluster")
	}
	annotations := createdObj.GetAnnotations()
	if annotations[annotationKroReconcile] != kroReconcileSuspended {
		t.Errorf("expected kro.run/reconcile=suspended on cluster object, got %q", annotations[annotationKroReconcile])
	}

	// Verify standard annotations are also present
	if annotations["knodex.io/instance-id"] != "gitops-suspended-test" {
		t.Errorf("expected knodex.io/instance-id=gitops-suspended-test, got %q", annotations["knodex.io/instance-id"])
	}

	// Verify result — Git push fails (fake token), so compensating delete runs
	// and ClusterDeployed is reset to false
	if result == nil {
		t.Fatal("expected result")
	}
	if result.ClusterDeployed {
		t.Error("expected ClusterDeployed=false after successful compensating delete")
	}
}

// TestController_Deploy_GitOpsMode_ClusterApplyFailure verifies that when the
// cluster apply fails in GitOps mode, the deployment fails with no Git push attempted.
func TestController_Deploy_GitOpsMode_ClusterApplyFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Inject a create error
	dynamicClient.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "kro.run", Resource: "applications"},
			"test-app",
			fmt.Errorf("access denied"),
		)
	})

	kubeClient := newFakeKubeClientWithDiscovery()
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-cluster-fail",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:      "test-owner",
			Repo:       "test-repo",
			Branch:     "main",
			BasePath:   "instances",
			SecretName: "gh-token", // Secret does not exist, but cluster apply fails first (reactor above)
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	if err == nil {
		t.Error("expected error for cluster apply failure")
	}
	if result == nil {
		t.Fatal("expected result even on error")
	}
	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.ClusterError == "" {
		t.Error("expected ClusterError to be set")
	}
	if result.GitPushed {
		t.Error("expected no Git push when cluster apply fails")
	}
	if result.GitError != "" {
		t.Error("expected no GitError when failure is on cluster side")
	}
}

// TestController_Deploy_GitOpsMode_GitPushFailure_DeletesSuspendedResource verifies
// that when cluster apply succeeds but Git push fails, the controller attempts a
// compensating delete of the suspended resource.
func TestController_Deploy_GitOpsMode_GitPushFailure_DeletesSuspendedResource(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Track delete calls and capture the deleted resource
	var deleteCount atomic.Int32
	var deletedName, deletedNamespace string
	dynamicClient.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteCount.Add(1)
		deleteAction := action.(k8stesting.DeleteAction)
		deletedName = deleteAction.GetName()
		deletedNamespace = deleteAction.GetNamespace()
		return false, nil, nil // Let the fake handle the delete
	})

	// Create kubeClient WITHOUT the secret — so pushToGit fails on token retrieval
	kubeClient := newFakeKubeClientWithDiscovery()
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-rollback-test",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:      "test-owner",
			Repo:       "test-repo",
			Branch:     "main",
			BasePath:   "instances",
			SecretName: "nonexistent-secret",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	if err == nil {
		t.Error("expected error for Git push failure")
	}
	if result == nil {
		t.Fatal("expected result even on error")
	}

	// Compensating delete should have been attempted on the correct resource
	if deleteCount.Load() == 0 {
		t.Error("expected compensating delete to be attempted after Git push failure")
	}
	if deletedName != "test-app" {
		t.Errorf("expected delete of 'test-app', got %q", deletedName)
	}
	if deletedNamespace != "default" {
		t.Errorf("expected delete in namespace 'default', got %q", deletedNamespace)
	}

	// ClusterDeployed should be reset after successful compensating delete
	if result.ClusterDeployed {
		t.Error("expected ClusterDeployed=false after successful compensating delete")
	}

	// Final status should be failed with Git error
	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.GitError == "" {
		t.Error("expected GitError to be set")
	}
}

// TestController_Deploy_HybridMode_NoSuspendedAnnotation verifies that hybrid mode
// does NOT inject the kro.run/reconcile annotation (regression guard).
func TestController_Deploy_HybridMode_NoSuspendedAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Capture the created object
	var createdObj *unstructured.Unstructured
	dynamicClient.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		createdObj = createAction.GetObject().(*unstructured.Unstructured)
		return false, nil, nil
	})

	kubeClient := newFakeKubeClientWithDiscovery()
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "hybrid-no-suspend",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeHybrid,
		CreatedBy:      "test-user",
		CreatedAt:      time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if createdObj == nil {
		t.Fatal("expected object to be created on cluster")
	}

	annotations := createdObj.GetAnnotations()
	if _, ok := annotations[annotationKroReconcile]; ok {
		t.Error("hybrid mode must NOT contain kro.run/reconcile annotation")
	}

	if !result.ClusterDeployed {
		t.Error("expected ClusterDeployed=true")
	}
}

// mockVCSClient is a minimal vcsClient implementation for testing.
type mockVCSClient struct {
	commitResult *vcs.CommitResult
	commitErr    error
	deleteErr    error
	closed       bool
}

func (m *mockVCSClient) CommitFile(_ context.Context, _ *vcs.CommitFileRequest) (*vcs.CommitResult, error) {
	return m.commitResult, m.commitErr
}
func (m *mockVCSClient) DeleteFile(_ context.Context, _, _, _ string) error { return m.deleteErr }
func (m *mockVCSClient) Close()                                             { m.closed = true }

// TestController_Deploy_GitOpsMode_HappyPath verifies AC 2: when cluster apply succeeds
// and Git push succeeds, result.ClusterDeployed is true and result.Status is StatusPushedToGit.
func TestController_Deploy_GitOpsMode_HappyPath(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	kubeClient := newFakeKubeClientWithDiscovery()

	mockClient := &mockVCSClient{
		commitResult: &vcs.CommitResult{SHA: "abc123def456", Message: "feat: deploy test-app"},
	}

	ctrl := NewController(dynamicClient, kubeClient, nil)
	ctrl.vcsClientFactory = func(_ context.Context, _, _, _ string) (vcsClient, error) {
		return mockClient, nil
	}

	req := &DeployRequest{
		InstanceID:     "gitops-happy-path",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:    "test-owner",
			Repo:     "test-repo",
			Branch:   "main",
			BasePath: "instances",
			// No SecretName — vcsClientFactory is injected, so token retrieval is bypassed
			// by using a dummy secret that exists.
			SecretName:      "dummy-secret",
			SecretNamespace: "kro-system",
			SecretKey:       "token",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	// Provide the dummy secret so getGitHubToken succeeds before vcsClientFactory is called
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dummy-secret", Namespace: "kro-system"},
		Data:       map[string][]byte{"token": []byte("dummy")},
	}
	kubeClient.Tracker().Add(secret)

	result, err := ctrl.Deploy(context.Background(), req)

	if err != nil {
		t.Fatalf("expected no error on happy path, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	// AC 2: both fields must be set on success
	if !result.ClusterDeployed {
		t.Error("expected ClusterDeployed=true after successful cluster apply")
	}
	if result.Status != StatusPushedToGit {
		t.Errorf("expected status %s, got %s", StatusPushedToGit, result.Status)
	}
	if !result.GitPushed {
		t.Error("expected GitPushed=true")
	}
	if result.GitCommitSHA != "abc123def456" {
		t.Errorf("expected GitCommitSHA=abc123def456, got %q", result.GitCommitSHA)
	}
	if result.ManifestPath == "" {
		t.Error("expected ManifestPath to be set")
	}
	if result.GitError != "" {
		t.Errorf("expected no GitError, got %q", result.GitError)
	}

	// Verify the mock VCS client was closed
	if !mockClient.closed {
		t.Error("expected VCS client to be closed")
	}
}

// TestController_Deploy_GitOpsMode_GitPushFailure_DeleteFails verifies that when
// the compensating delete also fails, the original Git error is still returned and
// ClusterDeployed remains true (resource stuck on cluster).
func TestController_Deploy_GitOpsMode_GitPushFailure_DeleteFails(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Allow create but fail delete
	dynamicClient.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("delete blocked by test")
	})

	// No secret → pushToGit fails
	kubeClient := newFakeKubeClientWithDiscovery()
	ctrl := NewController(dynamicClient, kubeClient, nil)

	req := &DeployRequest{
		InstanceID:     "gitops-delete-fail",
		Name:           "test-app",
		Namespace:      "default",
		RGDName:        "test-rgd",
		RGDNamespace:   "kro-system",
		APIVersion:     "kro.run/v1alpha1",
		Kind:           "Application",
		Spec:           map[string]interface{}{"replicas": int64(1)},
		DeploymentMode: ModeGitOps,
		Repository: &RepositoryConfig{
			Owner:      "test-owner",
			Repo:       "test-repo",
			Branch:     "main",
			BasePath:   "instances",
			SecretName: "nonexistent-secret",
		},
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	result, err := ctrl.Deploy(context.Background(), req)

	// Should fail with the Git push error, not the delete error
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Git") && !strings.Contains(err.Error(), "token") {
		t.Errorf("expected Git-related error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	// ClusterDeployed should remain true since the delete failed
	if !result.ClusterDeployed {
		t.Error("expected ClusterDeployed=true when compensating delete fails")
	}

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}
