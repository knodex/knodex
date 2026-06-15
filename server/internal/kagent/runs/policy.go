// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

// Policy validation status values for PolicyValidation.Status (and
// RevisedStatus). Story 50.3: pure data shapes — the OSS binary never
// produces these (the field stays absent); the EE Gatekeeper validator
// fills them through the handlers.AgentSpecValidator seam.
const (
	// PolicyStatusPassed means every reviewed object was allowed by all
	// active Gatekeeper constraints.
	PolicyStatusPassed = "passed"
	// PolicyStatusFailed means at least one constraint rejected (or warned
	// about) a reviewed object — Violations carries the details.
	PolicyStatusFailed = "failed"
	// PolicyStatusUnavailable means validation could not run (Gatekeeper not
	// installed or unreachable) — the spec is still returned, never failed.
	PolicyStatusUnavailable = "unavailable"
)

// PolicyViolation is one Gatekeeper constraint violation found while
// validating a generated spec (Story 50.3 AC #2). All strings originate from
// cluster objects / the Gatekeeper webhook and are UNTRUSTED — the result
// store strictly sanitizes them and the web renders them as escaped text.
type PolicyViolation struct {
	// Constraint is the violated constraint's name (parsed from the webhook
	// denial message).
	Constraint string `json:"constraint"`
	// ConstraintKind is the constraint's kind, enriched from the EE
	// constraint cache; empty on a cache miss.
	ConstraintKind string `json:"constraintKind,omitempty"`
	// EnforcementAction is the constraint's action ("deny", "warn", ...).
	EnforcementAction string `json:"enforcementAction,omitempty"`
	// Message is the policy's human-readable violation message.
	Message string `json:"message"`
	// ResourceID is the spec.resources[].id the reviewed object came from;
	// empty when the violation is against the RGD object itself.
	ResourceID string `json:"resourceId,omitempty"`
}

// PolicyValidation is the policy-validation outcome attached to a Result
// (Story 50.3). Absent (nil) on OSS builds and on EE builds without the
// compliance license — the web keys its Enterprise notice on that absence.
type PolicyValidation struct {
	// Status is one of the PolicyStatus* constants.
	Status string `json:"status"`
	// Reason explains an "unavailable" status in stable, non-leaking copy
	// (never an internal URL or raw transport error).
	Reason string `json:"reason,omitempty"`
	// Violations carries the constraint violations when Status is "failed".
	Violations []PolicyViolation `json:"violations,omitempty"`
	// RevisedResponse is the full agent text of the single automatic
	// revision attempt (failed validations only; empty when the revision
	// invoke itself failed).
	RevisedResponse string `json:"revisedResponse,omitempty"`
	// RevisedStatus is the re-validation outcome of the revised spec
	// (PolicyStatus* constant); empty when re-validation produced nothing.
	RevisedStatus string `json:"revisedStatus,omitempty"`
	// RevisedViolations carries violations of the revised spec when
	// RevisedStatus is "failed".
	RevisedViolations []PolicyViolation `json:"revisedViolations,omitempty"`
}
