package rbac

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNewPolicyEnforcer tests PolicyEnforcer creation
func TestNewPolicyEnforcer(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcer(enforcer, nil)
	require.NotNil(t, pe, "PolicyEnforcer should not be nil")
}

// TestPolicyEnforcer_CanAccess tests basic access checks
func TestPolicyEnforcer_CanAccess(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Assign global-admin role to test user
	err := pe.AssignUserRoles(context.Background(), "user:alice", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	tests := []struct {
		name     string
		user     string
		object   string
		action   string
		expected bool
		wantErr  bool
	}{
		// --- Global admin access ---
		{"admin get project", "user:alice", "projects/test", "get", true, false},
		{"admin create project", "user:alice", "projects/test", "create", true, false},
		{"admin delete project", "user:alice", "projects/test", "delete", true, false},
		{"admin get rgd", "user:alice", "rgds/my-rgd", "get", true, false},
		{"admin create instance", "user:alice", "instances/my-instance", "create", true, false},

		// --- No-role access denied ---
		{"no-role get project", "user:bob", "projects/test", "get", false, false},
		{"no-role create instance", "user:bob", "instances/test", "create", false, false},

		// --- Edge cases / errors ---
		{"empty user", "", "projects/test", "get", false, true},
		{"empty object", "user:alice", "", "get", false, true},
		{"empty action", "user:alice", "projects/test", "", false, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowed, err := pe.CanAccess(context.Background(), tt.user, tt.object, tt.action)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_CanAccess_GlobalReadonlyRemoved tests that deprecated global readonly has no access
func TestPolicyEnforcer_CanAccess_GlobalReadonlyRemoved(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Assign deprecated global readonly role - should have NO access
	err := pe.AssignUserRoles(context.Background(), "user:viewer", []string{"role:readonly"})
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		// Deprecated global readonly has NO policies - all access denied
		{"viewer get project denied", "projects/test", "get", false},
		{"viewer list projects denied", "projects/test", "list", false},
		{"viewer get rgd denied", "rgds/my-rgd", "get", false},
		{"viewer list instances denied", "instances/any", "list", false},
		{"viewer create project denied", "projects/test", "create", false},
		{"viewer delete project denied", "projects/test", "delete", false},
		{"viewer update rgd denied", "rgds/my-rgd", "update", false},
		{"viewer delete instance denied", "instances/any", "delete", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowed, err := pe.CanAccess(context.Background(), "user:viewer", tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed, "CanAccess(%s, %s) = %v, want %v",
				tt.object, tt.action, allowed, tt.expected)
		})
	}
}

// TestPolicyEnforcer_EnforceProjectAccess tests project-specific access enforcement
func TestPolicyEnforcer_EnforceProjectAccess(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Assign admin role
	err := pe.AssignUserRoles(context.Background(), "user:admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        string
		projectName string
		action      string
		wantErr     bool
		errContains string
	}{
		// Admin can access projects
		{"admin get project", "user:admin", "my-project", "get", false, ""},
		{"admin create project", "user:admin", "new-project", "create", false, ""},
		{"admin delete project", "user:admin", "old-project", "delete", false, ""},

		// Non-admin cannot access
		{"no-role get project", "user:nobody", "my-project", "get", true, "access denied"},

		// Error cases
		{"empty project name", "user:admin", "", "get", true, "project name cannot be empty"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := pe.EnforceProjectAccess(context.Background(), tt.user, tt.projectName, tt.action)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestPolicyEnforcer_EnforceProjectAccessAllowed tests EnforceProjectAccess for allowed access
func TestPolicyEnforcer_EnforceProjectAccessAllowed(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign global admin role
	err := pe.AssignUserRoles(ctx, "user:project-admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Global admin should be able to access any project
	err = pe.EnforceProjectAccess(ctx, "user:project-admin", "test-project", "get")
	assert.NoError(t, err)

	err = pe.EnforceProjectAccess(ctx, "user:project-admin", "test-project", "update")
	assert.NoError(t, err)

	err = pe.EnforceProjectAccess(ctx, "user:project-admin", "test-project", "delete")
	assert.NoError(t, err)
}

// TestPolicyEnforcer_EnforceProjectAccessDenied tests EnforceProjectAccess for denied access
func TestPolicyEnforcer_EnforceProjectAccessDenied(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// User without any roles should be denied
	err := pe.EnforceProjectAccess(ctx, "user:no-roles", "test-project", "get")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrAccessDenied)
}

// TestPolicyEnforcer_EnforceProjectAccessInvalidInputs tests EnforceProjectAccess with invalid inputs
func TestPolicyEnforcer_EnforceProjectAccessInvalidInputs(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Empty user
	err := pe.EnforceProjectAccess(ctx, "", "test-project", "get")
	assert.Error(t, err)

	// Empty project
	err = pe.EnforceProjectAccess(ctx, "user:test", "", "get")
	assert.Error(t, err)

	// Empty action
	err = pe.EnforceProjectAccess(ctx, "user:test", "test-project", "")
	assert.Error(t, err)
}

// TestPolicyEnforcer_ConcurrentAccess tests thread safety
func TestPolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	pe, mockReader := newTestEnforcerWithMock(t)
	ctx := context.Background()

	// Add initial project
	mockReader.AddProject(&Project{
		ObjectMeta: metav1.ObjectMeta{Name: "concurrent-test"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "reader",
					Policies: []string{"projects/concurrent-test, get, allow"},
				},
			},
		},
	})

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := pe.CanAccess(ctx, "user:test", "projects/concurrent-test", "get")
				if err != nil {
					errChan <- err
				}
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := "user:concurrent" + string(rune('0'+i))
			err := pe.AssignUserRoles(ctx, user, []string{"role:test-viewer"})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	// Concurrent sync
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := pe.SyncPolicies(ctx)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}
