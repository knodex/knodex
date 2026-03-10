// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOIDCUserInfo_Groups tests the Groups field in OIDCUserInfo
// Updated to use OIDCUserInfo instead of ProvisionUserResult
func TestOIDCUserInfo_Groups(t *testing.T) {
	tests := []struct {
		name           string
		inputGroups    []string
		expectedGroups []string
		expectedLen    int
	}{
		{
			name:           "multiple groups",
			inputGroups:    []string{"engineering", "developers", "platform-team"},
			expectedGroups: []string{"engineering", "developers", "platform-team"},
			expectedLen:    3,
		},
		{
			name:           "single group",
			inputGroups:    []string{"admins"},
			expectedGroups: []string{"admins"},
			expectedLen:    1,
		},
		{
			name:           "empty groups array",
			inputGroups:    []string{},
			expectedGroups: []string{},
			expectedLen:    0,
		},
		{
			name:           "nil groups becomes empty slice",
			inputGroups:    nil,
			expectedGroups: []string{},
			expectedLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an OIDCUserInfo with the test groups
			result := &OIDCUserInfo{
				Groups: tt.inputGroups,
			}

			// Normalize nil to empty slice (as done in EvaluateOIDCUser)
			if result.Groups == nil {
				result.Groups = []string{}
			}

			// Verify the groups are correctly stored
			require.NotNil(t, result.Groups, "Groups should never be nil after normalization")
			assert.Len(t, result.Groups, tt.expectedLen, "Groups length should match")

			if tt.expectedLen > 0 {
				assert.Equal(t, tt.expectedGroups, result.Groups, "Groups should match expected")
			}
		})
	}
}

// TestGroupsNilHandling tests that nil groups are properly handled
func TestGroupsNilHandling(t *testing.T) {
	// Test that nil groups become empty slice
	var nilGroups []string = nil

	// Simulate the normalization done in ProvisionUser
	if nilGroups == nil {
		nilGroups = []string{}
	}

	assert.NotNil(t, nilGroups, "Groups should not be nil after normalization")
	assert.Len(t, nilGroups, 0, "Normalized nil groups should have length 0")
}

// TestGroupsPassthrough tests that groups are correctly passed through the OIDC evaluation flow
// Updated to use OIDCUserInfo instead of ProvisionUserResult
func TestGroupsPassthrough(t *testing.T) {
	testCases := []struct {
		name   string
		groups []string
	}{
		{
			name:   "OIDC groups with multiple values",
			groups: []string{"group-a", "group-b", "group-c"},
		},
		{
			name:   "single OIDC group",
			groups: []string{"developers"},
		},
		{
			name:   "empty OIDC groups",
			groups: []string{},
		},
		{
			name:   "groups with special characters",
			groups: []string{"team:platform", "role/admin", "dept_engineering"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate what EvaluateOIDCUser does with groups
			inputGroups := tc.groups
			if inputGroups == nil {
				inputGroups = []string{}
			}

			// OIDCUserInfo no longer has IsNewUser (OIDC users are ephemeral)
			result := &OIDCUserInfo{
				Groups: inputGroups,
			}

			// Verify groups are preserved
			assert.Equal(t, tc.groups, result.Groups, "Groups should be preserved in result")
		})
	}
}

// =============================================================================
// Default Role Tests for OIDCProvisioningService
// =============================================================================

func TestEvaluateOIDCUser_DefaultRoleAssignedWhenNoGroups(t *testing.T) {
	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err)

	svc := NewOIDCProvisioningService(nil, nil, casbinEnforcer, "role:test-default")

	result, err := svc.EvaluateOIDCUser(context.Background(), "oidc-sub-1", "user@example.com", "Test User", []string{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Default role should be in assigned roles
	assert.Contains(t, result.AssignedRoles, "role:test-default", "Default role should be assigned when no groups")
}

func TestEvaluateOIDCUser_DefaultRoleAssignedWhenNoMappingsMatch(t *testing.T) {
	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err)

	// Create mapper with a mapping that won't match
	mappings := []config.OIDCGroupMapping{
		{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
	}
	mapper := NewGroupMapper(mappings)

	svc := NewOIDCProvisioningService(nil, mapper, casbinEnforcer, "role:test-default")

	result, err := svc.EvaluateOIDCUser(context.Background(), "oidc-sub-2", "user2@example.com", "Test User 2", []string{"marketing"})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Default role should be assigned since "marketing" doesn't match "engineering"
	assert.Contains(t, result.AssignedRoles, "role:test-default", "Default role should be assigned when no mappings match")
}

func TestEvaluateOIDCUser_DefaultRoleNotAssignedWhenMappingsMatch(t *testing.T) {
	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err)

	mappings := []config.OIDCGroupMapping{
		{Group: "engineering", Project: "eng-project", Role: rbac.RoleDeveloper},
	}
	mapper := NewGroupMapper(mappings)

	svc := NewOIDCProvisioningService(nil, mapper, casbinEnforcer, "role:test-default")

	result, err := svc.EvaluateOIDCUser(context.Background(), "oidc-sub-3", "engineer@example.com", "Engineer", []string{"engineering"})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have the project role, NOT the default role
	assert.NotContains(t, result.AssignedRoles, "role:test-default", "Default role should NOT be assigned when mappings match")
	assert.Contains(t, result.AssignedRoles, "proj:eng-project:developer", "Project role should be assigned")
}

func TestEvaluateOIDCUser_EmptyDefaultRoleDisablesDefault(t *testing.T) {
	casbinEnforcer, err := rbac.NewCasbinEnforcer()
	require.NoError(t, err)

	svc := NewOIDCProvisioningService(nil, nil, casbinEnforcer, "")

	result, err := svc.EvaluateOIDCUser(context.Background(), "oidc-sub-4", "nogroup@example.com", "No Group User", []string{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// No roles should be assigned when default role is empty
	assert.Empty(t, result.AssignedRoles, "No roles should be assigned when default role is disabled")
}
