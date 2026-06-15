// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// identityObserveFailures counts best-effort ObserveLogin failures at the auth
// callback (Story 15.2 / NFR-U1). A failure here is logged at WARN and metered
// but NEVER fails the login. Labeled by reason so operators can distinguish a
// persistent DB outage from transient errors. Registered once via promauto.
var identityObserveFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "knodex_identity_observe_failures_total",
		Help: "Best-effort identity ObserveLogin failures at the OIDC callback (NFR-U1).",
	},
	[]string{"reason"},
)

// observeFailureReason classifies an ObserveLogin error into a low-cardinality
// label so the `reason` dimension carries the discriminating information its
// Help text promises (DB outage vs transient vs cancellation). The auth package
// cannot import internal/services (import cycle), so classification is done on
// context sentinels + error-string heuristics rather than typed errors.
func observeFailureReason(err error) string {
	switch {
	case err == nil:
		return "observe_error"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not implemented"):
		return "not_implemented"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "connect") || strings.Contains(msg, "connection") ||
		strings.Contains(msg, "dial") || strings.Contains(msg, "refused") ||
		strings.Contains(msg, "no route") || strings.Contains(msg, "pool"):
		return "unavailable"
	default:
		return "observe_error"
	}
}
