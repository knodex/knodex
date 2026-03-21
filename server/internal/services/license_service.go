// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package services provides business logic services following clean architecture principles.
package services

import "time"

// Feature key constants for enterprise features.
// These keys must match the feature strings in the license JWT.
const (
	// FeatureCompliance is the feature key for OPA Gatekeeper compliance.
	FeatureCompliance = "compliance"

	// FeatureViews is the feature key for custom category views.
	FeatureViews = "views"

	// FeatureSecrets is the feature key for secrets management.
	FeatureSecrets = "secrets"
)

// LicenseInfo contains the parsed license data from the JWT claims.
type LicenseInfo struct {
	// LicenseID is the unique license identifier
	LicenseID string `json:"licenseId"`

	// Customer is the licensed customer name (JWT sub claim)
	Customer string `json:"customer"`

	// Edition is the license edition (e.g., "enterprise")
	Edition string `json:"edition"`

	// Features is the list of enabled feature keys
	Features []string `json:"features"`

	// MaxUsers is the maximum number of active users allowed
	MaxUsers int `json:"maxUsers"`

	// IssuedAt is when the license was issued
	IssuedAt time.Time `json:"issuedAt"`

	// ExpiresAt is when the license expires
	ExpiresAt time.Time `json:"expiresAt"`
}

// LicenseStatus represents the current license state for API responses.
type LicenseStatus struct {
	// Licensed indicates if a valid license is present
	Licensed bool `json:"licensed"`

	// Enterprise indicates if this is an enterprise build
	Enterprise bool `json:"enterprise"`

	// Status is the license state: "valid", "expired", "grace_period", "missing", "invalid"
	Status string `json:"status"`

	// Message provides a human-readable status explanation
	Message string `json:"message"`

	// License contains the license details (nil if unlicensed)
	License *LicenseInfo `json:"license,omitempty"`

	// GracePeriodEnd is set when the license is in grace period (7 days after expiry)
	GracePeriodEnd *time.Time `json:"gracePeriodEnd,omitempty"`
}

// LicenseService defines the interface for enterprise license validation.
// In OSS builds, this returns a NoopLicenseService (always false).
// In EE builds, this validates the JWT license and gates features.
type LicenseService interface {
	// IsLicensed returns true if a valid (non-expired or in grace period) license is present.
	IsLicensed() bool

	// IsFeatureEnabled returns true if the given feature key is enabled in the license.
	// Returns false if unlicensed or feature not in the license's features list.
	IsFeatureEnabled(feature string) bool

	// GetLicense returns the parsed license info, or nil if unlicensed.
	GetLicense() *LicenseInfo

	// GetStatus returns the full license status for API responses.
	GetStatus() *LicenseStatus

	// IsGracePeriod returns true if the license has expired but is within the 7-day grace period.
	IsGracePeriod() bool

	// IsReadOnly returns true when the license has expired past the grace period
	// but was previously valid. In this state, read operations (GET) should succeed
	// but write operations (POST/PATCH/PUT/DELETE) should return 402.
	IsReadOnly() bool

	// HasFeature returns true if the given feature key exists in the license claims,
	// regardless of expiry status. Used for read-only access after grace period ends.
	HasFeature(feature string) bool

	// UpdateLicense validates and applies a new license JWT at runtime.
	// Returns an error if the token is invalid.
	UpdateLicense(tokenString string) error
}

// NoopLicenseService is a no-op implementation of LicenseService for OSS builds.
// All methods indicate the feature is not licensed.
type NoopLicenseService struct{}

// IsLicensed returns false as this is an OSS build.
func (s *NoopLicenseService) IsLicensed() bool {
	return false
}

// IsFeatureEnabled returns false as this is an OSS build.
func (s *NoopLicenseService) IsFeatureEnabled(_ string) bool {
	return false
}

// GetLicense returns nil as this is an OSS build.
func (s *NoopLicenseService) GetLicense() *LicenseInfo {
	return nil
}

// GetStatus returns status indicating this is an OSS build.
func (s *NoopLicenseService) GetStatus() *LicenseStatus {
	return &LicenseStatus{
		Licensed:   false,
		Enterprise: false,
		Status:     "missing",
		Message:    "Enterprise license not available in OSS build",
	}
}

// IsGracePeriod returns false as this is an OSS build.
func (s *NoopLicenseService) IsGracePeriod() bool {
	return false
}

// IsReadOnly returns false as this is an OSS build.
func (s *NoopLicenseService) IsReadOnly() bool {
	return false
}

// HasFeature returns false as this is an OSS build.
func (s *NoopLicenseService) HasFeature(_ string) bool {
	return false
}

// UpdateLicense returns an error as license management is not available in OSS builds.
func (s *NoopLicenseService) UpdateLicense(_ string) error {
	return ErrServiceUnavailable
}

// Ensure NoopLicenseService implements LicenseService.
var _ LicenseService = (*NoopLicenseService)(nil)
