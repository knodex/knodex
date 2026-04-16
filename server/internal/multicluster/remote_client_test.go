// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package multicluster

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/capi"
)

func TestBuildRemoteDynamicClient_SecretNotFound(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := BuildRemoteDynamicClient(ctx, k8sClient, "nonexistent-cluster")
	if err == nil {
		t.Fatal("expected error when kubeconfig secret not found")
	}
	if got := err.Error(); !strings.Contains(got, "not found in any namespace") {
		t.Errorf("expected 'not found' error, got: %s", got)
	}
}

func TestBuildRemoteDynamicClient_MissingValueKey(t *testing.T) {
	secretName := capi.KubeconfigSecretName("test-cluster")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"wrong-key": []byte("data"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	ctx := context.Background()

	_, err := BuildRemoteDynamicClient(ctx, k8sClient, "test-cluster")
	if err == nil {
		t.Fatal("expected error when 'value' key is missing")
	}
	if got := err.Error(); !strings.Contains(got, "missing 'value' key") {
		t.Errorf("expected 'missing value key' error, got: %s", got)
	}
}

func TestBuildRemoteDynamicClient_InvalidKubeconfig(t *testing.T) {
	secretName := capi.KubeconfigSecretName("test-cluster")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("not-a-valid-kubeconfig"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	ctx := context.Background()

	_, err := BuildRemoteDynamicClient(ctx, k8sClient, "test-cluster")
	if err == nil {
		t.Fatal("expected error when kubeconfig is invalid")
	}
}

func TestBuildRemoteDynamicClient_ValidKubeconfig(t *testing.T) {
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	secretName := capi.KubeconfigSecretName("test-cluster")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "capi-system",
		},
		Data: map[string][]byte{
			"value": []byte(kubeconfig),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	ctx := context.Background()

	client, err := BuildRemoteDynamicClient(ctx, k8sClient, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil dynamic client")
	}
}
