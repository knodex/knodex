package oidc

// TestUser represents a pre-configured test user for OIDC authentication.
type TestUser struct {
	// Email is the user's email address (used in email claim)
	Email string

	// Subject is the unique identifier for the user (used in sub claim)
	Subject string

	// Name is the user's display name (used in name claim)
	Name string

	// Groups contains the OIDC group memberships
	Groups []string

	// EmailVerified indicates whether the email has been verified
	EmailVerified bool

	// Scenario overrides for edge case testing
	ForceExpiredToken  bool // Force this user's tokens to be expired
	ForceInvalidClaims bool // Force invalid claims for this user
	ForceUnverified    bool // Force email_verified to be false
}

// DefaultTestUsers returns the pre-configured test users for E2E tests.
// These users align with the OIDC group mappings configured in the backend.
func DefaultTestUsers() []*TestUser {
	return []*TestUser{
		{
			Email:         "admin@test.local",
			Subject:       "admin-user-id",
			Name:          "Test Admin",
			Groups:        []string{"knodex-admins"},
			EmailVerified: true,
		},
		{
			Email:         "developer@test.local",
			Subject:       "developer-user-id",
			Name:          "Test Developer",
			Groups:        []string{"alpha-developers"},
			EmailVerified: true,
		},
		{
			Email:         "viewer@test.local",
			Subject:       "viewer-user-id",
			Name:          "Test Viewer",
			Groups:        []string{"alpha-viewers"},
			EmailVerified: true,
		},
		{
			Email:         "nogroups@test.local",
			Subject:       "nogroups-user-id",
			Name:          "No Groups User",
			Groups:        []string{},
			EmailVerified: true,
		},
		// Edge case users for testing error scenarios
		{
			Email:             "expired@test.local",
			Subject:           "expired-user-id",
			Name:              "Expired Token User",
			Groups:            []string{"alpha-developers"},
			EmailVerified:     true,
			ForceExpiredToken: true,
		},
		{
			Email:           "unverified@test.local",
			Subject:         "unverified-user-id",
			Name:            "Unverified Email User",
			Groups:          []string{"alpha-developers"},
			EmailVerified:   false,
			ForceUnverified: true,
		},
		{
			Email:              "invalid@test.local",
			Subject:            "invalid-user-id",
			Name:               "Invalid Claims User",
			Groups:             []string{"alpha-developers"},
			EmailVerified:      true,
			ForceInvalidClaims: true,
		},
		// Multi-group users for testing role aggregation
		{
			Email:         "multi@test.local",
			Subject:       "multi-user-id",
			Name:          "Multi Group User",
			Groups:        []string{"alpha-developers", "beta-developers", "alpha-viewers"},
			EmailVerified: true,
		},
		// Platform admin for global admin testing
		{
			Email:         "platform-admin@test.local",
			Subject:       "platform-admin-user-id",
			Name:          "Platform Administrator",
			Groups:        []string{"platform-admins", "knodex-admins"},
			EmailVerified: true,
		},
	}
}

// TestUserEmails contains constants for common test user emails.
const (
	AdminEmail         = "admin@test.local"
	DeveloperEmail     = "developer@test.local"
	ViewerEmail        = "viewer@test.local"
	NoGroupsEmail      = "nogroups@test.local"
	ExpiredEmail       = "expired@test.local"
	UnverifiedEmail    = "unverified@test.local"
	InvalidEmail       = "invalid@test.local"
	MultiEmail         = "multi@test.local"
	PlatformAdminEmail = "platform-admin@test.local"
)

// TestGroups contains constants for common test group names.
const (
	GroupKnodexAdmins    = "knodex-admins"
	GroupPlatformAdmins  = "platform-admins"
	GroupAlphaDevelopers = "alpha-developers"
	GroupAlphaViewers    = "alpha-viewers"
	GroupBetaDevelopers  = "beta-developers"
	GroupTeamADevelopers = "team-a-developers"
	GroupTeamBDevelopers = "team-b-developers"
	GroupViewers         = "viewers"
)
