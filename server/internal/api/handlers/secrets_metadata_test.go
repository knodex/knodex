// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/models"
)

func TestComputeSecretStatus(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	mustPtr := func(t time.Time) *time.Time { return &t }

	tests := []struct {
		name string
		md   *SecretMetadata
		want SecretStatus
	}{
		{"nil metadata", nil, ""},
		{"no expiry", &SecretMetadata{Rotation: models.SecretRotationManual}, ""},
		{"expired by 1 day", &SecretMetadata{ExpiresAt: mustPtr(now.AddDate(0, 0, -1))}, SecretStatusExpired},
		{"expires exactly now", &SecretMetadata{ExpiresAt: mustPtr(now)}, SecretStatusExpired},
		{"expires in 1 second", &SecretMetadata{ExpiresAt: mustPtr(now.Add(time.Second))}, SecretStatusExpiringSoon},
		{"expires in 29 days", &SecretMetadata{ExpiresAt: mustPtr(now.AddDate(0, 0, 29))}, SecretStatusExpiringSoon},
		{"expires exactly at 30-day window", &SecretMetadata{ExpiresAt: mustPtr(now.Add(expiringSoonWindow))}, SecretStatusExpiringSoon},
		{"expires 30 days + 1s past window", &SecretMetadata{ExpiresAt: mustPtr(now.Add(expiringSoonWindow + time.Second))}, SecretStatusActive},
		{"far future", &SecretMetadata{ExpiresAt: mustPtr(now.AddDate(1, 0, 0))}, SecretStatusActive},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeSecretStatus(tc.md, now)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateSecretMetadata(t *testing.T) {
	mustPtr := func(t time.Time) *time.Time { return &t }
	now := time.Now()

	tests := []struct {
		name        string
		md          *SecretMetadata
		wantErrKeys []string
	}{
		{"nil ok", nil, nil},
		{"empty ok", &SecretMetadata{}, nil},
		{"valid manual", &SecretMetadata{Rotation: models.SecretRotationManual}, nil},
		{"valid auto", &SecretMetadata{Rotation: models.SecretRotationAuto}, nil},
		{"valid https url", &SecretMetadata{DocsURL: "https://wiki.example.com/secret"}, nil},
		{"valid http url", &SecretMetadata{DocsURL: "http://internal.local"}, nil},
		{"past expiry is allowed", &SecretMetadata{ExpiresAt: mustPtr(now.AddDate(-1, 0, 0))}, nil},
		{"future expiry is allowed", &SecretMetadata{ExpiresAt: mustPtr(now.AddDate(1, 0, 0))}, nil},
		{"all fields together", &SecretMetadata{
			Rotation: models.SecretRotationAuto, DocsURL: "https://docs.example.com",
			ExpiresAt: mustPtr(now.AddDate(0, 1, 0)),
		}, nil},

		// Rejections
		{"bad rotation", &SecretMetadata{Rotation: "automatic"}, []string{"metadata:rotation"}},
		{"plain string url", &SecretMetadata{DocsURL: "not-a-url"}, []string{"metadata:docsUrl"}},
		{"ftp url rejected", &SecretMetadata{DocsURL: "ftp://files.example.com"}, []string{"metadata:docsUrl"}},
		{"javascript url rejected", &SecretMetadata{DocsURL: "javascript:alert(1)"}, []string{"metadata:docsUrl"}},
		{"url too long", &SecretMetadata{DocsURL: "https://example.com/" + longString(MaxSecretDocsURLLength)}, []string{"metadata:docsUrl"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateSecretMetadata(tc.md, map[string]string{})
			if tc.wantErrKeys == nil {
				assert.Empty(t, errs)
				return
			}
			for _, k := range tc.wantErrKeys {
				assert.Contains(t, errs, k, "missing expected error key %q (got: %v)", k, errs)
			}
		})
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func TestApplyMetadataToSecret_WritesAndClears(t *testing.T) {
	expiry := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	mustPtr := func(t time.Time) *time.Time { return &t }

	t.Run("writes all three fields to fresh secret", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{models.ProjectLabel: "demo", models.ManagedByLabel: models.ManagedByValue},
		}}
		applyMetadataToSecret(s, &SecretMetadata{
			Rotation:  models.SecretRotationAuto,
			DocsURL:   "https://wiki.example.com/s",
			ExpiresAt: mustPtr(expiry),
		})

		assert.Equal(t, "auto", s.Labels[models.SecretRotationLabel])
		assert.Equal(t, "https://wiki.example.com/s", s.Annotations[models.SecretDocsURLAnnotation])
		assert.Equal(t, "2026-12-31T23:59:59Z", s.Annotations[models.SecretExpiresAtAnnotation])
		// System labels preserved
		assert.Equal(t, "demo", s.Labels[models.ProjectLabel])
		assert.Equal(t, models.ManagedByValue, s.Labels[models.ManagedByLabel])
	})

	t.Run("nil metadata is a no-op", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{models.ProjectLabel: "demo", models.SecretRotationLabel: "manual"},
			Annotations: map[string]string{models.SecretDocsURLAnnotation: "https://existing"},
		}}
		applyMetadataToSecret(s, nil)

		assert.Equal(t, "demo", s.Labels[models.ProjectLabel])
		assert.Equal(t, "manual", s.Labels[models.SecretRotationLabel])
		assert.Equal(t, "https://existing", s.Annotations[models.SecretDocsURLAnnotation])
	})

	t.Run("empty fields clear existing metadata but preserve system labels", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				models.ProjectLabel:        "demo",
				models.ManagedByLabel:      models.ManagedByValue,
				models.SecretRotationLabel: "manual",
			},
			Annotations: map[string]string{
				models.SecretDocsURLAnnotation:   "https://stale",
				models.SecretExpiresAtAnnotation: "2025-01-01T00:00:00Z",
				"knodex.io/updated-at":           "2026-01-01T00:00:00Z", // unrelated
			},
		}}
		// Non-nil metadata with all-empty fields = clear all three
		applyMetadataToSecret(s, &SecretMetadata{})

		assert.NotContains(t, s.Labels, models.SecretRotationLabel)
		assert.NotContains(t, s.Annotations, models.SecretDocsURLAnnotation)
		assert.NotContains(t, s.Annotations, models.SecretExpiresAtAnnotation)

		// System labels + unrelated annotations preserved
		assert.Equal(t, "demo", s.Labels[models.ProjectLabel])
		assert.Equal(t, models.ManagedByValue, s.Labels[models.ManagedByLabel])
		assert.Equal(t, "2026-01-01T00:00:00Z", s.Annotations["knodex.io/updated-at"])
	})

	t.Run("works on secret with nil labels/annotations", func(t *testing.T) {
		s := &corev1.Secret{}
		require.NotPanics(t, func() {
			applyMetadataToSecret(s, &SecretMetadata{Rotation: models.SecretRotationManual})
		})
		assert.Equal(t, "manual", s.Labels[models.SecretRotationLabel])
	})
}

func TestSecretsHandler_CreateWithMetadata_RoundTrip(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	// Pick an expiry well outside the 30-day window so status is "active"
	// regardless of when the test runs.
	expiry := time.Now().AddDate(1, 0, 0).UTC().Truncate(time.Second)
	body := CreateSecretRequest{
		Name: "my-secret",
		Data: map[string]string{"password": "s3cret"},
		Metadata: &SecretMetadata{
			Rotation:  models.SecretRotationAuto,
			DocsURL:   "https://wiki.example.com/secret",
			ExpiresAt: &expiry,
		},
	}

	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, "body=%s", rr.Body.String())

	var createResp SecretResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &createResp))
	require.NotNil(t, createResp.Metadata)
	assert.Equal(t, models.SecretRotationAuto, createResp.Metadata.Rotation)
	assert.Equal(t, "https://wiki.example.com/secret", createResp.Metadata.DocsURL)
	require.NotNil(t, createResp.Metadata.ExpiresAt)
	assert.True(t, createResp.Metadata.ExpiresAt.Equal(expiry), "round-trip expiry mismatch: got %v want %v", createResp.Metadata.ExpiresAt, expiry)
	assert.Equal(t, SecretStatusActive, createResp.Status)

	// Verify the underlying K8s Secret got the actual labels/annotations.
	// AC5: knodex.io/project label is NOT stamped under the new model.
	got, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "auto", got.Labels[models.SecretRotationLabel])
	assert.Equal(t, "https://wiki.example.com/secret", got.Annotations[models.SecretDocsURLAnnotation])
	assert.Equal(t, expiry.Format(time.RFC3339), got.Annotations[models.SecretExpiresAtAnnotation])
	assert.Equal(t, models.ManagedByValue, got.Labels[models.ManagedByLabel])
	_, hasProjectLabel := got.Labels[models.ProjectLabel]
	assert.False(t, hasProjectLabel, "knodex.io/project label MUST NOT be stamped (TD-2)")

	// Round-trip via GET (namespace-keyed URL)
	getReq := makeRequestForNamespace("GET", "default", "my-secret", nil, defaultUserCtx())
	getRR := httptest.NewRecorder()
	handler.GetSecret(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code, "body=%s", getRR.Body.String())

	var detail SecretDetailResponse
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &detail))
	require.NotNil(t, detail.Metadata)
	assert.Equal(t, models.SecretRotationAuto, detail.Metadata.Rotation)
	assert.Equal(t, SecretStatusActive, detail.Status)
}

func TestSecretsHandler_CreateWithBadMetadata_Rejected(t *testing.T) {
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: fake.NewSimpleClientset(),
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	body := CreateSecretRequest{
		Name:     "my-secret",
		Data:     map[string]string{"k": "v"},
		Metadata: &SecretMetadata{Rotation: "weekly"},
	}

	req := makeRequestForNamespace("POST", "default", "", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.CreateSecret(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "metadata:rotation")
}

func TestSecretsHandler_UpdateWithoutMetadata_PreservesExisting(t *testing.T) {
	// Existing secret has rotation=manual + a docs URL already set. The
	// legacy knodex.io/project label is intentionally retained on the
	// fixture to verify it is not modified by the update (it is "inert" in
	// the new model — read path ignores it).
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
			Labels: map[string]string{
				models.ProjectLabel:        "demo", // legacy, ignored by new read path
				models.ManagedByLabel:      models.ManagedByValue,
				models.SecretRotationLabel: "manual",
			},
			Annotations: map[string]string{
				models.SecretDocsURLAnnotation: "https://stays.example.com",
			},
		},
		Data: map[string][]byte{"old": []byte("v")},
	}
	k8sClient := fake.NewSimpleClientset(existing)
	handler := NewSecretsHandler(SecretsHandlerConfig{
		K8sClient: k8sClient,
		NSAccess:  &mockNSAccessProvider{namespaces: []string{"*"}},
	})

	// Update WITHOUT a metadata field — must not blow away existing labels/annotations.
	body := UpdateSecretRequest{
		Data: map[string]string{"new": "v2"},
	}
	req := makeRequestForNamespace("PUT", "default", "my-secret", body, defaultUserCtx())
	rr := httptest.NewRecorder()
	handler.UpdateSecret(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	got, err := k8sClient.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "manual", got.Labels[models.SecretRotationLabel], "rotation label was wiped by nil-metadata update")
	assert.Equal(t, "https://stays.example.com", got.Annotations[models.SecretDocsURLAnnotation], "docs-url annotation was wiped by nil-metadata update")
}

func TestExtractSecretMetadata(t *testing.T) {
	t.Run("returns nil for secret without metadata", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{models.ProjectLabel: "demo"},
		}}
		assert.Nil(t, extractSecretMetadata(s))
	})

	t.Run("reads all three fields", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				models.ProjectLabel:        "demo",
				models.SecretRotationLabel: "auto",
			},
			Annotations: map[string]string{
				models.SecretDocsURLAnnotation:   "https://docs.example.com",
				models.SecretExpiresAtAnnotation: "2026-12-31T23:59:59Z",
			},
		}}
		md := extractSecretMetadata(s)
		require.NotNil(t, md)
		assert.Equal(t, models.SecretRotationAuto, md.Rotation)
		assert.Equal(t, "https://docs.example.com", md.DocsURL)
		require.NotNil(t, md.ExpiresAt)
		assert.Equal(t, 2026, md.ExpiresAt.Year())
		assert.Equal(t, time.December, md.ExpiresAt.Month())
	})

	t.Run("malformed expires-at is skipped, other fields still returned", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{models.SecretRotationLabel: "manual"},
			Annotations: map[string]string{
				models.SecretExpiresAtAnnotation: "not-a-date",
			},
		}}
		md := extractSecretMetadata(s)
		require.NotNil(t, md)
		assert.Equal(t, models.SecretRotationManual, md.Rotation)
		assert.Nil(t, md.ExpiresAt)
	})

	t.Run("unknown rotation label is skipped", func(t *testing.T) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{models.SecretRotationLabel: "weekly"},
			Annotations: map[string]string{
				models.SecretDocsURLAnnotation: "https://docs.example.com",
			},
		}}
		md := extractSecretMetadata(s)
		require.NotNil(t, md)
		assert.Equal(t, models.SecretRotation(""), md.Rotation)
		assert.Equal(t, "https://docs.example.com", md.DocsURL)
	})
}
