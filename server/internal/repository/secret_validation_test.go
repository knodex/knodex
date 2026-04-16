// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretToRepositoryConfig_ValidFields(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo-secret",
			Namespace: "knodex",
		},
		Data: map[string][]byte{
			SecretKeyURL:     []byte("https://github.com/org/repo.git"),
			SecretKeyType:    []byte("token"),
			SecretKeyProject: []byte("proj-1"),
		},
	}

	result := secretToRepositoryConfig(secret)
	if result == nil {
		t.Fatal("expected non-nil result for valid secret")
	}
	if result.Spec.RepoURL != "https://github.com/org/repo.git" {
		t.Errorf("unexpected RepoURL: %s", result.Spec.RepoURL)
	}
	if result.Spec.AuthType != "token" {
		t.Errorf("unexpected AuthType: %s", result.Spec.AuthType)
	}
}

func TestSecretToRepositoryConfig_MissingRepoURL(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-secret", Namespace: "knodex"},
		Data: map[string][]byte{
			SecretKeyType: []byte("token"),
			// SecretKeyURL intentionally missing
		},
	}

	result := secretToRepositoryConfig(secret)
	if result != nil {
		t.Errorf("expected nil for secret missing RepoURL, got non-nil with RepoURL=%q", result.Spec.RepoURL)
	}
}

func TestSecretToRepositoryConfig_MissingAuthType(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-secret", Namespace: "knodex"},
		Data: map[string][]byte{
			SecretKeyURL: []byte("https://github.com/org/repo.git"),
			// SecretKeyType intentionally missing
		},
	}

	result := secretToRepositoryConfig(secret)
	if result != nil {
		t.Errorf("expected nil for secret missing AuthType, got non-nil with AuthType=%q", result.Spec.AuthType)
	}
}

func TestSecretToRepositoryConfig_EmptyData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-secret", Namespace: "knodex"},
		Data:       map[string][]byte{},
	}

	result := secretToRepositoryConfig(secret)
	if result != nil {
		t.Error("expected nil for secret with empty data")
	}
}
