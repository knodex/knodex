// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTeam_DeepCopyObject_IndependentGroups(t *testing.T) {
	t.Parallel()

	original := &Team{
		TypeMeta:   metav1.TypeMeta{Kind: TeamKind, APIVersion: TeamGroup + "/" + TeamVersion},
		ObjectMeta: metav1.ObjectMeta{Name: "alpha-team"},
		Spec: TeamSpec{
			Description: "alpha",
			OIDCGroups:  []string{"alpha-devs", "alpha-ops"},
		},
		Status: TeamStatus{
			Conditions: []TeamCondition{{Type: TeamConditionReady, Status: ConditionStatusTrue}},
		},
	}

	copied := original.DeepCopyObject().(*Team)

	// Mutating the copy must not affect the original.
	copied.Spec.OIDCGroups[0] = "MUTATED"
	copied.Spec.OIDCGroups = append(copied.Spec.OIDCGroups, "extra")
	copied.Status.Conditions[0].Status = ConditionStatusFalse

	if original.Spec.OIDCGroups[0] != "alpha-devs" {
		t.Errorf("original group mutated: got %q", original.Spec.OIDCGroups[0])
	}
	if len(original.Spec.OIDCGroups) != 2 {
		t.Errorf("original group slice length changed: got %d", len(original.Spec.OIDCGroups))
	}
	if original.Status.Conditions[0].Status != ConditionStatusTrue {
		t.Errorf("original condition mutated: got %q", original.Status.Conditions[0].Status)
	}
	if copied.Name != "alpha-team" {
		t.Errorf("expected copied name 'alpha-team', got %q", copied.Name)
	}
}

func TestTeam_DeepCopyObject_Nil(t *testing.T) {
	t.Parallel()

	var team *Team
	if team.DeepCopyObject() != nil {
		t.Error("expected nil DeepCopyObject for nil Team")
	}

	var list *TeamList
	if list.DeepCopyObject() != nil {
		t.Error("expected nil DeepCopyObject for nil TeamList")
	}
}

func TestTeamList_DeepCopyObject_IndependentItems(t *testing.T) {
	t.Parallel()

	original := &TeamList{
		Items: []Team{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}, Spec: TeamSpec{OIDCGroups: []string{"g1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}, Spec: TeamSpec{OIDCGroups: []string{"g2"}}},
		},
	}

	copied := original.DeepCopyObject().(*TeamList)
	copied.Items[0].Spec.OIDCGroups[0] = "MUTATED"

	if original.Items[0].Spec.OIDCGroups[0] != "g1" {
		t.Errorf("original list item mutated: got %q", original.Items[0].Spec.OIDCGroups[0])
	}
	if len(copied.Items) != 2 {
		t.Errorf("expected 2 copied items, got %d", len(copied.Items))
	}
}
