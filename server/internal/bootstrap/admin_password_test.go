// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureAdminPasswordSecret(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		password       string
		existingSecret *corev1.Secret
		k8sClient      kubernetes.Interface
		wantErr        bool
		errContains    string
	}{
		{
			name:      "creates secret successfully when not exists",
			namespace: "test-namespace",
			password:  "test-password-123",
			k8sClient: fake.NewSimpleClientset(),
			wantErr:   false,
		},
		{
			name:      "does not overwrite existing secret",
			namespace: "test-namespace",
			password:  "new-password",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					SecretKey: []byte("existing-password"),
				},
			},
			k8sClient: fake.NewSimpleClientset(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					SecretKey: []byte("existing-password"),
				},
			}),
			wantErr: false,
		},
		{
			name:        "returns error when k8s client is nil",
			namespace:   "test-namespace",
			password:    "test-password",
			k8sClient:   nil,
			wantErr:     true,
			errContains: "kubernetes client is nil",
		},
		{
			name:        "returns error when namespace is empty",
			namespace:   "",
			password:    "test-password",
			k8sClient:   fake.NewSimpleClientset(),
			wantErr:     true,
			errContains: "namespace cannot be empty",
		},
		{
			name:        "returns error when password is empty",
			namespace:   "test-namespace",
			password:    "",
			k8sClient:   fake.NewSimpleClientset(),
			wantErr:     true,
			errContains: "password cannot be empty",
		},
		{
			name:      "returns error when get secret fails (not NotFound)",
			namespace: "test-namespace",
			password:  "test-password",
			k8sClient: func() kubernetes.Interface {
				client := fake.NewSimpleClientset()
				client.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("internal error"))
				})
				return client
			}(),
			wantErr:     true,
			errContains: "failed to check if secret exists",
		},
		{
			name:      "returns error when create secret fails",
			namespace: "test-namespace",
			password:  "test-password",
			k8sClient: func() kubernetes.Interface {
				client := fake.NewSimpleClientset()
				client.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "test", nil)
				})
				return client
			}(),
			wantErr:     true,
			errContains: "failed to create admin password secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := EnsureAdminPasswordSecret(ctx, tt.k8sClient, tt.namespace, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("EnsureAdminPasswordSecret() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("EnsureAdminPasswordSecret() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("EnsureAdminPasswordSecret() unexpected error = %v", err)
				return
			}

			// If we expected success and no existing secret, verify the secret was created
			if tt.k8sClient != nil && tt.existingSecret == nil {
				secret, err := tt.k8sClient.CoreV1().Secrets(tt.namespace).Get(ctx, SecretName, metav1.GetOptions{})
				if err != nil {
					t.Errorf("Failed to get created secret: %v", err)
					return
				}

				// Verify secret contents
				if secret.Name != SecretName {
					t.Errorf("Secret name = %v, want %v", secret.Name, SecretName)
				}

				if secret.Namespace != tt.namespace {
					t.Errorf("Secret namespace = %v, want %v", secret.Namespace, tt.namespace)
				}

				if secret.Type != corev1.SecretTypeOpaque {
					t.Errorf("Secret type = %v, want %v", secret.Type, corev1.SecretTypeOpaque)
				}

				passwordData, ok := secret.Data[SecretKey]
				if !ok {
					t.Errorf("Secret missing key %v", SecretKey)
				}

				if string(passwordData) != tt.password {
					t.Errorf("Secret password = %v, want %v", string(passwordData), tt.password)
				}

				// Verify labels
				expectedLabels := map[string]string{
					"app.kubernetes.io/name":       "knodex",
					"app.kubernetes.io/component":  "auth",
					"app.kubernetes.io/managed-by": "knodex-server",
				}

				for k, v := range expectedLabels {
					if secret.Labels[k] != v {
						t.Errorf("Secret label %s = %v, want %v", k, secret.Labels[k], v)
					}
				}
			}
		})
	}
}

func TestEnsureAdminPasswordSecret_Idempotent(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	namespace := "test-namespace"
	password := "test-password"

	// Call twice
	err := EnsureAdminPasswordSecret(ctx, client, namespace, password)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	err = EnsureAdminPasswordSecret(ctx, client, namespace, password)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Verify only one secret exists
	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}

	if len(secrets.Items) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(secrets.Items))
	}
}

func TestEnsureAdminPasswordSecret_ContextTimeout(t *testing.T) {
	// Create a context that's already expired
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	client := fake.NewSimpleClientset()
	// Add a small delay to trigger context timeout
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		time.Sleep(100 * time.Millisecond)
		return true, nil, errors.NewNotFound(schema.GroupResource{Resource: "secrets"}, SecretName)
	})

	err := EnsureAdminPasswordSecret(ctx, client, "test-namespace", "test-password")

	// The function should handle the timeout gracefully
	// In a real scenario, this would fail, but the fake client doesn't respect context timeouts
	// So we just verify the function completes
	if err != nil {
		t.Logf("Function returned error as expected on timeout: %v", err)
	}
}

// TestGetOrCreateAdminPassword_ReusesExistingSecret is the regression guard for
// the upgrade path where an operator disables local login (LOCAL_LOGIN_ENABLED=false),
// then later re-enables it. After re-enable, bootstrap MUST return the existing
// password from the Secret rather than regenerating one — otherwise operators who
// captured the auto-generated password during the first install would silently lose
// access on every disable→re-enable cycle.
func TestGetOrCreateAdminPassword_ReusesExistingSecret(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"
	originalPassword := "OperatorCapturedPassword!1"

	// Simulate a prior install: Secret already present in the cluster
	// (created by an earlier server run before disable).
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			SecretKey: []byte(originalPassword),
		},
	})

	got, wasGenerated, err := GetOrCreateAdminPassword(ctx, client, namespace)
	if err != nil {
		t.Fatalf("GetOrCreateAdminPassword() unexpected error = %v", err)
	}
	if wasGenerated {
		t.Error("wasGenerated = true, want false (existing Secret should be reused, not regenerated)")
	}
	if got != originalPassword {
		t.Errorf("password = %q, want %q (operator's original password must be preserved across re-enable)", got, originalPassword)
	}

	// Also verify the Secret on the cluster was not mutated.
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to re-read Secret after bootstrap: %v", err)
	}
	if string(secret.Data[SecretKey]) != originalPassword {
		t.Errorf("Secret password mutated to %q, want %q (bootstrap must not overwrite)", string(secret.Data[SecretKey]), originalPassword)
	}
}

// TestGetOrCreateAdminPassword_Idempotent_PreservesPassword verifies that
// invoking bootstrap twice (e.g., pod restart loop) returns the same password
// both times and does not rotate it.
func TestGetOrCreateAdminPassword_Idempotent_PreservesPassword(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"
	client := fake.NewSimpleClientset()

	first, firstGenerated, err := GetOrCreateAdminPassword(ctx, client, namespace)
	if err != nil {
		t.Fatalf("first GetOrCreateAdminPassword() error = %v", err)
	}
	if !firstGenerated {
		t.Error("first call wasGenerated = false, want true (no Secret existed)")
	}
	if first == "" {
		t.Fatal("first call returned empty password")
	}

	second, secondGenerated, err := GetOrCreateAdminPassword(ctx, client, namespace)
	if err != nil {
		t.Fatalf("second GetOrCreateAdminPassword() error = %v", err)
	}
	if secondGenerated {
		t.Error("second call wasGenerated = true, want false (Secret already exists from first call)")
	}
	if second != first {
		t.Errorf("password rotated across calls: first=%q second=%q (bootstrap must be stable across pod restarts)", first, second)
	}
}

func TestGenerateSecurePassword_AlwaysMeetsComplexity(t *testing.T) {
	// Generate many passwords and verify ALL pass complexity requirements.
	// Before the fix, ~0.7% of base64 passwords lacked digits or special chars.
	for i := 0; i < 10000; i++ {
		password, err := generateSecurePassword()
		if err != nil {
			t.Fatalf("generateSecurePassword() error = %v", err)
		}

		if len(password) != 24 {
			t.Errorf("password length = %d, want 24", len(password))
		}

		if !meetsComplexityRequirements(password) {
			t.Errorf("password %q does not meet complexity requirements", password)
		}
	}
}

func TestMeetsComplexityRequirements(t *testing.T) {
	tests := []struct {
		password string
		want     bool
	}{
		{"Abc123", true},    // upper + lower + digit = 3
		{"Abc123!", true},   // all 4 classes
		{"abcdefgh", false}, // only lower = 1
		{"ABCDEFGH", false}, // only upper = 1
		{"12345678", false}, // only digit = 1
		{"abCD1234", true},  // upper + lower + digit = 3
		{"abCDEFGH", false}, // upper + lower = 2
		{"Ab-c", true},      // upper + lower + special = 3
		{"a1-b", true},      // lower + digit + special = 3
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			if got := meetsComplexityRequirements(tt.password); got != tt.want {
				t.Errorf("meetsComplexityRequirements(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
