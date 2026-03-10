// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGroupMapper(t *testing.T) {
	t.Run("with nil mappings", func(t *testing.T) {
		mapper := NewGroupMapper(nil)
		require.NotNil(t, mapper)
		assert.NotNil(t, mapper.mappings)
		assert.Empty(t, mapper.mappings)
	})

	t.Run("with empty mappings", func(t *testing.T) {
		mapper := NewGroupMapper([]config.OIDCGroupMapping{})
		require.NotNil(t, mapper)
		assert.NotNil(t, mapper.mappings)
		assert.Empty(t, mapper.mappings)
	})

	t.Run("with valid mappings", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
		}
		mapper := NewGroupMapper(mappings)
		require.NotNil(t, mapper)
		assert.Len(t, mapper.mappings, 1)
	})
}

func TestGroupMapper_EvaluateMappings(t *testing.T) {
	tests := []struct {
		name            string
		mappings        []config.OIDCGroupMapping
		groups          []string
		wantProjects    []ProjectMembership // expected project memberships (order-independent)
		wantGlobalRoles []string            // expected global Casbin roles (e.g., ["role:serveradmin"])
	}{
		{
			name: "empty groups returns empty result",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups:          []string{},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name:            "nil groups returns empty result",
			mappings:        []config.OIDCGroupMapping{{Group: "eng", Project: "org1", Role: rbac.RoleDeveloper}},
			groups:          nil,
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name:            "empty mappings returns empty result",
			mappings:        []config.OIDCGroupMapping{},
			groups:          []string{"engineering"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name:            "nil mappings returns empty result",
			mappings:        nil,
			groups:          []string{"engineering"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name: "single group match",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"engineering"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "no matching groups",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups:          []string{"marketing"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name: "multiple groups matching multiple projects",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "platform", Project: "platform-team", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"engineering", "platform"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
				{ProjectID: "platform-team", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "role precedence - developer beats viewer",
			mappings: []config.OIDCGroupMapping{
				{Group: "contractors", Project: "eng-team", Role: rbac.RoleViewer},
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"contractors", "engineering"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "role precedence - platform-admin beats developer",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "team-lead", Project: "eng-team", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"team-lead", "engineering"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "role precedence - platform-admin is highest",
			mappings: []config.OIDCGroupMapping{
				{Group: "viewers", Project: "eng-team", Role: rbac.RoleViewer},
				{Group: "devs", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "admins", Project: "eng-team", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"viewers", "devs", "admins"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "role precedence - order in mappings does not matter",
			mappings: []config.OIDCGroupMapping{
				{Group: "admins", Project: "eng-team", Role: rbac.RolePlatformAdmin},
				{Group: "devs", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "viewers", Project: "eng-team", Role: rbac.RoleViewer},
			},
			groups: []string{"viewers", "devs", "admins"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "globalAdmin mapping adds role:serveradmin to GlobalRoles",
			mappings: []config.OIDCGroupMapping{
				{Group: "kro-admins", GlobalAdmin: true},
			},
			groups:          []string{"kro-admins"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{rbac.CasbinRoleServerAdmin},
		},
		{
			name: "globalAdmin with project mappings - both applied",
			mappings: []config.OIDCGroupMapping{
				{Group: "kro-admins", GlobalAdmin: true},
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"kro-admins", "engineering"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{rbac.CasbinRoleServerAdmin},
		},
		{
			name: "case sensitive matching - exact match required",
			mappings: []config.OIDCGroupMapping{
				{Group: "Engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups:          []string{"engineering"}, // lowercase
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name: "case sensitive matching - exact case matches",
			mappings: []config.OIDCGroupMapping{
				{Group: "Engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"Engineering"}, // exact match
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "user has extra groups not in mappings",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"engineering", "hr", "finance", "marketing"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "mapping exists but user not in group",
			mappings: []config.OIDCGroupMapping{
				{Group: "engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "platform", Project: "platform-team", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"engineering"}, // not in "platform"
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "same role from multiple groups to same project",
			mappings: []config.OIDCGroupMapping{
				{Group: "team-a", Project: "shared-team", Role: rbac.RoleDeveloper},
				{Group: "team-b", Project: "shared-team", Role: rbac.RoleDeveloper},
			},
			groups: []string{"team-a", "team-b"},
			wantProjects: []ProjectMembership{
				{ProjectID: "shared-team", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "special characters in group names - exact match",
			mappings: []config.OIDCGroupMapping{
				{Group: "team/engineering", Project: "eng-team", Role: rbac.RoleDeveloper},
				{Group: "org.platform.admins", Project: "platform", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"team/engineering", "org.platform.admins"},
			wantProjects: []ProjectMembership{
				{ProjectID: "eng-team", Role: rbac.RoleDeveloper},
				{ProjectID: "platform", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "empty group name in mapping - empty strings filtered from groups",
			mappings: []config.OIDCGroupMapping{
				{Group: "", Project: "eng-team", Role: rbac.RoleDeveloper}, // invalid config
			},
			groups:          []string{"", "engineering"}, // empty string is filtered out
			wantProjects:    []ProjectMembership{},       // no match - empty strings filtered defensively
			wantGlobalRoles: []string{},
		},
		{
			name: "complex scenario - multiple projects, multiple roles, globalAdmin",
			mappings: []config.OIDCGroupMapping{
				{Group: "global-admins", GlobalAdmin: true},
				{Group: "eng-viewers", Project: "engineering", Role: rbac.RoleViewer},
				{Group: "eng-devs", Project: "engineering", Role: rbac.RoleDeveloper},
				{Group: "platform-admins", Project: "platform", Role: rbac.RolePlatformAdmin},
				{Group: "qa-team", Project: "qa", Role: rbac.RoleDeveloper},
			},
			groups: []string{"global-admins", "eng-viewers", "eng-devs", "platform-admins"},
			wantProjects: []ProjectMembership{
				{ProjectID: "engineering", Role: rbac.RoleDeveloper}, // viewer + developer = developer
				{ProjectID: "platform", Role: rbac.RolePlatformAdmin},
				// qa not included - user not in qa-team
			},
			wantGlobalRoles: []string{rbac.CasbinRoleServerAdmin},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper := NewGroupMapper(tt.mappings)
			result := mapper.EvaluateMappings(tt.groups)

			require.NotNil(t, result)

			// Handle nil vs empty slice comparison for GlobalRoles
			if len(tt.wantGlobalRoles) == 0 {
				assert.Empty(t, result.GlobalRoles, "GlobalRoles should be empty")
			} else {
				assert.Equal(t, tt.wantGlobalRoles, result.GlobalRoles, "GlobalRoles mismatch")
			}

			// Sort both slices for comparison (map iteration order is not deterministic)
			sortMemberships(result.ProjectMemberships)
			sortMemberships(tt.wantProjects)

			assert.Equal(t, tt.wantProjects, result.ProjectMemberships, "ProjectMemberships mismatch")
		})
	}
}

// sortMemberships sorts ProjectMembership slices by ProjectID for consistent comparison
func sortMemberships(memberships []ProjectMembership) {
	sort.Slice(memberships, func(i, j int) bool {
		return memberships[i].ProjectID < memberships[j].ProjectID
	})
}

func TestRoleIsHigher(t *testing.T) {
	tests := []struct {
		role1    string
		role2    string
		expected bool
	}{
		// platform-admin is highest
		{rbac.RolePlatformAdmin, rbac.RoleDeveloper, true},
		{rbac.RolePlatformAdmin, rbac.RoleViewer, true},
		{rbac.RolePlatformAdmin, "", true},

		// developer is middle
		{rbac.RoleDeveloper, rbac.RoleViewer, true},
		{rbac.RoleDeveloper, "", true},
		{rbac.RoleDeveloper, rbac.RolePlatformAdmin, false},

		// viewer is lowest (of valid roles)
		{rbac.RoleViewer, "", true},
		{rbac.RoleViewer, rbac.RoleDeveloper, false},
		{rbac.RoleViewer, rbac.RolePlatformAdmin, false},

		// same role - not higher
		{rbac.RolePlatformAdmin, rbac.RolePlatformAdmin, false},
		{rbac.RoleDeveloper, rbac.RoleDeveloper, false},
		{rbac.RoleViewer, rbac.RoleViewer, false},

		// unknown roles treated as lowest
		{"unknown", rbac.RoleViewer, false},
		{rbac.RoleViewer, "unknown", true},
		{"unknown1", "unknown2", false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s_vs_%s", tt.role1, tt.role2)
		if tt.role1 == "" {
			name = "empty_vs_" + tt.role2
		}
		if tt.role2 == "" {
			name = tt.role1 + "_vs_empty"
		}

		t.Run(name, func(t *testing.T) {
			result := roleIsHigher(tt.role1, tt.role2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroupMapper_Performance(t *testing.T) {
	// Generate 100 mappings
	mappings := make([]config.OIDCGroupMapping, 100)
	for i := 0; i < 100; i++ {
		mappings[i] = config.OIDCGroupMapping{
			Group:   fmt.Sprintf("group-%d", i),
			Project: fmt.Sprintf("proj-%d", i),
			Role:    rbac.RoleDeveloper,
		}
	}

	// Generate 100 groups (half matching, half not)
	groups := make([]string, 100)
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			groups[i] = fmt.Sprintf("group-%d", i) // matches
		} else {
			groups[i] = fmt.Sprintf("other-%d", i) // no match
		}
	}

	mapper := NewGroupMapper(mappings)

	// Warm up
	_ = mapper.EvaluateMappings(groups)

	// Measure performance
	start := time.Now()
	result := mapper.EvaluateMappings(groups)
	elapsed := time.Since(start)

	// Verify correctness
	assert.Equal(t, 50, len(result.ProjectMemberships), "Expected 50 matching projects (every even index)")
	assert.Empty(t, result.GlobalRoles)

	// Assert performance requirement: <100ms
	assert.Less(t, elapsed, 100*time.Millisecond,
		"Evaluation took %v, expected <100ms for 100 groups x 100 mappings", elapsed)

	// Log actual performance for visibility
	t.Logf("Performance: 100 groups x 100 mappings = %v", elapsed)
}

func TestGroupMapper_PerformanceLargeScale(t *testing.T) {
	// Test with larger scale to ensure O(n+m) complexity
	// 500 groups x 500 mappings should still be fast

	mappings := make([]config.OIDCGroupMapping, 500)
	for i := 0; i < 500; i++ {
		mappings[i] = config.OIDCGroupMapping{
			Group:   fmt.Sprintf("group-%d", i),
			Project: fmt.Sprintf("proj-%d", i),
			Role:    rbac.RoleDeveloper,
		}
	}

	groups := make([]string, 500)
	for i := 0; i < 500; i++ {
		groups[i] = fmt.Sprintf("group-%d", i) // all match
	}

	mapper := NewGroupMapper(mappings)

	start := time.Now()
	result := mapper.EvaluateMappings(groups)
	elapsed := time.Since(start)

	assert.Equal(t, 500, len(result.ProjectMemberships))

	// Still should be fast (under 100ms) due to O(n+m) complexity
	assert.Less(t, elapsed, 100*time.Millisecond,
		"Large scale test took %v, expected <100ms", elapsed)

	t.Logf("Large scale performance: 500 groups x 500 mappings = %v", elapsed)
}

func TestGroupMapper_ResultImmutability(t *testing.T) {
	// Ensure result can be modified without affecting mapper state
	mappings := []config.OIDCGroupMapping{
		{Group: "eng", Project: "eng-team", Role: rbac.RoleDeveloper},
	}
	groups := []string{"eng"}

	mapper := NewGroupMapper(mappings)

	result1 := mapper.EvaluateMappings(groups)
	result1.ProjectMemberships = append(result1.ProjectMemberships, ProjectMembership{ProjectID: "fake", Role: "fake"})
	result1.GlobalRoles = append(result1.GlobalRoles, "fake-role")

	result2 := mapper.EvaluateMappings(groups)

	assert.Len(t, result2.ProjectMemberships, 1, "Modifying result1 should not affect result2")
	assert.Empty(t, result2.GlobalRoles, "Modifying result1 should not affect result2")
}

func TestGroupMapper_EmptyStrings(t *testing.T) {
	// Test edge cases with empty strings

	t.Run("empty strings in groups are filtered out", func(t *testing.T) {
		// Empty strings in groups slice are filtered as a security measure
		mappings := []config.OIDCGroupMapping{
			{Group: "", Project: "org", Role: rbac.RoleDeveloper},
		}
		groups := []string{""}

		mapper := NewGroupMapper(mappings)
		result := mapper.EvaluateMappings(groups)

		// Empty strings are filtered - no match possible
		assert.Len(t, result.ProjectMemberships, 0)
	})

	t.Run("whitespace group names are distinct", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: " ", Project: "org1", Role: rbac.RoleDeveloper},
			{Group: "  ", Project: "org2", Role: rbac.RoleViewer},
		}
		groups := []string{" ", "  "}

		mapper := NewGroupMapper(mappings)
		result := mapper.EvaluateMappings(groups)

		assert.Len(t, result.ProjectMemberships, 2)
	})
}

func TestGroupMapper_UnknownRoleHandling(t *testing.T) {
	// Test that unknown roles are skipped as a defensive measure

	t.Run("unknown role is skipped", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineers", Project: "eng-team", Role: "unknown-role"},
		}
		groups := []string{"engineers"}

		mapper := NewGroupMapper(mappings)
		result := mapper.EvaluateMappings(groups)

		// Unknown role should be skipped
		assert.Len(t, result.ProjectMemberships, 0)
	})

	t.Run("unknown role mixed with valid roles", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineers", Project: "eng-team", Role: "invalid"},
			{Group: "admins", Project: "admin-team", Role: rbac.RolePlatformAdmin},
		}
		groups := []string{"engineers", "admins"}

		mapper := NewGroupMapper(mappings)
		result := mapper.EvaluateMappings(groups)

		// Only valid role should be included
		assert.Len(t, result.ProjectMemberships, 1)
		assert.Equal(t, "admin-team", result.ProjectMemberships[0].ProjectID)
		assert.Equal(t, rbac.RolePlatformAdmin, result.ProjectMemberships[0].Role)
	})

	t.Run("empty role string is skipped", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "engineers", Project: "eng-team", Role: ""},
		}
		groups := []string{"engineers"}

		mapper := NewGroupMapper(mappings)
		result := mapper.EvaluateMappings(groups)

		// Empty role should be skipped
		assert.Len(t, result.ProjectMemberships, 0)
	})
}

func TestGroupMapper_WildcardMatching(t *testing.T) {
	tests := []struct {
		name            string
		mappings        []config.OIDCGroupMapping
		groups          []string
		wantProjects    []ProjectMembership
		wantGlobalRoles []string
	}{
		{
			name: "asterisk wildcard matches prefix",
			mappings: []config.OIDCGroupMapping{
				{Group: "dev-*", Project: "dev-project", Role: rbac.RoleDeveloper},
			},
			groups: []string{"dev-team1", "dev-team2", "prod-team"},
			wantProjects: []ProjectMembership{
				{ProjectID: "dev-project", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "asterisk wildcard matches suffix",
			mappings: []config.OIDCGroupMapping{
				{Group: "*-admin", Project: "admin-project", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"platform-admin", "not-an-admin-group", "team-admin"},
			wantProjects: []ProjectMembership{
				{ProjectID: "admin-project", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "asterisk wildcard matches any in middle",
			mappings: []config.OIDCGroupMapping{
				{Group: "team-*-dev", Project: "devs", Role: rbac.RoleDeveloper},
			},
			groups: []string{"team-platform-dev", "team-frontend-dev", "team-backend-prod"},
			wantProjects: []ProjectMembership{
				{ProjectID: "devs", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "question mark matches single character",
			mappings: []config.OIDCGroupMapping{
				{Group: "team-?", Project: "teams", Role: rbac.RoleViewer},
			},
			groups: []string{"team-a", "team-b", "team-ab"}, // team-ab should NOT match
			wantProjects: []ProjectMembership{
				{ProjectID: "teams", Role: rbac.RoleViewer},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "character class matches set",
			mappings: []config.OIDCGroupMapping{
				{Group: "team-[abc]", Project: "special-teams", Role: rbac.RoleDeveloper},
			},
			groups: []string{"team-a", "team-d", "team-c"},
			wantProjects: []ProjectMembership{
				{ProjectID: "special-teams", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "character range matches",
			mappings: []config.OIDCGroupMapping{
				{Group: "dev-team[1-5]", Project: "dev-teams", Role: rbac.RoleDeveloper},
			},
			groups: []string{"dev-team1", "dev-team3", "dev-team7"},
			wantProjects: []ProjectMembership{
				{ProjectID: "dev-teams", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "wildcard globalAdmin",
			mappings: []config.OIDCGroupMapping{
				{Group: "*-global-admin", GlobalAdmin: true},
			},
			groups:          []string{"kro-global-admin", "regular-user"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{rbac.CasbinRoleServerAdmin},
		},
		{
			name: "wildcard no match returns empty",
			mappings: []config.OIDCGroupMapping{
				{Group: "prefix-*", Project: "project", Role: rbac.RoleDeveloper},
			},
			groups:          []string{"other-team", "different-group"},
			wantProjects:    []ProjectMembership{},
			wantGlobalRoles: []string{},
		},
		{
			name: "wildcard and exact match combined",
			mappings: []config.OIDCGroupMapping{
				{Group: "dev-*", Project: "dev-project", Role: rbac.RoleViewer},
				{Group: "dev-admins", Project: "dev-project", Role: rbac.RolePlatformAdmin},
			},
			groups: []string{"dev-admins"}, // matches both, higher role wins
			wantProjects: []ProjectMembership{
				{ProjectID: "dev-project", Role: rbac.RolePlatformAdmin},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "multiple wildcard patterns",
			mappings: []config.OIDCGroupMapping{
				{Group: "dev-*", Project: "dev-env", Role: rbac.RoleDeveloper},
				{Group: "*-admin", Project: "admin-env", Role: rbac.RolePlatformAdmin},
				{Group: "qa-*", Project: "qa-env", Role: rbac.RoleViewer},
			},
			groups: []string{"dev-frontend", "platform-admin", "marketing"},
			wantProjects: []ProjectMembership{
				{ProjectID: "admin-env", Role: rbac.RolePlatformAdmin},
				{ProjectID: "dev-env", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "asterisk matches empty string",
			mappings: []config.OIDCGroupMapping{
				{Group: "team*", Project: "teams", Role: rbac.RoleDeveloper},
			},
			groups: []string{"team"}, // matches "team" with empty suffix
			wantProjects: []ProjectMembership{
				{ProjectID: "teams", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
		{
			name: "complex pattern matches",
			mappings: []config.OIDCGroupMapping{
				{Group: "[a-z]*-team-[0-9]", Project: "numbered-teams", Role: rbac.RoleDeveloper},
			},
			groups: []string{"dev-team-1", "platform-team-5", "UPPER-team-1"},
			wantProjects: []ProjectMembership{
				{ProjectID: "numbered-teams", Role: rbac.RoleDeveloper},
			},
			wantGlobalRoles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper := NewGroupMapper(tt.mappings)
			result := mapper.EvaluateMappings(tt.groups)

			require.NotNil(t, result)

			// Handle nil vs empty slice comparison for GlobalRoles
			if len(tt.wantGlobalRoles) == 0 {
				assert.Empty(t, result.GlobalRoles, "GlobalRoles should be empty")
			} else {
				assert.Equal(t, tt.wantGlobalRoles, result.GlobalRoles, "GlobalRoles mismatch")
			}

			sortMemberships(result.ProjectMemberships)
			sortMemberships(tt.wantProjects)

			assert.Equal(t, tt.wantProjects, result.ProjectMemberships, "ProjectMemberships mismatch")
		})
	}
}

func TestIsWildcardPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		{"engineering", false},
		{"exact-match", false},
		{"team.name", false},
		{"team/subteam", false},

		// Wildcard patterns
		{"dev-*", true},
		{"*-admin", true},
		{"team-*-dev", true},
		{"team-?", true},
		{"team-[abc]", true},
		{"team-[1-9]", true},
		{"[a-z]*-team", true},
		{"*", true},
		{"?", true},
		{"[abc]", true},
		{"***", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := isWildcardPattern(tt.pattern)
			assert.Equal(t, tt.expected, result, "isWildcardPattern(%q) = %v, want %v", tt.pattern, result, tt.expected)
		})
	}
}

func TestGroupMapper_WildcardPerformance(t *testing.T) {
	// Test performance with wildcard patterns
	// Wildcard patterns should still be fast enough

	// Create mappings with various wildcard patterns
	mappings := make([]config.OIDCGroupMapping, 50)
	for i := 0; i < 25; i++ {
		// Exact matches
		mappings[i] = config.OIDCGroupMapping{
			Group:   fmt.Sprintf("exact-group-%d", i),
			Project: fmt.Sprintf("proj-%d", i),
			Role:    rbac.RoleDeveloper,
		}
	}
	for i := 25; i < 50; i++ {
		// Wildcard patterns
		mappings[i] = config.OIDCGroupMapping{
			Group:   fmt.Sprintf("team-%d-*", i),
			Project: fmt.Sprintf("proj-%d", i),
			Role:    rbac.RoleViewer,
		}
	}

	// Create 100 groups with mix of matching and non-matching
	groups := make([]string, 100)
	for i := 0; i < 100; i++ {
		if i%4 == 0 {
			groups[i] = fmt.Sprintf("exact-group-%d", i/4) // matches exact
		} else if i%4 == 1 {
			groups[i] = fmt.Sprintf("team-%d-frontend", 25+i/4) // matches wildcard
		} else {
			groups[i] = fmt.Sprintf("nomatch-%d", i)
		}
	}

	mapper := NewGroupMapper(mappings)

	start := time.Now()
	result := mapper.EvaluateMappings(groups)
	elapsed := time.Since(start)

	// Verify we got some matches
	assert.Greater(t, len(result.ProjectMemberships), 0, "Should have some matches")

	// Performance: should complete quickly even with wildcards
	assert.Less(t, elapsed, 100*time.Millisecond,
		"Wildcard evaluation took %v, expected <100ms", elapsed)

	t.Logf("Wildcard performance: 100 groups x 50 mappings (25 wildcard) = %v", elapsed)
}

func TestGroupMapper_DeterministicOrdering(t *testing.T) {
	// Test that ProjectMemberships are sorted by ProjectID for stable output

	t.Run("results are sorted by ProjectID", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "z-team", Project: "z-project", Role: rbac.RoleViewer},
			{Group: "a-team", Project: "a-project", Role: rbac.RoleDeveloper},
			{Group: "m-team", Project: "m-project", Role: rbac.RolePlatformAdmin},
		}
		groups := []string{"z-team", "a-team", "m-team"}

		mapper := NewGroupMapper(mappings)

		// Run multiple times to ensure deterministic ordering
		for i := 0; i < 10; i++ {
			result := mapper.EvaluateMappings(groups)

			require.Len(t, result.ProjectMemberships, 3)
			// Should always be sorted alphabetically by ProjectID
			assert.Equal(t, "a-project", result.ProjectMemberships[0].ProjectID)
			assert.Equal(t, "m-project", result.ProjectMemberships[1].ProjectID)
			assert.Equal(t, "z-project", result.ProjectMemberships[2].ProjectID)
		}
	})

	t.Run("consistent ordering across evaluations", func(t *testing.T) {
		mappings := []config.OIDCGroupMapping{
			{Group: "group1", Project: "proj-c", Role: rbac.RoleViewer},
			{Group: "group2", Project: "proj-a", Role: rbac.RoleDeveloper},
			{Group: "group3", Project: "proj-b", Role: rbac.RolePlatformAdmin},
		}
		groups := []string{"group1", "group2", "group3"}

		mapper := NewGroupMapper(mappings)
		result1 := mapper.EvaluateMappings(groups)
		result2 := mapper.EvaluateMappings(groups)

		// Results should be identical
		assert.Equal(t, result1.ProjectMemberships, result2.ProjectMemberships)
	})
}
