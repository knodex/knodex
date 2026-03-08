// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package compliance

// Severity constants for compliance findings
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// Deployment represents a deployment to be audited for compliance.
// This is a simplified representation for compliance checking purposes.
type Deployment struct {
	// Name is the deployment identifier
	Name string `json:"name"`

	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace"`

	// ProjectID is the project that owns this deployment
	ProjectID string `json:"projectId"`

	// RGDName is the ResourceGraphDefinition name
	RGDName string `json:"rgdName"`

	// Inputs contains the deployment input parameters
	Inputs map[string]interface{} `json:"inputs,omitempty"`

	// Labels contains deployment labels
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations contains deployment annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AuditResult contains the result of a compliance audit
type AuditResult struct {
	// Passed indicates whether the deployment passed all compliance checks
	Passed bool `json:"passed"`

	// Findings contains detailed information about each compliance check
	Findings []Finding `json:"findings"`
}

// Finding represents a single compliance check result
type Finding struct {
	// Severity indicates the importance of the finding (critical, high, medium, low)
	Severity string `json:"severity"`

	// Rule is the identifier of the compliance rule that was checked
	Rule string `json:"rule"`

	// Description provides details about the finding
	Description string `json:"description"`

	// Passed indicates whether this specific check passed
	Passed bool `json:"passed"`

	// Remediation provides guidance on how to fix the issue (if failed)
	Remediation string `json:"remediation,omitempty"`
}

// HasCritical returns true if there are any critical severity findings that failed
func (r *AuditResult) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical && !f.Passed {
			return true
		}
	}
	return false
}

// HasHigh returns true if there are any high severity findings that failed
func (r *AuditResult) HasHigh() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityHigh && !f.Passed {
			return true
		}
	}
	return false
}

// FailedFindings returns only the findings that did not pass
func (r *AuditResult) FailedFindings() []Finding {
	var failed []Finding
	for _, f := range r.Findings {
		if !f.Passed {
			failed = append(failed, f)
		}
	}
	return failed
}
