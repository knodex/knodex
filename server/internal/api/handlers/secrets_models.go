// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import "time"

// CreateSecretRequest represents the request body for creating a secret
type CreateSecretRequest struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
}

// SecretResponse represents a secret in API responses (never includes values)
type SecretResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Keys      []string          `json:"keys"`
	CreatedAt time.Time         `json:"createdAt"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// SecretDetailResponse represents a secret with its values (only used by GetSecret)
type SecretDetailResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time         `json:"createdAt"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// UpdateSecretRequest represents the request body for updating a secret
type UpdateSecretRequest struct {
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
}

// DeleteSecretResponse represents the response for deleting a secret
type DeleteSecretResponse struct {
	Deleted  bool     `json:"deleted"`
	Warnings []string `json:"warnings"`
}

// SecretListResponse represents a list of secrets
type SecretListResponse struct {
	Items      []SecretResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
	Continue   string           `json:"continue,omitempty"`
	HasMore    bool             `json:"hasMore"`
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
)
