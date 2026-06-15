// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package services provides business logic services following clean architecture principles.
package services

import (
	"context"
	"time"
)

// Feature key constants for enterprise features.
// These keys must match the feature strings in the license JWT.
const (
	// FeatureCompliance is the feature key for OPA Gatekeeper compliance.
	FeatureCompliance = "compliance"

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

	// Seats reports the current active-user count vs MaxUsers and the warn/exceeded
	// threshold (STORY-465). Omitted on OSS builds. Always present on EE builds — the
	// cold-start sentinel (Used=0, LastUpdated="") signals "calculating…" to the UI.
	Seats *SeatUsage `json:"seats,omitempty"`
}

// SeatUsage reports active-user seat usage for the current organization,
// compared against the license.MaxUsers claim.
//
// Story 15.2 (R5-2): "Used" is now the entitlement count — distinct users with
// state='active' in the canonical identity roster — fed by
// IdentityService.BilledSeatCount. WindowDays is retained for UI compatibility
// (the "inactive" badge, owned by Story 15.2a) but no longer drives billing.
type SeatUsage struct {
	// Used is the count of active (entitled) users.
	Used int64 `json:"used"`
	// Allowed mirrors LicenseInfo.MaxUsers (0 means unlimited).
	Allowed int `json:"allowed"`
	// WindowDays is the trailing-window length (currently hard-coded to 30).
	WindowDays int `json:"windowDays"`
	// Percent = Used / Allowed (0.0 when Allowed == 0).
	Percent float64 `json:"percent"`
	// Threshold: "ok" | "warn" | "exceeded".
	// - "ok"        Allowed == 0 OR Percent < 0.80
	// - "warn"      0.80 <= Percent < 1.0
	// - "exceeded"  Percent >= 1.0
	Threshold string `json:"threshold"`
	// LastUpdated is the RFC3339 timestamp of the most recent reconciliation, or
	// "" when the reconciler has never successfully run (cold start). The UI
	// renders "Active users: calculating…" in that case.
	LastUpdated string `json:"lastUpdated"`
	// AdvisoryOnly is true when an external system is the authoritative seat
	// enforcer (KNODEX_SEAT_ADVISORY_ONLY); the UI renders softer copy then.
	AdvisoryOnly bool `json:"advisoryOnly"`
}

// Seat threshold constants (STORY-465 AC #8).
const (
	// SeatThresholdOK indicates Allowed=0 (unlimited) OR Percent < 0.80.
	SeatThresholdOK = "ok"
	// SeatThresholdWarn indicates 0.80 <= Percent < 1.0 — render the amber warning banner.
	SeatThresholdWarn = "warn"
	// SeatThresholdExceeded indicates Percent >= 1.0 — render the destructive banner.
	SeatThresholdExceeded = "exceeded"

	// SeatWarnPercent is the lower bound for the "warn" band (80%).
	SeatWarnPercent = 0.80
	// SeatExceededPercent is the lower bound for the "exceeded" band (100%).
	SeatExceededPercent = 1.0

	// DefaultSeatWindowDays is the trailing window for "active" users (30 days).
	DefaultSeatWindowDays = 30
)

// ComputeSeatThreshold derives the threshold band from (used, allowed) per AC #8.
// Defined here (not in the EE package) so OSS handlers and unit tests can reuse it.
func ComputeSeatThreshold(used int64, allowed int) (percent float64, threshold string) {
	if allowed <= 0 {
		// Unlimited — never warn.
		return 0.0, SeatThresholdOK
	}
	percent = float64(used) / float64(allowed)
	switch {
	case percent >= SeatExceededPercent:
		return percent, SeatThresholdExceeded
	case percent >= SeatWarnPercent:
		return percent, SeatThresholdWarn
	default:
		return percent, SeatThresholdOK
	}
}

// Seat counting is fed by the canonical identity roster (Story 15.2): the EE
// seat reconciler reads IdentityService.BilledSeatCount — an entitlement-based
// COUNT(*) WHERE state='active', uniform across editions (R5-2). The former
// read-side seat-store abstraction and its per-login table were deleted; there
// is no separate seat store.

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

	// GetSeatUsage returns the cached active-seat snapshot for the current org.
	// Cold start (reconciler has not run yet) returns the sentinel
	// {Used: 0, LastUpdated: "", Threshold: "ok"} — the UI handles this by
	// rendering "Active users: calculating…" without a banner (STORY-465 AC #10).
	// OSS / no-op implementations always return the sentinel.
	GetSeatUsage(ctx context.Context) SeatUsage
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

// GetSeatUsage returns the cold-start sentinel — OSS builds have no Postgres to count.
func (s *NoopLicenseService) GetSeatUsage(_ context.Context) SeatUsage {
	return SeatUsage{
		WindowDays: DefaultSeatWindowDays,
		Threshold:  SeatThresholdOK,
	}
}

// Ensure NoopLicenseService implements LicenseService.
var _ LicenseService = (*NoopLicenseService)(nil)
