// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestUserConstants(t *testing.T) {
	// Verify test email constants are defined
	assert.NotEmpty(t, AdminEmail)
	assert.NotEmpty(t, DeveloperEmail)
	assert.NotEmpty(t, ViewerEmail)
	assert.NotEmpty(t, NoGroupsEmail)
	assert.NotEmpty(t, ExpiredEmail)
	assert.NotEmpty(t, UnverifiedEmail)
	assert.NotEmpty(t, InvalidEmail)
	assert.NotEmpty(t, MultiEmail)
	assert.NotEmpty(t, PlatformAdminEmail)

	// Verify group constants are defined
	assert.NotEmpty(t, GroupKnodexAdmins)
	assert.NotEmpty(t, GroupPlatformAdmins)
	assert.NotEmpty(t, GroupAlphaDevelopers)
	assert.NotEmpty(t, GroupAlphaViewers)
}

func TestDefaultTestUsersCount(t *testing.T) {
	users := DefaultTestUsers()
	require.NotEmpty(t, users)

	// Should have at least 9 default users
	assert.GreaterOrEqual(t, len(users), 9)
}

func TestDefaultTestUsersAdminUser(t *testing.T) {
	users := DefaultTestUsers()

	var adminUser *TestUser
	for _, u := range users {
		if u.Email == AdminEmail {
			adminUser = u
			break
		}
	}

	require.NotNil(t, adminUser, "Admin user should exist")
	assert.Equal(t, "admin@test.local", adminUser.Email)
	assert.NotEmpty(t, adminUser.Subject)
	assert.Equal(t, "Test Admin", adminUser.Name)
	assert.Contains(t, adminUser.Groups, GroupKnodexAdmins)
	assert.True(t, adminUser.EmailVerified)
}

func TestDefaultTestUsersDeveloperUser(t *testing.T) {
	users := DefaultTestUsers()

	var devUser *TestUser
	for _, u := range users {
		if u.Email == DeveloperEmail {
			devUser = u
			break
		}
	}

	require.NotNil(t, devUser, "Developer user should exist")
	assert.Equal(t, "developer@test.local", devUser.Email)
	assert.Contains(t, devUser.Groups, GroupAlphaDevelopers)
	assert.True(t, devUser.EmailVerified)
}

func TestDefaultTestUsersViewerUser(t *testing.T) {
	users := DefaultTestUsers()

	var viewerUser *TestUser
	for _, u := range users {
		if u.Email == ViewerEmail {
			viewerUser = u
			break
		}
	}

	require.NotNil(t, viewerUser, "Viewer user should exist")
	assert.Equal(t, "viewer@test.local", viewerUser.Email)
	assert.Contains(t, viewerUser.Groups, GroupAlphaViewers)
	assert.True(t, viewerUser.EmailVerified)
}

func TestDefaultTestUsersNoGroupsUser(t *testing.T) {
	users := DefaultTestUsers()

	var noGroupsUser *TestUser
	for _, u := range users {
		if u.Email == NoGroupsEmail {
			noGroupsUser = u
			break
		}
	}

	require.NotNil(t, noGroupsUser, "NoGroups user should exist")
	assert.Empty(t, noGroupsUser.Groups)
	assert.True(t, noGroupsUser.EmailVerified)
}

func TestDefaultTestUsersExpiredUser(t *testing.T) {
	users := DefaultTestUsers()

	var expiredUser *TestUser
	for _, u := range users {
		if u.Email == ExpiredEmail {
			expiredUser = u
			break
		}
	}

	require.NotNil(t, expiredUser, "Expired user should exist")
	assert.True(t, expiredUser.ForceExpiredToken)
}

func TestDefaultTestUsersUnverifiedUser(t *testing.T) {
	users := DefaultTestUsers()

	var unverifiedUser *TestUser
	for _, u := range users {
		if u.Email == UnverifiedEmail {
			unverifiedUser = u
			break
		}
	}

	require.NotNil(t, unverifiedUser, "Unverified user should exist")
	assert.False(t, unverifiedUser.EmailVerified)
	assert.True(t, unverifiedUser.ForceUnverified)
}

func TestDefaultTestUsersInvalidUser(t *testing.T) {
	users := DefaultTestUsers()

	var invalidUser *TestUser
	for _, u := range users {
		if u.Email == InvalidEmail {
			invalidUser = u
			break
		}
	}

	require.NotNil(t, invalidUser, "Invalid user should exist")
	assert.True(t, invalidUser.ForceInvalidClaims)
}

func TestDefaultTestUsersMultiGroupUser(t *testing.T) {
	users := DefaultTestUsers()

	var multiGroupUser *TestUser
	for _, u := range users {
		if u.Email == MultiEmail {
			multiGroupUser = u
			break
		}
	}

	require.NotNil(t, multiGroupUser, "MultiGroup user should exist")
	assert.GreaterOrEqual(t, len(multiGroupUser.Groups), 2, "Should have multiple groups")
	assert.Contains(t, multiGroupUser.Groups, GroupAlphaDevelopers)
	assert.Contains(t, multiGroupUser.Groups, GroupAlphaViewers)
}

func TestDefaultTestUsersPlatformAdminUser(t *testing.T) {
	users := DefaultTestUsers()

	var platformAdminUser *TestUser
	for _, u := range users {
		if u.Email == PlatformAdminEmail {
			platformAdminUser = u
			break
		}
	}

	require.NotNil(t, platformAdminUser, "PlatformAdmin user should exist")
	assert.Contains(t, platformAdminUser.Groups, GroupPlatformAdmins)
	assert.True(t, platformAdminUser.EmailVerified)
}

func TestTestUserStruct(t *testing.T) {
	user := &TestUser{
		Email:              "test@example.com",
		Subject:            "user-123",
		Name:               "Test User",
		Groups:             []string{"group1", "group2"},
		EmailVerified:      true,
		ForceExpiredToken:  true,
		ForceInvalidClaims: false,
		ForceUnverified:    false,
	}

	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "user-123", user.Subject)
	assert.Equal(t, "Test User", user.Name)
	assert.Len(t, user.Groups, 2)
	assert.True(t, user.EmailVerified)
	assert.True(t, user.ForceExpiredToken)
	assert.False(t, user.ForceInvalidClaims)
	assert.False(t, user.ForceUnverified)
}

func TestDefaultTestUsersUniqueness(t *testing.T) {
	users := DefaultTestUsers()

	// Verify all emails are unique
	emails := make(map[string]bool)
	for _, user := range users {
		assert.False(t, emails[user.Email], "Duplicate email found: %s", user.Email)
		emails[user.Email] = true
	}

	// Verify all subjects are unique
	subjects := make(map[string]bool)
	for _, user := range users {
		assert.False(t, subjects[user.Subject], "Duplicate subject found: %s", user.Subject)
		subjects[user.Subject] = true
	}
}
