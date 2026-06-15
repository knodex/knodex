// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

// Annotation constants for Team provenance.
//
// These are the canonical values written by TeamService.CreateTeam and read by
// the handler to derive the read-only managed field. A Team carrying the
// control-plane provenance value is treated as externally managed (read-only).
const (
	// TeamAnnotationCreatedBy is the metadata annotation key used to record
	// who (or what component) created a Team CRD.
	TeamAnnotationCreatedBy = "knodex.io/created-by"

	// TeamAnnotationCreatedByControlPlane is the value written when the
	// control-plane reconciler materializes a Team mirror from a managed
	// Keycloak group. Operator-authored teams carry no such annotation.
	TeamAnnotationCreatedByControlPlane = "control-plane"
)
