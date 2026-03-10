// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package manifest

import "time"

// DeploymentMode represents how an instance should be deployed
type DeploymentMode string

const (
	// ModeDirect deploys instance directly to cluster via K8s API
	ModeDirect DeploymentMode = "direct"
	// ModeGitOps pushes manifest to Git, waits for GitOps tool to sync
	ModeGitOps DeploymentMode = "gitops"
	// ModeHybrid deploys to cluster AND pushes to Git
	ModeHybrid DeploymentMode = "hybrid"
)

// InstanceSpec contains the specification for generating a manifest
type InstanceSpec struct {
	// Name is the instance resource name
	Name string
	// Namespace is the instance namespace
	Namespace string
	// RGDName is the name of the RGD that defines this instance type
	RGDName string
	// RGDVersion is the version of the RGD
	RGDVersion string
	// RGDNamespace is the namespace where the RGD exists
	RGDNamespace string
	// APIVersion is the full API version (group/version)
	APIVersion string
	// Kind is the resource Kind
	Kind string
	// Spec contains the instance specification data
	Spec map[string]interface{}
	// CreatedBy is the email/ID of the user creating this instance
	CreatedBy string
	// CreatedAt is when the instance was created
	CreatedAt time.Time
	// DeploymentMode indicates how this instance should be deployed
	DeploymentMode DeploymentMode
	// InstanceID is the unique identifier for tracking
	InstanceID string
	// ProjectID is the project that owns this instance
	ProjectID string
}

// ManifestOutput contains the generated manifest and metadata
type ManifestOutput struct {
	// Manifest is the Kubernetes YAML manifest
	Manifest string
	// Metadata is the companion metadata file content
	Metadata string
	// ManifestPath is the recommended file path for the manifest
	ManifestPath string
	// MetadataPath is the recommended file path for the metadata
	MetadataPath string
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
