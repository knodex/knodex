// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// TeamGVR is the GroupVersionResource for the namespace-scoped Team CRD.
var TeamGVR = schema.GroupVersionResource{
	Group:    TeamGroup,
	Version:  TeamVersion,
	Resource: TeamResource,
}

// TeamServiceInterface defines the contract for Team CRUD operations.
// Handlers depend on this abstraction (not the concrete *TeamService) so they
// stay unit-testable with a fake. The install namespace is held on the service
// implementation; it does not flow through the interface methods. A Team
// produces NO authorization on its own — it only resolves to OIDC groups when
// a Project role references it (Story 10.2).
type TeamServiceInterface interface {
	CreateTeam(ctx context.Context, name string, spec TeamSpec, createdBy string) (*Team, error)
	GetTeam(ctx context.Context, name string) (*Team, error)
	ListTeams(ctx context.Context) (*TeamList, error)
	UpdateTeam(ctx context.Context, team *Team, updatedBy string) (*Team, error)
	DeleteTeam(ctx context.Context, name string) error
}

// TeamService provides CRUD on Team CRDs scoped to the install namespace via
// the dynamic client. Symmetric with ProjectService — every call goes through
// dynamicClient.Resource(TeamGVR).Namespace(s.namespace). Writes here mutate
// the object; the Story 10.2 TeamWatcher observes the change and triggers the
// debounced policy re-sync. This service adds no second re-sync path (NFR-T1).
type TeamService struct {
	dynamicClient dynamic.Interface
	namespace     string
}

// Ensure TeamService implements TeamServiceInterface.
var _ TeamServiceInterface = (*TeamService)(nil)

// NewTeamService creates a new TeamService scoped to the given install
// namespace. All CRUD operations operate within that namespace only.
func NewTeamService(dynamicClient dynamic.Interface, namespace string) *TeamService {
	return &TeamService{dynamicClient: dynamicClient, namespace: namespace}
}

// CreateTeam creates a new Team CRD in the install namespace after validating
// its name and spec with the Story 10.1 validators.
func (s *TeamService) CreateTeam(ctx context.Context, name string, spec TeamSpec, createdBy string) (*Team, error) {
	if err := ValidateTeamName(name); err != nil {
		return nil, fmt.Errorf("invalid team name: %w", err)
	}
	if err := ValidateTeamSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid team spec: %w", err)
	}

	team := &Team{
		TypeMeta: metav1.TypeMeta{
			APIVersion: TeamGroup + "/" + TeamVersion,
			Kind:       TeamKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "knodex",
			},
			Annotations: map[string]string{
				"knodex.io/created-by": createdBy,
				"knodex.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
		Spec: spec,
	}

	unstructuredTeam, err := runtime.DefaultUnstructuredConverter.ToUnstructured(team)
	if err != nil {
		return nil, fmt.Errorf("failed to convert team to unstructured: %w", err)
	}

	result, err := s.dynamicClient.Resource(TeamGVR).Namespace(s.namespace).Create(
		ctx,
		&unstructured.Unstructured{Object: unstructuredTeam},
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	var createdTeam Team
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &createdTeam); err != nil {
		return nil, fmt.Errorf("failed to convert result to team: %w", err)
	}

	return &createdTeam, nil
}

// GetTeam retrieves a Team by name from the install namespace.
func (s *TeamService) GetTeam(ctx context.Context, name string) (*Team, error) {
	result, err := s.dynamicClient.Resource(TeamGVR).Namespace(s.namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get team %s: %w", name, err)
	}

	var team Team
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &team); err != nil {
		return nil, fmt.Errorf("failed to convert result to team: %w", err)
	}

	return &team, nil
}

// ListTeams lists all Team resources in the install namespace.
func (s *TeamService) ListTeams(ctx context.Context) (*TeamList, error) {
	result, err := s.dynamicClient.Resource(TeamGVR).Namespace(s.namespace).List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	var teamList TeamList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.UnstructuredContent(), &teamList); err != nil {
		return nil, fmt.Errorf("failed to convert result to team list: %w", err)
	}

	return &teamList, nil
}

// UpdateTeam updates an existing Team after re-validating its spec.
func (s *TeamService) UpdateTeam(ctx context.Context, team *Team, updatedBy string) (*Team, error) {
	if err := ValidateTeamSpec(team.Spec); err != nil {
		return nil, fmt.Errorf("invalid team spec: %w", err)
	}

	if team.Annotations == nil {
		team.Annotations = make(map[string]string)
	}
	team.Annotations["knodex.io/updated-by"] = updatedBy
	team.Annotations["knodex.io/updated-at"] = time.Now().Format(time.RFC3339)

	unstructuredTeam, err := runtime.DefaultUnstructuredConverter.ToUnstructured(team)
	if err != nil {
		return nil, fmt.Errorf("failed to convert team to unstructured: %w", err)
	}

	result, err := s.dynamicClient.Resource(TeamGVR).Namespace(s.namespace).Update(
		ctx,
		&unstructured.Unstructured{Object: unstructuredTeam},
		metav1.UpdateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update team %s: %w", team.Name, err)
	}

	var updatedTeam Team
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &updatedTeam); err != nil {
		return nil, fmt.Errorf("failed to convert result to team: %w", err)
	}

	return &updatedTeam, nil
}

// DeleteTeam deletes a Team by name from the install namespace.
func (s *TeamService) DeleteTeam(ctx context.Context, name string) error {
	err := s.dynamicClient.Resource(TeamGVR).Namespace(s.namespace).Delete(
		ctx,
		name,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete team %s: %w", name, err)
	}

	return nil
}
