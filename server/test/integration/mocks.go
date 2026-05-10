// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/rbac"
)

// MockPolicyEnforcer is a mock implementation of rbac.PolicyEnforcer for testing
type MockPolicyEnforcer struct {
	mu sync.RWMutex

	// allowedAccess tracks permissions as user -> object:action -> allowed
	allowedAccess map[string]map[string]bool

	// userRoles tracks role assignments as user -> []roles
	userRoles map[string][]string

	// loadedPolicies tracks policies loaded from projects
	loadedPolicies map[string][]string // projectName -> policies

	// knownProjects tracks projects for GetAccessibleProjects wildcard returns
	knownProjects []string

	// denyAll when true, denies all access checks
	denyAll bool

	// allowAll when true, allows all access checks (overrides denyAll)
	allowAll bool
}

// NewMockPolicyEnforcer creates a new MockPolicyEnforcer
func NewMockPolicyEnforcer() *MockPolicyEnforcer {
	return &MockPolicyEnforcer{
		allowedAccess:  make(map[string]map[string]bool),
		userRoles:      make(map[string][]string),
		loadedPolicies: make(map[string][]string),
	}
}

// Allow grants a user permission to perform an action on an object
func (m *MockPolicyEnforcer) Allow(user, object, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", object, action)
	if m.allowedAccess[user] == nil {
		m.allowedAccess[user] = make(map[string]bool)
	}
	m.allowedAccess[user][key] = true
}

// Deny revokes a user's permission to perform an action on an object
func (m *MockPolicyEnforcer) Deny(user, object, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", object, action)
	if m.allowedAccess[user] != nil {
		delete(m.allowedAccess[user], key)
	}
}

// SetDenyAll sets the enforcer to deny all access checks
func (m *MockPolicyEnforcer) SetDenyAll(denyAll bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.denyAll = denyAll
}

// SetAllowAll sets the enforcer to allow all access checks
func (m *MockPolicyEnforcer) SetAllowAll(allowAll bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowAll = allowAll
}

// Reset clears all mock state
func (m *MockPolicyEnforcer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowedAccess = make(map[string]map[string]bool)
	m.userRoles = make(map[string][]string)
	m.loadedPolicies = make(map[string][]string)
	m.knownProjects = nil
	m.denyAll = false
	m.allowAll = false
}

// RegisterProject adds a project to the list of known projects
// This is used by GetAccessibleProjects when wildcard permissions are granted
func (m *MockPolicyEnforcer) RegisterProject(projectName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.knownProjects = append(m.knownProjects, projectName)
}

// CanAccess implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.allowAll {
		return true, nil
	}
	if m.denyAll {
		return false, nil
	}

	if permissions, ok := m.allowedAccess[user]; ok {

		// If user has "*:*" permission, they can access everything
		if allowed, exists := permissions["*:*"]; exists && allowed {
			return true, nil
		}

		// Check for exact match
		key := fmt.Sprintf("%s:%s", object, action)
		if allowed, exists := permissions[key]; exists {
			return allowed, nil
		}
	}
	return false, nil
}

// CanAccessWithGroups implements rbac.PolicyEnforcer
// For mock purposes, this checks user permission first, then checks each group as a user
func (m *MockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	// First check direct user permission
	allowed, err := m.CanAccess(ctx, user, object, action)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Then check each group (in mock, we treat group names as user names for simplicity)
	for _, group := range groups {
		if group == "" {
			continue
		}
		groupSubject := fmt.Sprintf("group:%s", group)
		allowed, err = m.CanAccess(ctx, groupSubject, object, action)
		if err != nil {
			continue
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// EnforceProjectAccess implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	allowed, err := m.CanAccess(ctx, user, fmt.Sprintf("project/%s", projectName), action)
	if err != nil {
		return err
	}
	if !allowed {
		return rbac.ErrAccessDenied
	}
	return nil
}

// LoadProjectPolicies implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policies := []string{}
	for _, role := range project.Spec.Roles {
		policies = append(policies, role.Policies...)
	}
	m.loadedPolicies[project.Name] = policies
	return nil
}

// SyncPolicies implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

// AssignUserRoles implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userRoles[user] = roles
	return nil
}

// GetUserRoles implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if roles, ok := m.userRoles[user]; ok {
		return roles, nil
	}
	return []string{}, nil
}

// HasRole implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if roles, ok := m.userRoles[user]; ok {
		for _, r := range roles {
			if r == role {
				return true, nil
			}
		}
	}
	return false, nil
}

// RemoveUserRoles implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.userRoles, user)
	return nil
}

// RemoveUserRole implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if roles, ok := m.userRoles[user]; ok {
		newRoles := []string{}
		for _, r := range roles {
			if r != role {
				newRoles = append(newRoles, r)
			}
		}
		m.userRoles[user] = newRoles
	}
	return nil
}

// RestorePersistedRoles implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	// No-op for mock
	return nil
}

// RemoveProjectPolicies implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.loadedPolicies, projectName)
	return nil
}

// InvalidateCache implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) InvalidateCache() {
	// No-op for mock
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) InvalidateCacheForUser(user string) int {
	// No-op for mock - returns 0 entries invalidated
	return 0
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	// No-op for mock - returns 0 entries invalidated
	return 0
}

// CacheStats implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) CacheStats() rbac.CacheStats {
	return rbac.CacheStats{}
}

// Metrics implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) Metrics() rbac.PolicyMetrics {
	return rbac.PolicyMetrics{}
}

// IncrementPolicyReloads implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) IncrementPolicyReloads() {
	// No-op for mock
}

// IncrementBackgroundSyncs implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) IncrementBackgroundSyncs() {
	// No-op for mock
}

// IncrementWatcherRestarts implements rbac.PolicyEnforcer
func (m *MockPolicyEnforcer) IncrementWatcherRestarts() {
	// No-op for mock
}

// GetAccessibleProjects implements rbac.PolicyEnforcer (unified approach)
// For mock purposes, this returns an empty list or uses allowAll to return a predefined list
func (m *MockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Determine what to return for wildcard access
	// If knownProjects is set, use that; otherwise fall back to default test projects
	wildcardProjects := m.knownProjects
	if len(wildcardProjects) == 0 {
		wildcardProjects = []string{"proj-alpha-team", "proj-beta-team", "proj-development"}
	}

	// If allowAll is set, return known projects
	if m.allowAll {
		return wildcardProjects, nil
	}

	// For more specific testing, check which projects the user has access to
	// by looking at permissions in the form "projects/{name}:get"
	accessibleProjects := []string{}
	if permissions, ok := m.allowedAccess[user]; ok {
		// Check for wildcard permission
		if allowed, exists := permissions["*:*"]; exists && allowed {
			return wildcardProjects, nil
		}
		if allowed, exists := permissions["projects/*:get"]; exists && allowed {
			return wildcardProjects, nil
		}

		// Check individual project permissions
		for key, allowed := range permissions {
			if allowed && len(key) > 9 && key[:9] == "projects/" {
				// Extract project name from "projects/{name}:get"
				rest := key[9:]
				colonIdx := -1
				for i, c := range rest {
					if c == ':' {
						colonIdx = i
						break
					}
				}
				if colonIdx > 0 {
					projectName := rest[:colonIdx]
					accessibleProjects = append(accessibleProjects, projectName)
				}
			}
		}
	}

	return accessibleProjects, nil
}

// GetLoadedPolicies returns policies loaded for a project (test helper)
func (m *MockPolicyEnforcer) GetLoadedPolicies(projectName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loadedPolicies[projectName]
}

// MockAuthService is a mock implementation of auth.ServiceInterface for testing
type MockAuthService struct {
	mu sync.RWMutex

	// validTokens maps token string to JWTClaims
	validTokens map[string]*auth.JWTClaims

	// jwtSecret for generating test tokens
	jwtSecret string

	// LocalLoginEnabled controls the value returned by IsLocalLoginEnabled.
	// Defaults to false to preserve historical behavior — set explicitly when
	// a test needs to exercise the local-login pathway.
	LocalLoginEnabled bool
}

// NewMockAuthService creates a new MockAuthService
func NewMockAuthService(jwtSecret string) *MockAuthService {
	return &MockAuthService{
		validTokens: make(map[string]*auth.JWTClaims),
		jwtSecret:   jwtSecret,
	}
}

// AddValidToken registers a token as valid with the given claims
func (m *MockAuthService) AddValidToken(token string, claims *auth.JWTClaims) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validTokens[token] = claims
}

// RemoveValidToken removes a token from the valid tokens list
func (m *MockAuthService) RemoveValidToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.validTokens, token)
}

// Reset clears all mock state
func (m *MockAuthService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validTokens = make(map[string]*auth.JWTClaims)
}

// AuthenticateLocal implements auth.ServiceInterface
func (m *MockAuthService) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
	return nil, fmt.Errorf("mock: AuthenticateLocal not implemented")
}

// GenerateTokenForAccount implements auth.ServiceInterface
func (m *MockAuthService) GenerateTokenForAccount(account *auth.Account, userID string) (string, time.Time, error) {
	return "", time.Time{}, fmt.Errorf("mock: GenerateTokenForAccount not implemented")
}

// GenerateTokenWithGroups implements auth.ServiceInterface
func (m *MockAuthService) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	return "", time.Time{}, fmt.Errorf("mock: GenerateTokenWithGroups not implemented")
}

// ValidateToken implements auth.ServiceInterface
func (m *MockAuthService) ValidateToken(_ context.Context, tokenString string) (*auth.JWTClaims, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if claims, ok := m.validTokens[tokenString]; ok {
		// Check if token is expired
		if claims.ExpiresAt < time.Now().Unix() {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// RevokeToken implements auth.ServiceInterface
func (m *MockAuthService) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

// IsLocalLoginEnabled implements auth.ServiceInterface. Returns the
// configurable LocalLoginEnabled field (default false).
func (m *MockAuthService) IsLocalLoginEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LocalLoginEnabled
}

// CreateTestClaims creates JWT claims for testing.
// The addAdminRole parameter sets casbin_roles=["role:serveradmin"] in JWT as a UI display hint.
// Actual authorization enforcement uses server-side Casbin policies, not JWT claims (STORY-228).
func CreateTestClaims(userID, email, displayName string, projects []string, defaultProject string, addAdminRole bool) *auth.JWTClaims {
	now := time.Now()
	var casbinRoles []string
	if addAdminRole {
		casbinRoles = []string{"role:serveradmin"}
	}
	return &auth.JWTClaims{
		UserID:         userID,
		Email:          email,
		DisplayName:    displayName,
		Projects:       projects,
		DefaultProject: defaultProject,
		CasbinRoles:    casbinRoles,
		ExpiresAt:      now.Add(1 * time.Hour).Unix(),
		IssuedAt:       now.Unix(),
	}
}

// Ensure mocks implement the interfaces
var _ rbac.PolicyEnforcer = (*MockPolicyEnforcer)(nil)
var _ auth.ServiceInterface = (*MockAuthService)(nil)
