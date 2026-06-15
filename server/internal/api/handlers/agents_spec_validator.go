// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"

	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// AgentSpecValidator validates a generated spec against cluster policy
// (Story 50.3). The handler-owned one-method interface (the A2AInvoker
// precedent): the EE Gatekeeper validator implements it; OSS builds wire
// nil. A nil return means "nothing to validate" (no extractable RGD spec) OR
// the feature is not licensed — the result then carries no policyValidation
// and the web renders its Enterprise notice. Implementations never return an
// error: every failure mode maps onto a PolicyValidation status (a policy
// problem must never fail a completed run).
//
// The seam lives here (not on an invoke handler) because Story 53.1 removed
// the built-in invoke handler that used to own it; Story 53.5 re-homes it onto
// the surviving BYOA invoke path. It is wired only through RouterConfig and the
// EE/OSS InitAgentSpecValidator dispatch for now.
type AgentSpecValidator interface {
	ValidateSpec(ctx context.Context, responseText string) *runs.PolicyValidation
}
