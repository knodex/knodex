// Package services provides business logic services following clean architecture principles.
package services

import (
	"context"
	"time"
)

// ComplianceStatus represents the availability status of the compliance feature.
type ComplianceStatus struct {
	// Available indicates if compliance features are fully operational
	Available bool `json:"available"`

	// Enterprise indicates if this is an enterprise build
	Enterprise bool `json:"enterprise"`

	// Message provides a human-readable status explanation
	Message string `json:"message"`

	// Gatekeeper indicates Gatekeeper status: "installed", "not_installed", "syncing"
	Gatekeeper string `json:"gatekeeper,omitempty"`
}

// ComplianceService defines the interface for OPA Gatekeeper compliance operations.
// In OSS builds, this returns enterprise-required errors.
// In EE builds, this returns actual compliance data from Gatekeeper.
type ComplianceService interface {
	// IsEnabled returns true if the compliance feature is available.
	// In OSS builds, this returns false.
	IsEnabled() bool

	// GetStatus returns detailed status information about the compliance feature.
	// This is useful for debugging and UI display.
	GetStatus() *ComplianceStatus

	// ListConstraintTemplates returns all ConstraintTemplates.
	ListConstraintTemplates(ctx context.Context) ([]ConstraintTemplate, error)

	// GetConstraintTemplate returns a specific ConstraintTemplate by name.
	GetConstraintTemplate(ctx context.Context, name string) (*ConstraintTemplate, error)

	// ListConstraints returns all Constraints.
	ListConstraints(ctx context.Context) ([]Constraint, error)

	// GetConstraint returns a specific Constraint by kind and name.
	GetConstraint(ctx context.Context, kind, name string) (*Constraint, error)

	// ListViolations returns all violations across all constraints.
	ListViolations(ctx context.Context) ([]Violation, error)

	// GetViolationsByConstraint returns violations for a specific constraint.
	GetViolationsByConstraint(ctx context.Context, kind, name string) ([]Violation, error)

	// GetViolationsByResource returns violations for a specific resource.
	GetViolationsByResource(ctx context.Context, kind, namespace, name string) ([]Violation, error)

	// GetSummary returns aggregate compliance statistics.
	GetSummary(ctx context.Context) (*ComplianceSummary, error)

	// UpdateConstraintEnforcement updates a constraint's enforcementAction field.
	// Valid values are: deny, warn, dryrun.
	// Returns an error if the constraint doesn't exist.
	UpdateConstraintEnforcement(ctx context.Context, kind, name, newAction string) (*Constraint, error)

	// CreateConstraint creates a new constraint from a ConstraintTemplate.
	// Returns an error if the template doesn't exist or parameters are invalid.
	CreateConstraint(ctx context.Context, req CreateConstraintRequest) (*Constraint, error)
}

// ConstraintTemplate represents an OPA Gatekeeper ConstraintTemplate.
// ConstraintTemplates define the Rego policy logic and create a new constraint CRD kind.
type ConstraintTemplate struct {
	// Name is the ConstraintTemplate resource name
	Name string `json:"name"`

	// Kind is the constraint kind this template creates (e.g., K8sRequiredLabels)
	Kind string `json:"kind"`

	// Description provides human-readable explanation of what the template enforces
	Description string `json:"description"`

	// Rego contains the policy logic (optional in responses for security)
	Rego string `json:"rego,omitempty"`

	// Parameters defines the schema for constraint parameters
	Parameters map[string]any `json:"parameters,omitempty"`

	// Labels are Kubernetes labels on the ConstraintTemplate
	Labels map[string]string `json:"labels,omitempty"`

	// CreatedAt is when the ConstraintTemplate was created
	CreatedAt time.Time `json:"createdAt"`
}

// Constraint represents an active OPA Gatekeeper constraint instance.
// Constraints are created from ConstraintTemplates and define the scope of enforcement.
type Constraint struct {
	// Name is the constraint resource name
	Name string `json:"name"`

	// Kind is the constraint kind (matches the ConstraintTemplate's generated kind)
	Kind string `json:"kind"`

	// TemplateName is the name of the ConstraintTemplate this constraint uses
	TemplateName string `json:"templateName"`

	// EnforcementAction specifies what happens on violation: deny, dryrun, or warn
	EnforcementAction string `json:"enforcementAction"`

	// Match defines which resources this constraint applies to
	Match ConstraintMatch `json:"match"`

	// Parameters are the values passed to the Rego policy
	Parameters map[string]any `json:"parameters,omitempty"`

	// ViolationCount is the current number of violations from the audit controller
	ViolationCount int `json:"violationCount"`

	// Violations contains the detailed violation data from the audit controller.
	Violations []Violation `json:"violations,omitempty"`

	// Labels are Kubernetes labels on the constraint
	Labels map[string]string `json:"labels,omitempty"`

	// CreatedAt is when the constraint was created
	CreatedAt time.Time `json:"createdAt"`
}

// ConstraintMatch defines which resources a constraint applies to.
type ConstraintMatch struct {
	// Kinds specifies which resource kinds to match
	Kinds []MatchKind `json:"kinds,omitempty"`

	// Namespaces limits enforcement to specific namespaces (empty = all namespaces)
	Namespaces []string `json:"namespaces,omitempty"`

	// Scope specifies the scope of resources: Cluster, Namespaced, or * (both)
	Scope string `json:"scope,omitempty"`
}

// MatchKind specifies a group of Kubernetes resource kinds to match.
type MatchKind struct {
	// APIGroups are the API groups to match (empty string = core API group)
	APIGroups []string `json:"apiGroups"`

	// Kinds are the resource kinds to match within the API groups
	Kinds []string `json:"kinds"`
}

// Violation represents a policy violation detected by Gatekeeper's audit controller.
type Violation struct {
	// ConstraintName is the name of the constraint that was violated
	ConstraintName string `json:"constraintName"`

	// ConstraintKind is the kind of the constraint that was violated
	ConstraintKind string `json:"constraintKind"`

	// Resource identifies the Kubernetes resource that violates the constraint
	Resource ViolationResource `json:"resource"`

	// Message is the human-readable violation message from the Rego policy
	Message string `json:"message"`

	// EnforcementAction is the action taken: deny, dryrun, or warn
	EnforcementAction string `json:"enforcementAction"`
}

// ViolationResource identifies a Kubernetes resource that violated a constraint.
type ViolationResource struct {
	// Kind is the resource kind (e.g., Pod, Deployment)
	Kind string `json:"kind"`

	// Namespace is the resource namespace (empty for cluster-scoped resources)
	Namespace string `json:"namespace"`

	// Name is the resource name
	Name string `json:"name"`

	// APIGroup is the API group of the resource (empty for core API group)
	APIGroup string `json:"apiGroup,omitempty"`
}

// ComplianceSummary provides aggregate Gatekeeper compliance statistics.
type ComplianceSummary struct {
	// TotalTemplates is the count of ConstraintTemplates
	TotalTemplates int `json:"totalTemplates"`

	// TotalConstraints is the count of active constraints
	TotalConstraints int `json:"totalConstraints"`

	// TotalViolations is the total count of violations across all constraints
	TotalViolations int `json:"totalViolations"`

	// ByEnforcement breaks down violations by enforcement action
	// Keys are: deny, warn, dryrun
	ByEnforcement map[string]int `json:"byEnforcement"`
}

// CreateConstraintRequest contains the data needed to create a new constraint.
type CreateConstraintRequest struct {
	// Name is the constraint resource name (must be DNS-compatible)
	Name string `json:"name"`

	// TemplateName is the name of the ConstraintTemplate to use
	TemplateName string `json:"templateName"`

	// EnforcementAction specifies what happens on violation: deny, dryrun, or warn
	// Defaults to "deny" if not specified
	EnforcementAction string `json:"enforcementAction,omitempty"`

	// Match defines which resources this constraint applies to
	Match *ConstraintMatch `json:"match,omitempty"`

	// Parameters are the values passed to the Rego policy
	Parameters map[string]any `json:"parameters,omitempty"`

	// Labels are Kubernetes labels to apply to the constraint
	Labels map[string]string `json:"labels,omitempty"`
}
