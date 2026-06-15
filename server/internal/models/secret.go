// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

// Secret-specific labels and annotations stamped on corev1.Secret objects
// managed by Knodex. These are the public, user-editable metadata fields
// exposed through the Secrets API (separate from the system labels
// ProjectLabel + ManagedByLabel which identify ownership).
const (
	// SecretRotationLabel records the rotation policy for a secret.
	// Value is one of "manual" or "auto". A label (not an annotation)
	// so operators can select on it via `kubectl get secrets -l
	// knodex.io/rotation=auto`.
	SecretRotationLabel = "knodex.io/rotation"

	// SecretDocsURLAnnotation points at human documentation for this
	// secret (where it comes from, who owns it, how to rotate it).
	// An annotation rather than a label because URL values contain
	// characters illegal in label values (slashes, colons).
	SecretDocsURLAnnotation = "knodex.io/docs-url"

	// SecretExpiresAtAnnotation records the expiration timestamp for
	// this secret in RFC3339. An annotation rather than a label because
	// RFC3339 contains colons (illegal in label values).
	SecretExpiresAtAnnotation = "knodex.io/expires-at"
)

// SecretRotation is the rotation policy enum stored in SecretRotationLabel.
type SecretRotation string

const (
	// SecretRotationManual indicates the secret is rotated by hand.
	SecretRotationManual SecretRotation = "manual"
	// SecretRotationAuto indicates the secret is rotated by an external
	// automation (Knodex does not perform the rotation itself today —
	// this is metadata that documents intent).
	SecretRotationAuto SecretRotation = "auto"
)

// IsValidSecretRotation reports whether v is "", "manual", or "auto".
// Empty is valid because rotation is an optional field.
func IsValidSecretRotation(v SecretRotation) bool {
	return v == "" || v == SecretRotationManual || v == SecretRotationAuto
}
