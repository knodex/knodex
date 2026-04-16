// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/compliance"
)

// ComplianceValidateRequest is the request body for pre-deploy validation.
type ComplianceValidateRequest struct {
	RGDName   string                 `json:"rgdName"`
	Project   string                 `json:"project"`
	Namespace string                 `json:"namespace"`
	Values    map[string]interface{} `json:"values"`
}

// ComplianceViolation represents a single compliance violation.
type ComplianceViolation struct {
	Policy   string `json:"policy"`
	Severity string `json:"severity"` // "warning" or "error"
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Guidance string `json:"guidance,omitempty"`
}

// ComplianceValidateResponse is the response for pre-deploy validation.
type ComplianceValidateResponse struct {
	Result     string                `json:"result"` // "pass", "warning", "block"
	Violations []ComplianceViolation `json:"violations"`
}

// ComplianceValidateHandler handles POST /api/v1/compliance/validate.
type ComplianceValidateHandler struct {
	checker compliance.ComplianceChecker
	logger  *slog.Logger
}

// NewComplianceValidateHandler creates a new compliance validate handler.
func NewComplianceValidateHandler(checker compliance.ComplianceChecker, logger *slog.Logger) *ComplianceValidateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ComplianceValidateHandler{
		checker: checker,
		logger:  logger.With("component", "compliance-validate-handler"),
	}
}

// Validate handles POST /api/v1/compliance/validate.
// Performs a dry-run compliance check without creating any K8s resources.
func (h *ComplianceValidateHandler) Validate(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	var req ComplianceValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body", nil)
		return
	}

	if req.RGDName == "" {
		response.BadRequest(w, "rgdName is required", nil)
		return
	}

	// If compliance is not enabled (OSS build), always return pass
	if h.checker == nil || !h.checker.IsEnabled() {
		response.WriteJSON(w, http.StatusOK, ComplianceValidateResponse{
			Result:     "pass",
			Violations: []ComplianceViolation{},
		})
		return
	}

	// Build deployment for compliance audit
	deployment := &compliance.Deployment{
		RGDName:   req.RGDName,
		ProjectID: req.Project,
		Namespace: req.Namespace,
		Inputs:    req.Values,
	}

	auditResult, err := h.checker.AuditDeployment(r.Context(), deployment)
	if err != nil {
		h.logger.Error("compliance audit failed",
			"userId", userCtx.UserID,
			"rgdName", req.RGDName,
			"error", err,
		)
		response.InternalError(w, "Compliance validation failed")
		return
	}

	// Map audit findings to violations
	resp := mapAuditResultToResponse(auditResult)
	response.WriteJSON(w, http.StatusOK, resp)
}

// mapAuditResultToResponse converts an AuditResult to the validate response format.
func mapAuditResultToResponse(auditResult *compliance.AuditResult) ComplianceValidateResponse {
	if auditResult == nil || auditResult.Passed {
		return ComplianceValidateResponse{
			Result:     "pass",
			Violations: []ComplianceViolation{},
		}
	}

	violations := make([]ComplianceViolation, 0)
	hasError := false

	for _, finding := range auditResult.Findings {
		if finding.Passed {
			continue
		}

		severity := "warning"
		if finding.Severity == compliance.SeverityCritical || finding.Severity == compliance.SeverityHigh {
			severity = "error"
			hasError = true
		}

		violations = append(violations, ComplianceViolation{
			Policy:   finding.Rule,
			Severity: severity,
			Message:  finding.Description,
			Guidance: finding.Remediation,
		})
	}

	result := "pass"
	if hasError {
		result = "block"
	} else if len(violations) > 0 {
		result = "warning"
	}

	return ComplianceValidateResponse{
		Result:     result,
		Violations: violations,
	}
}
