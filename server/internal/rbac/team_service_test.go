// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// teamServiceListGVR maps the Team GVR to its list kind so the fake dynamic
// client can serve LIST without panicking.
var teamServiceListGVR = map[schema.GroupVersionResource]string{
	TeamGVR: "TeamList",
}

func newFakeTeamService(t *testing.T) *TeamService {
	t.Helper()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, teamServiceListGVR)
	return NewTeamService(client, testNamespace)
}

// newFakeTeamServiceInNamespace returns a TeamService bound to the given
// namespace, sharing the dynamic client with any peer service so a test can
// assert cross-namespace isolation against the same backing store.
func newFakeTeamServiceInNamespace(t *testing.T, client *dynamicfake.FakeDynamicClient, namespace string) *TeamService {
	t.Helper()
	return NewTeamService(client, namespace)
}

func TestTeamService_CreateGetRoundTrip(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	spec := TeamSpec{Description: "alpha team", OIDCGroups: []string{"alpha-devs", "alpha-ops"}}
	created, err := svc.CreateTeam(ctx, "alpha", spec, "admin@test.local")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if created.Name != "alpha" {
		t.Errorf("expected name alpha, got %s", created.Name)
	}
	if created.Annotations["knodex.io/created-by"] != "admin@test.local" {
		t.Errorf("expected created-by annotation, got %v", created.Annotations)
	}

	got, err := svc.GetTeam(ctx, "alpha")
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if len(got.Spec.OIDCGroups) != 2 || got.Spec.OIDCGroups[0] != "alpha-devs" {
		t.Errorf("groups not round-tripped: %+v", got.Spec.OIDCGroups)
	}
}

// TestTeamService_NamespaceIsolation asserts a TeamService bound to namespace A
// cannot see a Team that lives in namespace B (and vice versa). This is the
// service-layer twin of TestTeamWatcher_NamespaceIsolation — together they
// prove the "two Knodex installs in the same cluster" topology is safe.
func TestTeamService_NamespaceIsolation(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, teamServiceListGVR)
	svcA := newFakeTeamServiceInNamespace(t, client, "knodex-a")
	svcB := newFakeTeamServiceInNamespace(t, client, "knodex-b")
	ctx := context.Background()

	if _, err := svcA.CreateTeam(ctx, "shared-name", TeamSpec{OIDCGroups: []string{"a-grp"}}, "admin-a"); err != nil {
		t.Fatalf("svcA.CreateTeam: %v", err)
	}
	// Same name in B must succeed (per-namespace uniqueness — this would have
	// failed under the previous cluster-scoped model, AC #2).
	if _, err := svcB.CreateTeam(ctx, "shared-name", TeamSpec{OIDCGroups: []string{"b-grp"}}, "admin-b"); err != nil {
		t.Fatalf("svcB.CreateTeam (same name, different namespace): %v", err)
	}

	// A sees its own Team only.
	gotA, err := svcA.GetTeam(ctx, "shared-name")
	if err != nil {
		t.Fatalf("svcA.GetTeam: %v", err)
	}
	if len(gotA.Spec.OIDCGroups) != 1 || gotA.Spec.OIDCGroups[0] != "a-grp" {
		t.Errorf("svcA returned wrong namespace's team: %v", gotA.Spec.OIDCGroups)
	}

	// B sees its own.
	gotB, err := svcB.GetTeam(ctx, "shared-name")
	if err != nil {
		t.Fatalf("svcB.GetTeam: %v", err)
	}
	if len(gotB.Spec.OIDCGroups) != 1 || gotB.Spec.OIDCGroups[0] != "b-grp" {
		t.Errorf("svcB returned wrong namespace's team: %v", gotB.Spec.OIDCGroups)
	}

	// Listing in A returns only A's team.
	listA, err := svcA.ListTeams(ctx)
	if err != nil {
		t.Fatalf("svcA.ListTeams: %v", err)
	}
	if len(listA.Items) != 1 || listA.Items[0].Spec.OIDCGroups[0] != "a-grp" {
		t.Errorf("svcA.ListTeams leaked across namespaces: %+v", listA.Items)
	}
}

func TestTeamService_List(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	for _, n := range []string{"a-team", "b-team"} {
		if _, err := svc.CreateTeam(ctx, n, TeamSpec{OIDCGroups: []string{n + "-grp"}}, "admin"); err != nil {
			t.Fatalf("CreateTeam %s: %v", n, err)
		}
	}

	list, err := svc.ListTeams(ctx)
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(list.Items))
	}
}

func TestTeamService_Update(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	created, err := svc.CreateTeam(ctx, "gamma", TeamSpec{OIDCGroups: []string{"gamma-1"}}, "admin")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	created.Spec.OIDCGroups = []string{"gamma-1", "gamma-2"}
	created.Spec.Description = "updated"
	updated, err := svc.UpdateTeam(ctx, created, "editor@test.local")
	if err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if len(updated.Spec.OIDCGroups) != 2 {
		t.Errorf("expected 2 groups after update, got %d", len(updated.Spec.OIDCGroups))
	}
	if updated.Annotations["knodex.io/updated-by"] != "editor@test.local" {
		t.Errorf("expected updated-by annotation, got %v", updated.Annotations)
	}
}

func TestTeamService_Delete(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	if _, err := svc.CreateTeam(ctx, "delta", TeamSpec{OIDCGroups: []string{"delta-grp"}}, "admin"); err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if err := svc.DeleteTeam(ctx, "delta"); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}
	_, err := svc.GetTeam(ctx, "delta")
	if err == nil {
		t.Fatal("expected error getting deleted team")
	}
}

func TestTeamService_CreateRejectsInvalidName(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	_, err := svc.CreateTeam(ctx, "Invalid_Name", TeamSpec{OIDCGroups: []string{"g"}}, "admin")
	if err == nil {
		t.Fatal("expected invalid-name error before API call")
	}
}

func TestTeamService_CreateRejectsInvalidSpec(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	// No groups → spec validation fails before the API call.
	_, err := svc.CreateTeam(ctx, "empty", TeamSpec{OIDCGroups: nil}, "admin")
	if err == nil {
		t.Fatal("expected invalid-spec error before API call")
	}
}

func TestTeamService_UpdateRejectsInvalidSpec(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	created, err := svc.CreateTeam(ctx, "epsilon", TeamSpec{OIDCGroups: []string{"e-grp"}}, "admin")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	created.Spec.OIDCGroups = nil
	if _, err := svc.UpdateTeam(ctx, created, "admin"); err == nil {
		t.Fatal("expected invalid-spec error on update with no groups")
	}
}

func TestTeamService_GetNotFound(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService(t)
	ctx := context.Background()

	_, err := svc.GetTeam(ctx, "ghost")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}
