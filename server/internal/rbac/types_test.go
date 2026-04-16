// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRoleConstants tests role constant values
func TestRoleConstants(t *testing.T) {
	if RolePlatformAdmin != "platform-admin" {
		t.Errorf("Expected RolePlatformAdmin = 'platform-admin', got %s", RolePlatformAdmin)
	}
	if RoleDeveloper != "developer" {
		t.Errorf("Expected RoleDeveloper = 'developer', got %s", RoleDeveloper)
	}
	if RoleViewer != "viewer" {
		t.Errorf("Expected RoleViewer = 'viewer', got %s", RoleViewer)
	}
}

// User CRD tests removed - User CRD is no longer part of the architecture
// Local users are now stored in ConfigMap/Secret
// OIDC users are ephemeral and not persisted

// TestProjectDeepCopyObject tests Project.DeepCopyObject
func TestProjectDeepCopyObject(t *testing.T) {
	t.Run("NilProject", func(t *testing.T) {
		var p *Project = nil
		result := p.DeepCopyObject()
		if result != nil {
			t.Error("Expected nil result for nil project")
		}
	})

	t.Run("FullProject", func(t *testing.T) {
		now := metav1.Now()
		p := &Project{
			TypeMeta: metav1.TypeMeta{
				Kind:       ProjectKind,
				APIVersion: ProjectGroup + "/" + ProjectVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-project",
				ResourceVersion: "456",
			},
			Spec: ProjectSpec{
				Description: "Test project description",
				Destinations: []Destination{
					{Namespace: "default"},
					{Name: "prod-cluster", Namespace: "production"},
				},
				ClusterResourceWhitelist: []ResourceSpec{
					{Group: "", Kind: "Namespace"},
				},
				ClusterResourceBlacklist: []ResourceSpec{
					{Group: "", Kind: "Node"},
				},
				NamespaceResourceWhitelist: []ResourceSpec{
					{Group: "apps", Kind: "Deployment"},
				},
				NamespaceResourceBlacklist: []ResourceSpec{
					{Group: "", Kind: "Secret"},
				},
				Roles: []ProjectRole{
					{
						Name:        "developer",
						Description: "Developer role",
						Policies:    []string{"p, proj:test:developer, applications, *, test/*, allow"},
						Groups:      []string{"dev-team"},
					},
				},
			},
			Status: ProjectStatus{
				Conditions: []ProjectCondition{
					{
						Type:               ProjectConditionReady,
						Status:             ConditionStatusTrue,
						LastTransitionTime: now,
						Reason:             "Synced",
						Message:            "Project is ready",
					},
				},
			},
		}

		copy := p.DeepCopyObject().(*Project)

		// Verify basic fields
		if copy.Name != p.Name {
			t.Errorf("Name mismatch: %s != %s", copy.Name, p.Name)
		}
		if copy.Spec.Description != p.Spec.Description {
			t.Errorf("Description mismatch")
		}

		// Verify slices are deep copied
		if len(copy.Spec.Destinations) != 2 {
			t.Errorf("Expected 2 destinations, got %d", len(copy.Spec.Destinations))
		}
		if len(copy.Spec.ClusterResourceWhitelist) != 1 {
			t.Errorf("Expected 1 cluster whitelist, got %d", len(copy.Spec.ClusterResourceWhitelist))
		}
		if len(copy.Spec.Roles) != 1 {
			t.Errorf("Expected 1 role, got %d", len(copy.Spec.Roles))
		}

		// Verify role deep copy
		if len(copy.Spec.Roles[0].Policies) != 1 {
			t.Errorf("Expected 1 policy, got %d", len(copy.Spec.Roles[0].Policies))
		}
		if len(copy.Spec.Roles[0].Groups) != 1 {
			t.Errorf("Expected 1 group, got %d", len(copy.Spec.Roles[0].Groups))
		}

		// Verify conditions deep copy
		if len(copy.Status.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(copy.Status.Conditions))
		}
	})

	t.Run("ProjectWithNilFields", func(t *testing.T) {
		p := &Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "minimal-project",
			},
			Spec: ProjectSpec{
				Destinations: nil,
				Roles:        nil,
			},
		}

		copy := p.DeepCopyObject().(*Project)

		if copy.Spec.Destinations != nil {
			t.Error("Destinations should remain nil")
		}
		if copy.Spec.Roles != nil {
			t.Error("Roles should remain nil")
		}
	})
}

// TestProjectListDeepCopyObject tests ProjectList.DeepCopyObject
func TestProjectListDeepCopyObject(t *testing.T) {
	t.Run("NilProjectList", func(t *testing.T) {
		var pl *ProjectList = nil
		result := pl.DeepCopyObject()
		if result != nil {
			t.Error("Expected nil result for nil project list")
		}
	})

	t.Run("ProjectListWithItems", func(t *testing.T) {
		pl := &ProjectList{
			Items: []Project{
				{ObjectMeta: metav1.ObjectMeta{Name: "proj-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "proj-2"}},
			},
		}

		copy := pl.DeepCopyObject().(*ProjectList)

		if len(copy.Items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(copy.Items))
		}
	})
}

// TestProjectToProjectInfo tests Project.ToProjectInfo conversion
func TestProjectToProjectInfo(t *testing.T) {
	t.Run("FullProject", func(t *testing.T) {
		now := metav1.Now()
		p := &Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-project",
				ResourceVersion:   "123",
				CreationTimestamp: now,
			},
			Spec: ProjectSpec{
				Description: "Test description",
				Destinations: []Destination{
					{Namespace: "*"},
				},
				Roles: []ProjectRole{
					{Name: "admin", Policies: []string{"p, admin, *, *"}},
				},
			},
			Status: ProjectStatus{
				Conditions: []ProjectCondition{
					{Type: ProjectConditionReady, Status: ConditionStatusTrue},
				},
			},
		}

		info := p.ToProjectInfo()

		if info.ID != "test-project" {
			t.Errorf("Expected ID 'test-project', got %s", info.ID)
		}
		if info.Description != "Test description" {
			t.Errorf("Expected description 'Test description', got %s", info.Description)
		}
		if info.ResourceVersion != "123" {
			t.Errorf("Expected ResourceVersion '123', got %s", info.ResourceVersion)
		}
		if info.CreatedAt == nil {
			t.Error("Expected CreatedAt to be set")
		}
		if len(info.Roles) != 1 {
			t.Errorf("Expected 1 role, got %d", len(info.Roles))
		}
		if len(info.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(info.Conditions))
		}
	})

	t.Run("ProjectWithZeroTimestamp", func(t *testing.T) {
		p := &Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-project",
				CreationTimestamp: metav1.Time{}, // zero time
			},
		}

		info := p.ToProjectInfo()

		if info.CreatedAt != nil {
			t.Error("Expected CreatedAt to be nil for zero timestamp")
		}
	})
}

// TestCRDConstants tests CRD constant values
func TestCRDConstants(t *testing.T) {
	// User CRD constants removed - User CRD is no longer part of the architecture
	// Local users are now stored in ConfigMap/Secret

	// Project CRD constants
	if ProjectGroup != "knodex.io" {
		t.Errorf("ProjectGroup mismatch")
	}
	if ProjectVersion != "v1alpha1" {
		t.Errorf("ProjectVersion mismatch")
	}
	if ProjectResource != "projects" {
		t.Errorf("ProjectResource mismatch")
	}
	if ProjectKind != "Project" {
		t.Errorf("ProjectKind mismatch")
	}

}

// TestConditionConstants tests condition constant values
func TestConditionConstants(t *testing.T) {
	if ProjectConditionReady != "Ready" {
		t.Errorf("ProjectConditionReady mismatch")
	}
	if ProjectConditionValidationError != "ValidationError" {
		t.Errorf("ProjectConditionValidationError mismatch")
	}
	if ProjectConditionSyncError != "SyncError" {
		t.Errorf("ProjectConditionSyncError mismatch")
	}

	if ConditionStatusTrue != "True" {
		t.Errorf("ConditionStatusTrue mismatch")
	}
	if ConditionStatusFalse != "False" {
		t.Errorf("ConditionStatusFalse mismatch")
	}
	if ConditionStatusUnknown != "Unknown" {
		t.Errorf("ConditionStatusUnknown mismatch")
	}
}
