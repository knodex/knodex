//go:build integration

package rbac_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/provops-org/knodex/server/internal/rbac"
)

var (
	// Global test clients initialized in TestMain
	testK8sClient     kubernetes.Interface
	testDynamicClient dynamic.Interface
	testProjectSvc    *rbac.ProjectService
)

const (
	// testCreatedBy identifies resources created by integration tests
	testCreatedBy = "integration-test"
	// testPrefix ensures test resources are identifiable
	testPrefix = "inttest-"
)

// TestMain sets up integration test environment
func TestMain(m *testing.M) {
	// Check if running in integration test mode
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		fmt.Println("Skipping integration tests. Set INTEGRATION_TESTS=true to run.")
		os.Exit(0)
	}

	// Load kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Failed to load kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create k8s client
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create dynamic client: %v\n", err)
		os.Exit(1)
	}

	testK8sClient = k8sClient
	testDynamicClient = dynamicClient
	testProjectSvc = rbac.NewProjectService(k8sClient, dynamicClient)

	// Verify CRD is available
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = testProjectSvc.ListProjects(ctx)
	if err != nil {
		fmt.Printf("Failed to verify CRD availability (list projects): %v\n", err)
		fmt.Println("Ensure the Project CRD is installed: kubectl apply -k deploy/crds/")
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup: Delete all test Projects
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cleanupCancel()

	cleanupTestProjects(cleanupCtx)

	os.Exit(code)
}

// cleanupTestProjects removes all projects created during integration tests
func cleanupTestProjects(ctx context.Context) {
	projects, err := testProjectSvc.ListProjects(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to list projects for cleanup: %v\n", err)
		return
	}

	for _, proj := range projects.Items {
		if strings.HasPrefix(proj.Name, testPrefix) {
			_ = testProjectSvc.DeleteProject(ctx, proj.Name)
		}
	}
}

// testProjectName generates a unique test project name
func testProjectName() string {
	return fmt.Sprintf("%s%s", testPrefix, uuid.New().String()[:8])
}

// newTestProjectSpec creates a test ProjectSpec
func newTestProjectSpec(description string) rbac.ProjectSpec {
	return rbac.ProjectSpec{
		Description: description,
		Destinations: []rbac.Destination{
			{
				Namespace: "default",
			},
		},
		Roles: []rbac.ProjectRole{
			{
				Name:        "admin",
				Description: "Administrator role for testing",
				Policies: []string{
					"p, proj:test:admin, applications, *, test/*, allow",
				},
			},
		},
	}
}

// cleanupProject is a helper to delete a project (used with defer)
func cleanupProject(t *testing.T, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := testProjectSvc.DeleteProject(ctx, name)
	if err != nil && !errors.IsNotFound(err) {
		t.Logf("Warning: failed to cleanup project %s: %v", name, err)
	}
}

// =============================================================================
// CREATE INTEGRATION TESTS
// =============================================================================

func TestProjectService_CreateProject_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		spec := newTestProjectSpec("Test project for integration tests")

		// Create
		created, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)
		require.NotNil(t, created)

		// Verify returned project
		assert.Equal(t, name, created.Name)
		assert.Equal(t, "Test project for integration tests", created.Spec.Description)

		// Verify in Kubernetes
		retrieved, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, name, retrieved.Name)
		assert.Equal(t, "Test project for integration tests", retrieved.Spec.Description)

		// Verify Kubernetes metadata populated
		assert.NotEmpty(t, retrieved.UID, "UID should be populated by Kubernetes")
		assert.NotEmpty(t, retrieved.ResourceVersion, "ResourceVersion should be populated")
		assert.False(t, retrieved.CreationTimestamp.IsZero(), "CreationTimestamp should be set")
	})

	t.Run("duplicate creation returns AlreadyExists error", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		spec := newTestProjectSpec("First project")

		// Create first time
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Try to create again with same name
		_, err = testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		assert.Error(t, err)
		assert.True(t, errors.IsAlreadyExists(err), "Expected AlreadyExists error, got: %v", err)
	})

	t.Run("invalid name returns validation error", func(t *testing.T) {
		// Invalid name with special characters
		invalidName := "Invalid_Name!"
		spec := newTestProjectSpec("Invalid project")

		_, err := testProjectSvc.CreateProject(ctx, invalidName, spec, testCreatedBy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid project name")
	})

	t.Run("labels are applied correctly", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		spec := newTestProjectSpec("Project with labels")

		created, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Verify labels and annotations
		assert.Equal(t, "knodex", created.Labels["app.kubernetes.io/managed-by"])
		assert.Equal(t, testCreatedBy, created.Annotations["knodex.io/created-by"])
	})
}

// =============================================================================
// GET INTEGRATION TESTS
// =============================================================================

func TestProjectService_GetProject_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing project", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Get test project")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get project
		retrieved, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)

		// Verify all fields
		assert.Equal(t, name, retrieved.Name)
		assert.Equal(t, "Get test project", retrieved.Spec.Description)
		assert.Len(t, retrieved.Spec.Destinations, 1)
		assert.Equal(t, "default", retrieved.Spec.Destinations[0].Namespace)
		assert.Len(t, retrieved.Spec.Roles, 1)
		assert.Equal(t, "admin", retrieved.Spec.Roles[0].Name)

		// Verify Kubernetes metadata
		assert.NotEmpty(t, retrieved.UID)
		assert.NotEmpty(t, retrieved.ResourceVersion)
		assert.False(t, retrieved.CreationTimestamp.IsZero())
	})

	t.Run("get non-existent project returns NotFound error", func(t *testing.T) {
		nonExistentName := testProjectName()

		_, err := testProjectSvc.GetProject(ctx, nonExistentName)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err), "Expected NotFound error, got: %v", err)
	})

	t.Run("get project with complex spec", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project with complex spec
		spec := rbac.ProjectSpec{
			Description: "Complex project",
			Destinations: []rbac.Destination{
				{Namespace: "default"},
				{Namespace: "production"},
				{Namespace: "*"}, // Allow all
			},
			Roles: []rbac.ProjectRole{
				{
					Name:        "admin",
					Description: "Full access",
					Policies:    []string{"p, role:serveradmin, *, *, *, allow"},
					Groups:      []string{"admins"},
				},
				{
					Name:        "developer",
					Description: "Limited access",
					Policies:    []string{"p, role:developer, applications, get, *, allow"},
					Groups:      []string{"developers"},
				},
			},
		}

		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get and verify
		retrieved, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)

		assert.Len(t, retrieved.Spec.Destinations, 3)
		assert.Len(t, retrieved.Spec.Roles, 2)
		assert.Contains(t, retrieved.Spec.Roles[0].Groups, "admins")
	})
}

// =============================================================================
// UPDATE INTEGRATION TESTS
// =============================================================================

func TestProjectService_UpdateProject_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Original description")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get current version
		current, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		originalVersion := current.ResourceVersion

		// Update description
		current.Spec.Description = "Updated description"
		updated, err := testProjectSvc.UpdateProject(ctx, current, testCreatedBy)
		require.NoError(t, err)

		// Verify update returned
		assert.Equal(t, "Updated description", updated.Spec.Description)
		assert.NotEqual(t, originalVersion, updated.ResourceVersion, "ResourceVersion should increment")

		// Verify persisted in Kubernetes
		retrieved, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", retrieved.Spec.Description)
	})

	t.Run("concurrent updates trigger optimistic locking conflict", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Concurrent update test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get two copies with same ResourceVersion
		copy1, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)

		copy2, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)

		assert.Equal(t, copy1.ResourceVersion, copy2.ResourceVersion)

		// Update copy1 (should succeed)
		copy1.Spec.Description = "Updated by copy1"
		_, err = testProjectSvc.UpdateProject(ctx, copy1, testCreatedBy)
		require.NoError(t, err)

		// Update copy2 with stale ResourceVersion (should fail with Conflict)
		copy2.Spec.Description = "Updated by copy2"
		_, err = testProjectSvc.UpdateProject(ctx, copy2, testCreatedBy)
		assert.Error(t, err)
		assert.True(t, errors.IsConflict(err), "Expected Conflict error, got: %v", err)

		// Verify final state is from copy1
		final, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "Updated by copy1", final.Spec.Description)
	})

	t.Run("update preserves immutable metadata", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Metadata preservation test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get and record immutable metadata
		current, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		originalUID := current.UID
		originalCreationTime := current.CreationTimestamp

		// Update
		current.Spec.Description = "Updated"
		_, err = testProjectSvc.UpdateProject(ctx, current, testCreatedBy)
		require.NoError(t, err)

		// Verify immutable metadata preserved
		updated, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, originalUID, updated.UID, "UID should be immutable")
		assert.Equal(t, originalCreationTime, updated.CreationTimestamp, "CreationTimestamp should be immutable")
	})

	t.Run("update adds new roles", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project with one role
		spec := newTestProjectSpec("Role update test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get current
		current, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Len(t, current.Spec.Roles, 1)

		// Add new role
		current.Spec.Roles = append(current.Spec.Roles, rbac.ProjectRole{
			Name:        "viewer",
			Description: "Read-only access",
			Policies:    []string{"p, role:viewer, applications, get, *, allow"},
		})
		_, err = testProjectSvc.UpdateProject(ctx, current, testCreatedBy)
		require.NoError(t, err)

		// Verify
		updated, err := testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)
		assert.Len(t, updated.Spec.Roles, 2)
	})

	t.Run("update non-existent project returns NotFound", func(t *testing.T) {
		nonExistentProject := &rbac.Project{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "knodex.io/v1alpha1",
				Kind:       "Project",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            testProjectName(),
				ResourceVersion: "1",
			},
			Spec: newTestProjectSpec("Non-existent"),
		}

		_, err := testProjectSvc.UpdateProject(ctx, nonExistentProject, testCreatedBy)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err), "Expected NotFound error, got: %v", err)
	})
}

// =============================================================================
// DELETE INTEGRATION TESTS
// =============================================================================

func TestProjectService_DeleteProject_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		name := testProjectName()

		// Create project
		spec := newTestProjectSpec("Delete test project")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Verify exists
		_, err = testProjectSvc.GetProject(ctx, name)
		require.NoError(t, err)

		// Delete
		err = testProjectSvc.DeleteProject(ctx, name)
		require.NoError(t, err)

		// Verify no longer exists
		_, err = testProjectSvc.GetProject(ctx, name)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err), "Expected NotFound error after delete")
	})

	t.Run("delete non-existent project returns NotFound error", func(t *testing.T) {
		nonExistentName := testProjectName()

		err := testProjectSvc.DeleteProject(ctx, nonExistentName)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err), "Expected NotFound error, got: %v", err)
	})

	t.Run("delete is idempotent failure", func(t *testing.T) {
		name := testProjectName()

		// Create and delete
		spec := newTestProjectSpec("Idempotent delete test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		err = testProjectSvc.DeleteProject(ctx, name)
		require.NoError(t, err)

		// Second delete should fail with NotFound
		err = testProjectSvc.DeleteProject(ctx, name)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})
}

// =============================================================================
// LIST INTEGRATION TESTS
// =============================================================================

func TestProjectService_ListProjects_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("list returns created projects", func(t *testing.T) {
		// Create 3 test projects
		names := make([]string, 3)
		for i := 0; i < 3; i++ {
			names[i] = testProjectName()
			defer cleanupProject(t, names[i])

			spec := newTestProjectSpec(fmt.Sprintf("List test project %d", i))
			_, err := testProjectSvc.CreateProject(ctx, names[i], spec, testCreatedBy)
			require.NoError(t, err)
		}

		// List all projects
		projectList, err := testProjectSvc.ListProjects(ctx)
		require.NoError(t, err)

		// Verify our test projects are in the list
		foundCount := 0
		for _, proj := range projectList.Items {
			for _, name := range names {
				if proj.Name == name {
					foundCount++
					assert.NotEmpty(t, proj.UID)
					assert.NotEmpty(t, proj.ResourceVersion)
				}
			}
		}
		assert.Equal(t, 3, foundCount, "All test projects should be in list")
	})

	t.Run("list returns projects with full metadata", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		spec := newTestProjectSpec("Full metadata test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		projectList, err := testProjectSvc.ListProjects(ctx)
		require.NoError(t, err)

		// Find our project
		var found *rbac.Project
		for i := range projectList.Items {
			if projectList.Items[i].Name == name {
				found = &projectList.Items[i]
				break
			}
		}
		require.NotNil(t, found, "Project should be in list")

		// Verify metadata
		assert.NotEmpty(t, found.UID)
		assert.NotEmpty(t, found.ResourceVersion)
		assert.False(t, found.CreationTimestamp.IsZero())
		assert.Equal(t, "Full metadata test", found.Spec.Description)
	})

	t.Run("list works with empty cluster", func(t *testing.T) {
		// First, clean up all test projects
		cleanupTestProjects(ctx)

		// Give cluster time to process deletions
		time.Sleep(100 * time.Millisecond)

		// List should not error even if empty (though there might be non-test projects)
		projectList, err := testProjectSvc.ListProjects(ctx)
		require.NoError(t, err)

		// Verify no test projects in list
		for _, proj := range projectList.Items {
			assert.False(t, strings.HasPrefix(proj.Name, testPrefix),
				"No test projects should remain after cleanup")
		}
	})
}

// =============================================================================
// CONCURRENT OPERATIONS TESTS
// =============================================================================

func TestProjectService_ConcurrentOperations_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent creates with unique names succeed", func(t *testing.T) {
		const numProjects = 10
		names := make([]string, numProjects)
		for i := 0; i < numProjects; i++ {
			names[i] = testProjectName()
			defer cleanupProject(t, names[i])
		}

		var wg sync.WaitGroup
		errChan := make(chan error, numProjects)

		// Create projects concurrently
		for i := 0; i < numProjects; i++ {
			wg.Add(1)
			go func(idx int, name string) {
				defer wg.Done()
				spec := newTestProjectSpec(fmt.Sprintf("Concurrent project %d", idx))
				_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
				if err != nil {
					errChan <- fmt.Errorf("failed to create project %s: %w", name, err)
				}
			}(i, names[i])
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Errorf("Concurrent create failed: %v", err)
		}

		// Verify all projects created
		for _, name := range names {
			_, err := testProjectSvc.GetProject(ctx, name)
			assert.NoError(t, err, "Project %s should exist", name)
		}
	})

	t.Run("concurrent gets succeed", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Concurrent get test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		const numGoroutines = 20
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// Get concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := testProjectSvc.GetProject(ctx, name)
				if err != nil {
					errChan <- err
				}
			}()
		}

		wg.Wait()
		close(errChan)

		// All gets should succeed
		for err := range errChan {
			t.Errorf("Concurrent get failed: %v", err)
		}
	})

	t.Run("concurrent updates on same project cause conflicts", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Concurrent update test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		const numUpdates = 5
		var wg sync.WaitGroup
		var conflictCount int32
		var successCount int32
		var mu sync.Mutex

		// Try concurrent updates (most should conflict)
		for i := 0; i < numUpdates; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Get current version
				current, err := testProjectSvc.GetProject(ctx, name)
				if err != nil {
					return
				}

				// Simulate some work
				time.Sleep(10 * time.Millisecond)

				// Try to update
				current.Spec.Description = fmt.Sprintf("Updated by goroutine %d", idx)
				_, err = testProjectSvc.UpdateProject(ctx, current, testCreatedBy)

				mu.Lock()
				if err != nil && errors.IsConflict(err) {
					conflictCount++
				} else if err == nil {
					successCount++
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()

		// At least some updates should have conflicted
		t.Logf("Results: %d conflicts, %d successes out of %d attempts",
			conflictCount, successCount, numUpdates)
		assert.Greater(t, int32(conflictCount), int32(0), "Expected some optimistic locking conflicts")
	})

	t.Run("concurrent deletes handle race correctly", func(t *testing.T) {
		name := testProjectName()
		// Don't defer cleanup - we're testing delete

		// Create project
		spec := newTestProjectSpec("Concurrent delete test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		const numDeletes = 5
		var wg sync.WaitGroup
		var successCount int32
		var notFoundCount int32
		var mu sync.Mutex

		// Try concurrent deletes
		for i := 0; i < numDeletes; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := testProjectSvc.DeleteProject(ctx, name)

				mu.Lock()
				if err == nil {
					successCount++
				} else if errors.IsNotFound(err) {
					notFoundCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Exactly one delete should succeed, others should get NotFound
		t.Logf("Results: %d success, %d NotFound out of %d attempts",
			successCount, notFoundCount, numDeletes)
		assert.Equal(t, int32(1), successCount, "Exactly one delete should succeed")
		assert.Equal(t, int32(numDeletes-1), notFoundCount, "Others should get NotFound")
	})
}

// =============================================================================
// ROLE OPERATIONS INTEGRATION TESTS
// =============================================================================

func TestProjectService_RoleOperations_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("add role to project", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project with one role
		spec := newTestProjectSpec("Role operations test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Add new role
		newRole := rbac.ProjectRole{
			Name:        "viewer",
			Description: "Read-only viewer role",
			Policies:    []string{"p, role:viewer, applications, get, *, allow"},
		}
		updated, err := testProjectSvc.AddRole(ctx, name, newRole, testCreatedBy)
		require.NoError(t, err)

		// Verify role added
		assert.Len(t, updated.Spec.Roles, 2)

		foundViewer := false
		for _, r := range updated.Spec.Roles {
			if r.Name == "viewer" {
				foundViewer = true
				assert.Equal(t, "Read-only viewer role", r.Description)
			}
		}
		assert.True(t, foundViewer, "Viewer role should be added")
	})

	t.Run("remove role from project", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project with multiple roles
		spec := rbac.ProjectSpec{
			Description:  "Role removal test",
			Destinations: []rbac.Destination{{Namespace: "*"}},
			Roles: []rbac.ProjectRole{
				{Name: "admin", Description: "Admin", Policies: []string{"p, allow"}},
				{Name: "viewer", Description: "Viewer", Policies: []string{"p, allow"}},
			},
		}
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Remove viewer role
		updated, err := testProjectSvc.RemoveRole(ctx, name, "viewer", testCreatedBy)
		require.NoError(t, err)

		// Verify role removed
		assert.Len(t, updated.Spec.Roles, 1)
		assert.Equal(t, "admin", updated.Spec.Roles[0].Name)
	})

	t.Run("add group to role", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Group operations test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Add group to admin role
		updated, err := testProjectSvc.AddGroupToRole(ctx, name, "admin", "developers", testCreatedBy)
		require.NoError(t, err)

		// Verify group added
		var adminRole *rbac.ProjectRole
		for i := range updated.Spec.Roles {
			if updated.Spec.Roles[i].Name == "admin" {
				adminRole = &updated.Spec.Roles[i]
				break
			}
		}
		require.NotNil(t, adminRole)
		assert.Contains(t, adminRole.Groups, "developers")
	})

	t.Run("get project roles", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project with multiple roles
		spec := rbac.ProjectSpec{
			Description:  "Get roles test",
			Destinations: []rbac.Destination{{Namespace: "*"}},
			Roles: []rbac.ProjectRole{
				{Name: "admin", Description: "Admin role", Policies: []string{"p, allow"}},
				{Name: "viewer", Description: "Viewer role", Policies: []string{"p, get, allow"}},
				{Name: "developer", Description: "Developer role", Policies: []string{"p, sync, allow"}},
			},
		}
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Get roles
		roles, err := testProjectSvc.GetProjectRoles(ctx, name)
		require.NoError(t, err)
		assert.Len(t, roles, 3)

		roleNames := make([]string, len(roles))
		for i, r := range roles {
			roleNames[i] = r.Name
		}
		assert.Contains(t, roleNames, "admin")
		assert.Contains(t, roleNames, "viewer")
		assert.Contains(t, roleNames, "developer")
	})
}

// =============================================================================
// GetProjectByDestinationNamespace INTEGRATION TESTS
// =============================================================================

func TestProjectService_GetProjectByDestinationNamespace_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("find project by destination namespace", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		uniqueNamespace := fmt.Sprintf("ns-%s", uuid.New().String()[:8])

		// Create project with specific namespace
		spec := rbac.ProjectSpec{
			Description: "Namespace lookup test",
			Destinations: []rbac.Destination{
				{Namespace: uniqueNamespace},
			},
		}
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Find by namespace
		found, err := testProjectSvc.GetProjectByDestinationNamespace(ctx, uniqueNamespace)
		require.NoError(t, err)
		assert.Equal(t, name, found.Name)
	})

	t.Run("namespace not found returns error", func(t *testing.T) {
		nonExistentNs := fmt.Sprintf("ns-%s-nonexistent", uuid.New().String()[:8])

		found, err := testProjectSvc.GetProjectByDestinationNamespace(ctx, nonExistentNs)
		assert.Error(t, err)
		assert.Nil(t, found)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// GetUserProjectsByGroup INTEGRATION TESTS
// =============================================================================

func TestProjectService_GetUserProjectsByGroup_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("find projects by user groups", func(t *testing.T) {
		// Create multiple projects with different groups
		name1 := testProjectName()
		defer cleanupProject(t, name1)
		name2 := testProjectName()
		defer cleanupProject(t, name2)
		name3 := testProjectName()
		defer cleanupProject(t, name3)

		// Project 1: developers group
		spec1 := rbac.ProjectSpec{
			Description:  "Developers project",
			Destinations: []rbac.Destination{{Namespace: "*"}},
			Roles: []rbac.ProjectRole{
				{Name: "developer", Description: "Dev", Policies: []string{"p, allow"}, Groups: []string{"developers"}},
			},
		}
		_, err := testProjectSvc.CreateProject(ctx, name1, spec1, testCreatedBy)
		require.NoError(t, err)

		// Project 2: admins group
		spec2 := rbac.ProjectSpec{
			Description:  "Admins project",
			Destinations: []rbac.Destination{{Namespace: "*"}},
			Roles: []rbac.ProjectRole{
				{Name: "admin", Description: "Admin", Policies: []string{"p, allow"}, Groups: []string{"admins"}},
			},
		}
		_, err = testProjectSvc.CreateProject(ctx, name2, spec2, testCreatedBy)
		require.NoError(t, err)

		// Project 3: both groups
		spec3 := rbac.ProjectSpec{
			Description:  "Shared project",
			Destinations: []rbac.Destination{{Namespace: "*"}},
			Roles: []rbac.ProjectRole{
				{Name: "all", Description: "All", Policies: []string{"p, allow"}, Groups: []string{"developers", "admins"}},
			},
		}
		_, err = testProjectSvc.CreateProject(ctx, name3, spec3, testCreatedBy)
		require.NoError(t, err)

		// Find projects for user in "developers" group
		projects, err := testProjectSvc.GetUserProjectsByGroup(ctx, []string{"developers"})
		require.NoError(t, err)

		// Should find project1 and project3
		foundNames := make([]string, 0)
		for _, p := range projects {
			if strings.HasPrefix(p.Name, testPrefix) {
				foundNames = append(foundNames, p.Name)
			}
		}
		assert.Contains(t, foundNames, name1)
		assert.Contains(t, foundNames, name3)
		assert.NotContains(t, foundNames, name2)
	})
}

// =============================================================================
// EXISTS INTEGRATION TESTS
// =============================================================================

func TestProjectService_Exists_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("returns true for created project", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Exists test - created project")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Verify Exists returns true
		exists, err := testProjectSvc.Exists(ctx, name)
		require.NoError(t, err)
		assert.True(t, exists, "Exists should return true for created project")
	})

	t.Run("returns false after project deleted", func(t *testing.T) {
		name := testProjectName()
		// Don't defer cleanup - we're testing deletion

		// Create project
		spec := newTestProjectSpec("Exists test - deleted project")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		// Verify exists initially
		exists, err := testProjectSvc.Exists(ctx, name)
		require.NoError(t, err)
		assert.True(t, exists, "Project should exist after creation")

		// Delete project
		err = testProjectSvc.DeleteProject(ctx, name)
		require.NoError(t, err)

		// Verify Exists returns false after deletion
		exists, err = testProjectSvc.Exists(ctx, name)
		require.NoError(t, err)
		assert.False(t, exists, "Exists should return false after project deleted")
	})

	t.Run("returns false for non-existent project name", func(t *testing.T) {
		// Generate a name that definitely doesn't exist
		name := testProjectName() + "-nonexistent"

		// Verify Exists returns false for non-existent project
		exists, err := testProjectSvc.Exists(ctx, name)
		require.NoError(t, err)
		assert.False(t, exists, "Exists should return false for non-existent project")
	})

	t.Run("returns false for empty name", func(t *testing.T) {
		// Test edge case with empty string
		exists, err := testProjectSvc.Exists(ctx, "")
		// This should either return false or an error, both are acceptable
		if err == nil {
			assert.False(t, exists, "Exists should return false for empty name")
		}
		// If error, that's also acceptable behavior for empty name
	})

	t.Run("handles concurrent exists checks", func(t *testing.T) {
		name := testProjectName()
		defer cleanupProject(t, name)

		// Create project
		spec := newTestProjectSpec("Concurrent exists test")
		_, err := testProjectSvc.CreateProject(ctx, name, spec, testCreatedBy)
		require.NoError(t, err)

		const numGoroutines = 20
		var wg sync.WaitGroup
		results := make(chan bool, numGoroutines)
		errChan := make(chan error, numGoroutines)

		// Check exists concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				exists, err := testProjectSvc.Exists(ctx, name)
				if err != nil {
					errChan <- err
					return
				}
				results <- exists
			}()
		}

		wg.Wait()
		close(results)
		close(errChan)

		// All checks should succeed and return true
		for err := range errChan {
			t.Errorf("Concurrent exists check failed: %v", err)
		}

		for exists := range results {
			assert.True(t, exists, "All concurrent checks should return true")
		}
	})
}
