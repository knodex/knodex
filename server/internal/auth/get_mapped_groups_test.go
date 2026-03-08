package auth

import (
	"errors"
	"testing"

	"github.com/knodex/knodex/server/internal/config"
)

// mockRoleManager implements AuthRoleManager for testing GetMappedGroups
type mockRoleManager struct {
	// rolesMap maps "group:name" subjects to their Casbin roles
	rolesMap map[string][]string
	// errMap maps subjects to errors that should be returned
	errMap map[string]error
}

func (m *mockRoleManager) HasUserRole(user, role string) (bool, error) {
	return false, nil
}

func (m *mockRoleManager) AddUserRole(user, role string) (bool, error) {
	return false, nil
}

func (m *mockRoleManager) GetRolesForUser(user string) ([]string, error) {
	if m.errMap != nil {
		if err, ok := m.errMap[user]; ok {
			return nil, err
		}
	}
	if m.rolesMap != nil {
		if roles, ok := m.rolesMap[user]; ok {
			return roles, nil
		}
	}
	return []string{}, nil
}

func (m *mockRoleManager) Enforce(sub, obj, act string) (bool, error) {
	return false, nil
}

func (m *mockRoleManager) GetPoliciesForRole(role string) ([][]string, error) {
	return nil, nil
}

func TestService_GetMappedGroups(t *testing.T) {
	tests := []struct {
		name           string
		groups         []string
		rolesMap       map[string][]string
		errMap         map[string]error
		nilRoleManager bool
		wantGroups     []string
	}{
		{
			name:   "filters to only mapped groups",
			groups: []string{"engineering", "alpha-developers", "hr-team"},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
			},
			wantGroups: []string{"alpha-developers"},
		},
		{
			name:   "returns empty when no groups are mapped",
			groups: []string{"engineering", "hr-team"},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
			},
			wantGroups: []string{},
		},
		{
			name:   "returns all groups when all are mapped",
			groups: []string{"alpha-developers", "beta-viewers"},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
				"group:beta-viewers":     {"proj:beta:viewer"},
			},
			wantGroups: []string{"alpha-developers", "beta-viewers"},
		},
		{
			name:       "returns empty for nil input",
			groups:     nil,
			wantGroups: []string{},
		},
		{
			name:       "returns empty for empty input",
			groups:     []string{},
			wantGroups: []string{},
		},
		{
			name:           "returns empty when roleManager is nil and no groupMapper",
			groups:         []string{"engineering"},
			nilRoleManager: true,
			wantGroups:     []string{},
		},
		{
			name:   "skips groups with errors and continues",
			groups: []string{"alpha-developers", "broken-group", "beta-viewers"},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
				"group:beta-viewers":     {"proj:beta:viewer"},
			},
			errMap: map[string]error{
				"group:broken-group": errors.New("casbin error"),
			},
			wantGroups: []string{"alpha-developers", "beta-viewers"},
		},
		{
			name:   "uses group: prefix for Casbin lookup",
			groups: []string{"my-team"},
			rolesMap: map[string][]string{
				// Only "group:my-team" should match, not "my-team" directly
				"my-team":       {"role:serveradmin"},
				"group:my-team": {},
			},
			wantGroups: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{}
			if !tt.nilRoleManager {
				svc.roleManager = &mockRoleManager{
					rolesMap: tt.rolesMap,
					errMap:   tt.errMap,
				}
			}

			got, err := svc.GetMappedGroups(tt.groups)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantGroups) {
				t.Fatalf("expected %d groups, got %d: %v", len(tt.wantGroups), len(got), got)
			}
			for i, want := range tt.wantGroups {
				if got[i] != want {
					t.Errorf("group[%d]: expected %q, got %q", i, want, got[i])
				}
			}
		})
	}
}

func TestService_GetMappedGroups_WithGroupMapper(t *testing.T) {
	tests := []struct {
		name           string
		groups         []string
		mappings       []config.OIDCGroupMapping
		rolesMap       map[string][]string
		nilRoleManager bool
		wantGroups     []string
	}{
		{
			name:   "globalAdmin group is visible via GroupMapper",
			groups: []string{"knodex-admins", "hr-team"},
			mappings: []config.OIDCGroupMapping{
				{Group: "knodex-admins", GlobalAdmin: true},
			},
			wantGroups: []string{"knodex-admins"},
		},
		{
			name:   "project-mapped group is visible via GroupMapper",
			groups: []string{"engineering", "random-group"},
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-project", Role: "developer"},
			},
			wantGroups: []string{"engineering"},
		},
		{
			name:   "wildcard mapping matches groups",
			groups: []string{"dev-team-alpha", "dev-team-beta", "hr-team"},
			mappings: []config.OIDCGroupMapping{
				{Group: "dev-*", Project: "dev-project", Role: "developer"},
			},
			wantGroups: []string{"dev-team-alpha", "dev-team-beta"},
		},
		{
			name:   "combines GroupMapper and Casbin results without duplicates",
			groups: []string{"knodex-admins", "alpha-developers", "unmapped-group"},
			mappings: []config.OIDCGroupMapping{
				{Group: "knodex-admins", GlobalAdmin: true},
			},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
			},
			wantGroups: []string{"knodex-admins", "alpha-developers"},
		},
		{
			name:   "group mapped in both sources appears only once",
			groups: []string{"alpha-developers"},
			mappings: []config.OIDCGroupMapping{
				{Group: "alpha-developers", Project: "alpha", Role: "developer"},
			},
			rolesMap: map[string][]string{
				"group:alpha-developers": {"proj:alpha:developer"},
			},
			wantGroups: []string{"alpha-developers"},
		},
		{
			name:           "GroupMapper works even when roleManager is nil",
			groups:         []string{"knodex-admins", "unmapped"},
			nilRoleManager: true,
			mappings: []config.OIDCGroupMapping{
				{Group: "knodex-admins", GlobalAdmin: true},
			},
			wantGroups: []string{"knodex-admins"},
		},
		{
			name:       "no mappings configured returns empty",
			groups:     []string{"some-group"},
			mappings:   []config.OIDCGroupMapping{},
			wantGroups: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{}
			if !tt.nilRoleManager {
				svc.roleManager = &mockRoleManager{
					rolesMap: tt.rolesMap,
				}
			}
			if tt.mappings != nil {
				svc.groupMapper = NewGroupMapper(tt.mappings)
			}

			got, err := svc.GetMappedGroups(tt.groups)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantGroups) {
				t.Fatalf("expected %d groups, got %d: %v", len(tt.wantGroups), len(got), got)
			}
			for i, want := range tt.wantGroups {
				if got[i] != want {
					t.Errorf("group[%d]: expected %q, got %q", i, want, got[i])
				}
			}
		})
	}
}
