// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/health"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	checker *health.Checker
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(checker *health.Checker) *HealthHandler {
	return &HealthHandler{
		checker: checker,
	}
}

// Healthz handles liveness probe requests
// Liveness probes determine if the container needs to be restarted
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	status := h.checker.CheckLiveness(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if status.Status != health.StatusHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}

// Readyz handles readiness probe requests
// Readiness probes determine if the container can receive traffic
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	status := h.checker.CheckReadiness(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if status.Status != health.StatusHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Error("failed to encode readiness response", "error", err)
	}
}
