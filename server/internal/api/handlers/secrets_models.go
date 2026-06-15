// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"time"

	"github.com/knodex/knodex/server/internal/models"
)

// SecretMetadata is the typed metadata surface exposed by the API for a
// secret — rotation policy, documentation URL, and expiration timestamp.
// On the wire these arrive flat (rotation/docsUrl/expiresAt); on the
// underlying corev1.Secret they are stored as labels (Rotation) and
// annotations (DocsURL, ExpiresAt). See server/internal/models/secret.go
// for the actual label/annotation keys.
type SecretMetadata struct {
	// Rotation is the secret rotation policy: "manual" or "auto".
	// Empty means the operator has not declared a policy.
	Rotation models.SecretRotation `json:"rotation,omitempty"`
	// DocsURL is a link to human documentation for this secret
	// (provenance, ownership, rotation runbook). Must be an http or
	// https URL when set.
	DocsURL string `json:"docsUrl,omitempty"`
	// ExpiresAt is when the secret is considered expired (RFC3339).
	// The frontend models expiration as a day; the constructed value
	// is end-of-day UTC, but the server stores whatever timestamp the
	// client sends and lets the status helper interpret it.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// SecretStatus is the computed expiry state surfaced in API responses.
// Empty string means "no expiration date is set" (the UI renders an
// em-dash for that case).
type SecretStatus string

const (
	// SecretStatusActive indicates ExpiresAt is in the future and outside
	// the expiring-soon window.
	SecretStatusActive SecretStatus = "active"
	// SecretStatusExpiringSoon indicates ExpiresAt is within 30 days.
	SecretStatusExpiringSoon SecretStatus = "expiring-soon"
	// SecretStatusExpired indicates ExpiresAt is at or before now.
	SecretStatusExpired SecretStatus = "expired"
)

// expiringSoonWindow is the threshold under which a not-yet-expired
// secret is reported as "expiring-soon". Tuned to 30 days because
// rotation procedures typically take a sprint to coordinate.
const expiringSoonWindow = 30 * 24 * time.Hour

// CreateSecretRequest represents the request body for creating a secret.
// The namespace is carried in the URL path (/api/v1/namespaces/{namespace}/secrets),
// not in the body — mirroring how Instances handle their namespace dimension.
type CreateSecretRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
	// Metadata is optional typed metadata (rotation, docs URL, expiry).
	// nil or all-empty leaves the labels/annotations unset.
	Metadata *SecretMetadata `json:"metadata,omitempty"`
}

// SecretResponse represents a secret in API responses (never includes values)
type SecretResponse struct {
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Keys      []string   `json:"keys"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	// Labels are user-visible labels on the Secret with system labels
	// and the Knodex-typed metadata labels (rotation) stripped. Surfaced
	// for completeness — typed metadata is in Metadata.
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata is the typed metadata for this secret. Omitted when no
	// metadata fields are set on the underlying object.
	Metadata *SecretMetadata `json:"metadata,omitempty"`
	// Status is the server-computed expiry state derived from
	// Metadata.ExpiresAt. Empty when no expiration is set.
	Status SecretStatus `json:"status,omitempty"`
}

// SecretDetailResponse represents a secret with its values (only used by GetSecret)
type SecretDetailResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt *time.Time        `json:"updatedAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Metadata  *SecretMetadata   `json:"metadata,omitempty"`
	Status    SecretStatus      `json:"status,omitempty"`
}

// UpdateSecretRequest represents the request body for updating a secret.
// The namespace is carried in the URL path; the body holds only the new
// data and optional metadata. Metadata semantics: nil means "leave existing
// metadata untouched"; a non-nil pointer is a full replacement of the three
// metadata fields (an empty string clears that field).
type UpdateSecretRequest struct {
	Data     map[string]string `json:"data"`
	Metadata *SecretMetadata   `json:"metadata,omitempty"`
}

// DeleteSecretResponse represents the response for deleting a secret
type DeleteSecretResponse struct {
	Deleted  bool     `json:"deleted"`
	Warnings []string `json:"warnings"`
}

// SecretListResponse represents a list of secrets
type SecretListResponse struct {
	Items     []SecretResponse `json:"items"`
	PageCount int              `json:"pageCount"`
	Continue  string           `json:"continue,omitempty"`
	HasMore   bool             `json:"hasMore"`
}

// Pagination defaults and limits for secret list operations.
const (
	defaultSecretPageSize = 100
	maxSecretPageSize     = 500
)

// Size limits for secret data validation.
const (
	// MaxSecretValueSize is the maximum size of a single secret value (256KB).
	MaxSecretValueSize = 256 * 1024
	// MaxSecretTotalSize is the maximum total size of all secret data (512KB).
	MaxSecretTotalSize = 512 * 1024
	// MaxSecretDocsURLLength is the maximum length of the DocsURL field.
	// 2048 mirrors what most browsers accept for URLs without quibbling.
	MaxSecretDocsURLLength = 2048
)
