// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Team CRD constants.
//
// Team is cluster-scoped (unlike the namespaced Project CRD). It names a set
// of OIDC groups so operators can reuse a group set across projects instead of
// repeating raw group strings. A Team produces NO authorization on its own —
// it only becomes access when a Project's roles[].teams[] reference resolves
// the team to its groups (Story 10.2). See CLAUDE.md "Unified Casbin
// Authorization Model": there is ONE enforcement layer.
const (
	TeamGroup    = "knodex.io"
	TeamVersion  = "v1alpha1"
	TeamResource = "teams"
	TeamKind     = "Team"
)

// Team is a cluster-scoped object naming a set of OIDC groups.
// This is stored as a Kubernetes Custom Resource.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Team struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeamSpec   `json:"spec"`
	Status TeamStatus `json:"status,omitempty"`
}

// TeamSpec defines the desired state of a Team.
type TeamSpec struct {
	// Description is a human-readable description of the team.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// OIDCGroups is the set of OIDC group names this team represents.
	// At least one group is required. A later story resolves these groups
	// into Casbin policies when a Project role references the team.
	OIDCGroups []string `json:"oidcGroups" yaml:"oidcGroups"`
}

// TeamStatus defines the observed state of a Team.
type TeamStatus struct {
	// Conditions is an array of current status conditions.
	Conditions []TeamCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// TeamCondition represents a status condition.
type TeamCondition struct {
	// Type is the condition type (Ready, ValidationError).
	Type string `json:"type" yaml:"type"`

	// Status is the condition status (True, False, Unknown).
	Status string `json:"status" yaml:"status"`

	// LastTransitionTime is the last time the condition transitioned.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`

	// Reason is a brief machine-readable explanation.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`

	// Message is a human-readable explanation.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// Team condition type constants.
const (
	TeamConditionReady           = "Ready"
	TeamConditionValidationError = "ValidationError"
)

// TeamList contains a list of Team resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Team `json:"items"`
}

// DeepCopyObject implements runtime.Object interface for Team.
func (t *Team) DeepCopyObject() runtime.Object {
	if t == nil {
		return nil
	}
	out := new(Team)
	*out = *t
	out.TypeMeta = t.TypeMeta
	t.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	// Deep copy Spec
	out.Spec.Description = t.Spec.Description
	if t.Spec.OIDCGroups != nil {
		out.Spec.OIDCGroups = make([]string, len(t.Spec.OIDCGroups))
		copy(out.Spec.OIDCGroups, t.Spec.OIDCGroups)
	}

	// Deep copy Status
	if t.Status.Conditions != nil {
		out.Status.Conditions = make([]TeamCondition, len(t.Status.Conditions))
		for i, cond := range t.Status.Conditions {
			out.Status.Conditions[i] = TeamCondition{
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

// DeepCopyObject implements runtime.Object interface for TeamList.
func (t *TeamList) DeepCopyObject() runtime.Object {
	if t == nil {
		return nil
	}
	out := new(TeamList)
	*out = *t
	out.TypeMeta = t.TypeMeta
	t.ListMeta.DeepCopyInto(&out.ListMeta)

	if t.Items != nil {
		out.Items = make([]Team, len(t.Items))
		for i := range t.Items {
			out.Items[i] = *t.Items[i].DeepCopyObject().(*Team)
		}
	}

	return out
}
