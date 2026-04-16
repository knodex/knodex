// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// User CRD has been removed. Local users are now stored in
// ConfigMap/Secret following the ArgoCD pattern. OIDC users are not
// persisted - their info comes from JWT claims at runtime.
// See server/internal/auth/account_store.go for the new implementation.

// Role constants (used for RBAC permissions)
const (
	RolePlatformAdmin = "platform-admin"
	RoleDeveloper     = "developer"
	RoleViewer        = "viewer"
)

// Project CRD constants
const (
	ProjectGroup    = "knodex.io"
	ProjectVersion  = "v1alpha1"
	ProjectResource = "projects"
	ProjectKind     = "Project"
)

// Project represents a multi-tenancy boundary with RBAC policies
// Mirrors ArgoCD AppProject structure for GitOps alignment
// This is stored as a Kubernetes Custom Resource
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

// ProjectType represents the tier of a project (platform or app).
type ProjectType string

const (
	// ProjectTypePlatform is for infrastructure/cluster management projects.
	ProjectTypePlatform ProjectType = "platform"
	// ProjectTypeApp is for application workload projects (default).
	ProjectTypeApp ProjectType = "app"
)

// ProjectSpec defines the desired state of a Project
// Mirrors ArgoCD AppProjectSpec structure
type ProjectSpec struct {
	// Type is the project tier: "platform" or "app".
	// Immutable after creation. Defaults to "app" when empty (backward compatible).
	Type ProjectType `json:"type,omitempty" yaml:"type,omitempty"`

	// Description is a human-readable description of the project
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Destinations is a list of allowed deployment targets (namespaces)
	Destinations []Destination `json:"destinations" yaml:"destinations"`

	// ClusterResourceWhitelist defines which cluster-scoped resources are allowed
	// If empty, no cluster resources are allowed
	ClusterResourceWhitelist []ResourceSpec `json:"clusterResourceWhitelist,omitempty" yaml:"clusterResourceWhitelist,omitempty"`

	// ClusterResourceBlacklist defines which cluster-scoped resources are denied
	// Deny rules take precedence over whitelist
	ClusterResourceBlacklist []ResourceSpec `json:"clusterResourceBlacklist,omitempty" yaml:"clusterResourceBlacklist,omitempty"`

	// NamespaceResourceWhitelist defines which namespace-scoped resources are allowed
	// If empty, no namespace resources are allowed
	NamespaceResourceWhitelist []ResourceSpec `json:"namespaceResourceWhitelist,omitempty" yaml:"namespaceResourceWhitelist,omitempty"`

	// NamespaceResourceBlacklist defines which namespace-scoped resources are denied
	// Deny rules take precedence over whitelist
	NamespaceResourceBlacklist []ResourceSpec `json:"namespaceResourceBlacklist,omitempty" yaml:"namespaceResourceBlacklist,omitempty"`

	// Roles are custom RBAC roles specific to this project
	// Each role contains Casbin policy strings
	Roles []ProjectRole `json:"roles,omitempty" yaml:"roles,omitempty"`

	// Clusters binds the project to CAPI clusters (App Projects only).
	// Omit for monocluster mode (backward compatible).
	Clusters []ClusterBinding `json:"clusters,omitempty" yaml:"clusters,omitempty"`

	// Namespace is the namespace claim across all bound clusters (App Projects only).
	// Required when Clusters is non-empty.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// NamespaceRetention controls what happens to provisioned namespaces when
	// the project is deleted. "delete" removes them; "keep" (default) leaves
	// them in place. Omitted/empty means "keep" (safe default).
	NamespaceRetention string `json:"namespaceRetention,omitempty" yaml:"namespaceRetention,omitempty"`
}

// ClusterBinding references a target cluster by name.
type ClusterBinding struct {
	// ClusterRef is the name of the target cluster.
	ClusterRef string `json:"clusterRef" yaml:"clusterRef"`
}

// Destination represents a deployment target (namespace-only for single-cluster)
// All deployments target the local cluster, so only Namespace is required.
type Destination struct {
	// Namespace is the target Kubernetes namespace
	// Supports wildcards: "*", "dev-*", "team-*"
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Name is an optional identifier for the destination
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// ResourceSpec defines a Kubernetes resource type
// Mirrors ArgoCD GroupKind structure
type ResourceSpec struct {
	// Group is the Kubernetes API group (e.g., "apps", "")
	// Use "*" to match all groups
	Group string `json:"group" yaml:"group"`

	// Kind is the Kubernetes resource kind (e.g., "Deployment", "Pod")
	// Use "*" to match all kinds
	Kind string `json:"kind" yaml:"kind"`
}

// ProjectRole defines a custom RBAC role for a project
// Mirrors ArgoCD ProjectRole structure
type ProjectRole struct {
	// Name is the role identifier (e.g., "developer", "viewer")
	Name string `json:"name" yaml:"name"`

	// Description is a human-readable description of the role
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Policies are Casbin policy strings defining permissions
	// Format: "p, subject, resource, action, object, effect"
	// Example: "p, proj:myproject:developer, applications, *, myproject/*, allow"
	Policies []string `json:"policies" yaml:"policies"`

	// Groups are OIDC group names bound to this role
	// Users in these groups will have this role
	Groups []string `json:"groups,omitempty" yaml:"groups,omitempty"`

	// Destinations is an optional list of namespace patterns from the project's
	// destinations list. When set, policies for this role are scoped to only
	// these namespaces. When empty/nil, the role gets project-wide policies
	// (backward compatible).
	Destinations []string `json:"destinations,omitempty" yaml:"destinations,omitempty"`
}

// ClusterPhase represents the provisioning state of a cluster binding.
type ClusterPhase string

const (
	ClusterPhasePending      ClusterPhase = "Pending"
	ClusterPhaseProvisioning ClusterPhase = "Provisioning"
	ClusterPhaseProvisioned  ClusterPhase = "Provisioned"
	ClusterPhaseDeleting     ClusterPhase = "Deleting"
	ClusterPhaseDeleted      ClusterPhase = "Deleted"
	ClusterPhaseFailed       ClusterPhase = "Failed"
	ClusterPhaseUnknown      ClusterPhase = "Unknown"
	ClusterPhaseUnreachable  ClusterPhase = "ClusterUnreachable"
)

// ClusterState tracks the provisioning status of a single cluster binding.
type ClusterState struct {
	// ClusterRef is the name of the target cluster.
	ClusterRef string `json:"clusterRef" yaml:"clusterRef"`
	// Phase is the provisioning phase (Pending, Provisioning, Provisioned, Deleting, Deleted, Failed, Unknown, ClusterUnreachable).
	Phase ClusterPhase `json:"phase" yaml:"phase"`
	// Message is a human-readable status message (e.g. error details).
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// ProjectStatus defines the observed state of a Project
type ProjectStatus struct {
	// Conditions is an array of current status conditions
	Conditions []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	// ClusterStates tracks per-cluster provisioning status (App Projects with cluster bindings).
	ClusterStates []ClusterState `json:"clusterStates,omitempty" yaml:"clusterStates,omitempty"`
}

// ProjectCondition represents a status condition
type ProjectCondition struct {
	// Type is the condition type (Ready, ValidationError, SyncError)
	Type string `json:"type" yaml:"type"`

	// Status is the condition status (True, False, Unknown)
	Status string `json:"status" yaml:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`

	// Reason is a brief machine-readable explanation
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`

	// Message is a human-readable explanation
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// Condition type constants
const (
	ProjectConditionReady           = "Ready"
	ProjectConditionValidationError = "ValidationError"
	ProjectConditionSyncError       = "SyncError"
)

// Condition status constants
const (
	ConditionStatusTrue    = "True"
	ConditionStatusFalse   = "False"
	ConditionStatusUnknown = "Unknown"
)

// ProjectList contains a list of Project resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Project `json:"items"`
}

// ProjectInfo is a simplified view of Project for API responses
type ProjectInfo struct {
	ID                         string             `json:"id"` // resource name
	Description                string             `json:"description,omitempty"`
	Destinations               []Destination      `json:"destinations"`
	ClusterResourceWhitelist   []ResourceSpec     `json:"clusterResourceWhitelist,omitempty"`
	ClusterResourceBlacklist   []ResourceSpec     `json:"clusterResourceBlacklist,omitempty"`
	NamespaceResourceWhitelist []ResourceSpec     `json:"namespaceResourceWhitelist,omitempty"`
	NamespaceResourceBlacklist []ResourceSpec     `json:"namespaceResourceBlacklist,omitempty"`
	Roles                      []ProjectRole      `json:"roles,omitempty"`
	Conditions                 []ProjectCondition `json:"conditions,omitempty"`
	CreatedAt                  *time.Time         `json:"createdAt,omitempty"`
	ResourceVersion            string             `json:"-"` // for optimistic concurrency
}

// ToProjectInfo converts Project CRD to ProjectInfo for API responses
func (p *Project) ToProjectInfo() *ProjectInfo {
	info := &ProjectInfo{
		ID:                         p.Name,
		Description:                p.Spec.Description,
		Destinations:               p.Spec.Destinations,
		ClusterResourceWhitelist:   p.Spec.ClusterResourceWhitelist,
		ClusterResourceBlacklist:   p.Spec.ClusterResourceBlacklist,
		NamespaceResourceWhitelist: p.Spec.NamespaceResourceWhitelist,
		NamespaceResourceBlacklist: p.Spec.NamespaceResourceBlacklist,
		Roles:                      p.Spec.Roles,
		Conditions:                 p.Status.Conditions,
		ResourceVersion:            p.ResourceVersion,
	}

	if p.CreationTimestamp.Time.IsZero() == false {
		t := p.CreationTimestamp.Time
		info.CreatedAt = &t
	}

	return info
}

// DeepCopyObject implements runtime.Object interface for Project
func (p *Project) DeepCopyObject() runtime.Object {
	if p == nil {
		return nil
	}
	out := new(Project)
	*out = *p
	out.TypeMeta = p.TypeMeta
	p.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	// Deep copy Spec
	out.Spec.Type = p.Spec.Type
	out.Spec.Description = p.Spec.Description

	if p.Spec.Destinations != nil {
		out.Spec.Destinations = make([]Destination, len(p.Spec.Destinations))
		for i, d := range p.Spec.Destinations {
			out.Spec.Destinations[i] = Destination{
				Namespace: d.Namespace,
				Name:      d.Name,
			}
		}
	}

	if p.Spec.ClusterResourceWhitelist != nil {
		out.Spec.ClusterResourceWhitelist = make([]ResourceSpec, len(p.Spec.ClusterResourceWhitelist))
		copy(out.Spec.ClusterResourceWhitelist, p.Spec.ClusterResourceWhitelist)
	}

	if p.Spec.ClusterResourceBlacklist != nil {
		out.Spec.ClusterResourceBlacklist = make([]ResourceSpec, len(p.Spec.ClusterResourceBlacklist))
		copy(out.Spec.ClusterResourceBlacklist, p.Spec.ClusterResourceBlacklist)
	}

	if p.Spec.NamespaceResourceWhitelist != nil {
		out.Spec.NamespaceResourceWhitelist = make([]ResourceSpec, len(p.Spec.NamespaceResourceWhitelist))
		copy(out.Spec.NamespaceResourceWhitelist, p.Spec.NamespaceResourceWhitelist)
	}

	if p.Spec.NamespaceResourceBlacklist != nil {
		out.Spec.NamespaceResourceBlacklist = make([]ResourceSpec, len(p.Spec.NamespaceResourceBlacklist))
		copy(out.Spec.NamespaceResourceBlacklist, p.Spec.NamespaceResourceBlacklist)
	}

	if p.Spec.Roles != nil {
		out.Spec.Roles = make([]ProjectRole, len(p.Spec.Roles))
		for i, role := range p.Spec.Roles {
			out.Spec.Roles[i] = ProjectRole{
				Name:        role.Name,
				Description: role.Description,
			}
			if role.Policies != nil {
				out.Spec.Roles[i].Policies = make([]string, len(role.Policies))
				copy(out.Spec.Roles[i].Policies, role.Policies)
			}
			if role.Groups != nil {
				out.Spec.Roles[i].Groups = make([]string, len(role.Groups))
				copy(out.Spec.Roles[i].Groups, role.Groups)
			}
			if role.Destinations != nil {
				out.Spec.Roles[i].Destinations = make([]string, len(role.Destinations))
				copy(out.Spec.Roles[i].Destinations, role.Destinations)
			}
		}
	}

	if p.Spec.Clusters != nil {
		out.Spec.Clusters = make([]ClusterBinding, len(p.Spec.Clusters))
		copy(out.Spec.Clusters, p.Spec.Clusters)
	}
	out.Spec.Namespace = p.Spec.Namespace

	// Deep copy Status
	if p.Status.ClusterStates != nil {
		out.Status.ClusterStates = make([]ClusterState, len(p.Status.ClusterStates))
		copy(out.Status.ClusterStates, p.Status.ClusterStates)
	}
	if p.Status.Conditions != nil {
		out.Status.Conditions = make([]ProjectCondition, len(p.Status.Conditions))
		for i, cond := range p.Status.Conditions {
			out.Status.Conditions[i] = ProjectCondition{
				Type:               cond.Type,
				Status:             cond.Status,
				LastTransitionTime: *cond.LastTransitionTime.DeepCopy(),
				Reason:             cond.Reason,
				Message:            cond.Message,
			}
		}
	}

	return out
}

// DeepCopyObject implements runtime.Object interface for ProjectList
func (p *ProjectList) DeepCopyObject() runtime.Object {
	if p == nil {
		return nil
	}
	out := new(ProjectList)
	*out = *p
	out.TypeMeta = p.TypeMeta
	p.ListMeta.DeepCopyInto(&out.ListMeta)

	if p.Items != nil {
		out.Items = make([]Project, len(p.Items))
		for i := range p.Items {
			out.Items[i] = *p.Items[i].DeepCopyObject().(*Project)
		}
	}

	return out
}
