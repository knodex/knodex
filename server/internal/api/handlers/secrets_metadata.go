// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/knodex/knodex/server/internal/models"
)

// validateSecretMetadata checks the typed metadata fields and merges any
// problems into errors (so the caller can collect every issue in one
// round-trip). Returns the (possibly mutated) errors map. A nil md is a
// no-op — the caller decides whether absent metadata is allowed.
func validateSecretMetadata(md *SecretMetadata, errors map[string]string) map[string]string {
	if md == nil {
		return errors
	}
	if !models.IsValidSecretRotation(md.Rotation) {
		errors["metadata:rotation"] = fmt.Sprintf("rotation must be %q or %q",
			models.SecretRotationManual, models.SecretRotationAuto)
	}
	if md.DocsURL != "" {
		if len(md.DocsURL) > MaxSecretDocsURLLength {
			errors["metadata:docsUrl"] = fmt.Sprintf("docsUrl exceeds maximum length of %d characters", MaxSecretDocsURLLength)
		} else {
			u, err := url.Parse(md.DocsURL)
			if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				errors["metadata:docsUrl"] = "docsUrl must be an http or https URL"
			}
		}
	}
	// ExpiresAt: nothing to validate — *time.Time already gates parseability
	// at the JSON layer, and past timestamps are legal (status will report
	// them as expired).
	return errors
}

// applyMetadataToSecret writes the typed metadata onto a corev1.Secret's
// Labels and Annotations. nil md is a no-op (caller chose to leave
// existing metadata untouched). A non-nil md is treated as a full
// replacement of the three metadata fields — empty values clear the
// corresponding label/annotation rather than leaving stale state.
//
// System labels (ProjectLabel, ManagedByLabel) and the updatedAt
// annotation are NOT touched by this function — the caller is
// responsible for those.
func applyMetadataToSecret(secret *corev1.Secret, md *SecretMetadata) {
	if md == nil {
		return
	}

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}

	if md.Rotation == "" {
		delete(secret.Labels, models.SecretRotationLabel)
	} else {
		secret.Labels[models.SecretRotationLabel] = string(md.Rotation)
	}

	if md.DocsURL == "" {
		delete(secret.Annotations, models.SecretDocsURLAnnotation)
	} else {
		secret.Annotations[models.SecretDocsURLAnnotation] = md.DocsURL
	}

	if md.ExpiresAt == nil {
		delete(secret.Annotations, models.SecretExpiresAtAnnotation)
	} else {
		secret.Annotations[models.SecretExpiresAtAnnotation] = md.ExpiresAt.UTC().Format(time.RFC3339)
	}
}

// extractSecretMetadata reads the typed metadata back from a Secret's
// labels/annotations. Returns nil when none of the three fields are set
// so JSON responses can omit the field entirely.
//
// Malformed values (an unknown rotation enum or an unparsable
// expires-at) are logged and skipped — the rest of the metadata is
// still returned so a single bad annotation doesn't black out the row.
func extractSecretMetadata(secret *corev1.Secret) *SecretMetadata {
	var (
		rotation  models.SecretRotation
		docsURL   string
		expiresAt *time.Time
	)

	if v, ok := secret.Labels[models.SecretRotationLabel]; ok && v != "" {
		r := models.SecretRotation(v)
		if models.IsValidSecretRotation(r) && r != "" {
			rotation = r
		} else {
			slog.Warn("ignoring unknown rotation label value",
				"label", models.SecretRotationLabel,
				"value", v,
				"secret", secret.Namespace+"/"+secret.Name,
			)
		}
	}

	if v, ok := secret.Annotations[models.SecretDocsURLAnnotation]; ok {
		docsURL = v
	}

	if v, ok := secret.Annotations[models.SecretExpiresAtAnnotation]; ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			slog.Warn("ignoring malformed expires-at annotation",
				"annotation", models.SecretExpiresAtAnnotation,
				"value", v,
				"secret", secret.Namespace+"/"+secret.Name,
				"error", err,
			)
		} else {
			expiresAt = &t
		}
	}

	if rotation == "" && docsURL == "" && expiresAt == nil {
		return nil
	}
	return &SecretMetadata{
		Rotation:  rotation,
		DocsURL:   docsURL,
		ExpiresAt: expiresAt,
	}
}

// computeSecretStatus reports the expiry state given the typed metadata
// and the reference "now". Pure function so tests don't need a clock.
//
//   - nil metadata or nil ExpiresAt → ""           (no expiration set)
//   - ExpiresAt at-or-before now    → "expired"
//   - ExpiresAt within 30 days      → "expiring-soon"
//   - otherwise                     → "active"
func computeSecretStatus(md *SecretMetadata, now time.Time) SecretStatus {
	if md == nil || md.ExpiresAt == nil {
		return ""
	}
	expiry := *md.ExpiresAt
	if !expiry.After(now) {
		return SecretStatusExpired
	}
	if expiry.Sub(now) <= expiringSoonWindow {
		return SecretStatusExpiringSoon
	}
	return SecretStatusActive
}
