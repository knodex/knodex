package models

import (
	"time"

	"github.com/provops-org/knodex/server/internal/deployment"
)

// Instance labels and annotations used for tracking
// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label
// on instances, so we don't set RGD labels ourselves.
const (
	// DeploymentModeLabel identifies the deployment mode used
	DeploymentModeLabel = "knodex.io/deployment-mode"
	// ProjectLabel identifies the project using namespace name (e.g., "acme"), not project ID (e.g., "proj-acme")
	ProjectLabel = "knodex.io/project"
	// RepositoryIDLabel tracks the repository used for GitOps
	RepositoryIDLabel = "knodex.io/repository-id"

	// Annotation keys for dashboard-specific metadata
	AnnotationInstanceID  = "knodex.io/instance-id"
	AnnotationCreatedBy   = "knodex.io/created-by"
	AnnotationCreatedAt   = "knodex.io/created-at"
	AnnotationProjectID   = "knodex.io/project-id"
	AnnotationGeneratedAt = "knodex.io/generated-at"
	AnnotationGeneratedBy = "knodex.io/generated-by"
)

// InstanceHealth represents the health status of an instance
type InstanceHealth string

const (
	HealthHealthy     InstanceHealth = "Healthy"
	HealthDegraded    InstanceHealth = "Degraded"
	HealthUnhealthy   InstanceHealth = "Unhealthy"
	HealthProgressing InstanceHealth = "Progressing"
	HealthUnknown     InstanceHealth = "Unknown"
)

// Instance represents a deployed instance of an RGD
type Instance struct {
	// Name is the instance resource name
	Name string `json:"name"`
	// Namespace is the instance namespace
	Namespace string `json:"namespace"`
	// RGDName is the name of the RGD that created this instance
	RGDName string `json:"rgdName"`
	// RGDNamespace is the namespace of the RGD
	RGDNamespace string `json:"rgdNamespace"`
	// APIVersion is the API version of this instance
	APIVersion string `json:"apiVersion"`
	// Kind is the Kind of this instance
	Kind string `json:"kind"`
	// Health is the calculated health status
	Health InstanceHealth `json:"health"`
	// Phase is the current phase from status
	Phase string `json:"phase,omitempty"`
	// Message is a human-readable status message
	Message string `json:"message,omitempty"`
	// Conditions are the status conditions
	Conditions []InstanceCondition `json:"conditions,omitempty"`
	// Spec contains the instance spec values
	Spec map[string]interface{} `json:"spec,omitempty"`
	// Status contains the instance status values
	Status map[string]interface{} `json:"status,omitempty"`
	// Labels from the instance metadata
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations from the instance metadata
	Annotations map[string]string `json:"annotations,omitempty"`
	// CreatedAt is when the instance was created
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the instance was last updated
	UpdatedAt time.Time `json:"updatedAt"`
	// ResourceVersion for optimistic concurrency (exposed for frontend edit conflict detection)
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// UID is the unique identifier
	UID string `json:"uid"`
	// GitOps deployment fields
	// DeploymentMode indicates how this instance was deployed
	DeploymentMode deployment.DeploymentMode `json:"deploymentMode,omitempty"`
	// GitInfo contains Git-related information for GitOps/Hybrid deployments
	GitInfo *deployment.GitInfo `json:"gitInfo,omitempty"`
	// ProjectID is the project that owns this instance
	ProjectID string `json:"projectId,omitempty"`
	// ProjectName is the display name of the project
	ProjectName string `json:"projectName,omitempty"`
	// GitOpsDrift indicates the live spec doesn't match the desired spec pushed to Git
	GitOpsDrift bool `json:"gitopsDrift,omitempty"`
	// DesiredSpec is the spec that was pushed to Git (for drift comparison)
	DesiredSpec map[string]interface{} `json:"desiredSpec,omitempty"`
}

// InstanceCondition represents a status condition
type InstanceCondition struct {
	// Type is the condition type (e.g., "Ready", "Available")
	Type string `json:"type"`
	// Status is the condition status ("True", "False", "Unknown")
	Status string `json:"status"`
	// Reason is a machine-readable reason
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message
	Message string `json:"message,omitempty"`
	// LastTransitionTime is when the condition last changed
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
}

// InstanceList represents a paginated list of instances
type InstanceList struct {
	Items      []Instance `json:"items"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// InstanceListOptions contains options for listing instances
type InstanceListOptions struct {
	// Namespace filters by namespace (empty = all namespaces)
	Namespace string
	// RGDName filters by RGD name
	RGDName string
	// RGDNamespace filters by RGD namespace
	RGDNamespace string
	// Health filters by health status
	Health InstanceHealth
	// DeploymentMode filters by deployment mode
	DeploymentMode deployment.DeploymentMode
	// ProjectID filters by project
	ProjectID string
	// Search filters by name (case-insensitive contains)
	Search string
	// Page is the page number (1-indexed)
	Page int
	// PageSize is the number of items per page
	PageSize int
	// SortBy is the field to sort by (name, createdAt, updatedAt, health)
	SortBy string
	// SortOrder is asc or desc
	SortOrder string
}

// DefaultInstanceListOptions returns default list options
func DefaultInstanceListOptions() InstanceListOptions {
	return InstanceListOptions{
		Page:      1,
		PageSize:  20,
		SortBy:    "createdAt", // Sort by creation time (newest first) for better UX
		SortOrder: "desc",
	}
}
