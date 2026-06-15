// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// auditEmitFailures counts post-commit identity hook failures, labeled by
// event type (R5-7). A hook failure never rolls back the identity write and
// never fails the login — it is logged at ERROR and metered here so operators
// can alert on lost audit emission. Registered once via promauto against the
// default registerer (same pattern as the seat gauges).
var auditEmitFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "knodex_identity_audit_emit_failures_total",
		Help: "Identity post-commit audit-hook emission failures by event type (R5-7).",
	},
	[]string{"event_type"},
)
