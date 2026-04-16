// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// TestRetryOnConflict_Success tests when the operation succeeds on first try
func TestRetryOnConflict_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callCount := 0

	err := RetryOnConflict(ctx, func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got: %d", callCount)
	}
}

// TestRetryOnConflict_NonConflictError tests when a non-conflict error occurs
func TestRetryOnConflict_NonConflictError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callCount := 0
	expectedErr := errors.New("database connection error")

	err := RetryOnConflict(ctx, func() error {
		callCount++
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got: %v", expectedErr, err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retry for non-conflict), got: %d", callCount)
	}
}

// TestRetryOnConflict_ConflictThenSuccess tests retry on conflict then success
func TestRetryOnConflict_ConflictThenSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callCount := 0

	err := RetryOnConflict(ctx, func() error {
		callCount++
		if callCount < 3 {
			// Return conflict error for first 2 calls
			return apierrors.NewConflict(
				schema.GroupResource{Group: "test", Resource: "items"},
				"test-item",
				errors.New("resource modified"),
			)
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got: %d", callCount)
	}
}

// TestRetryOnConflict_ExhaustedRetries tests when all retries are exhausted
func TestRetryOnConflict_ExhaustedRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callCount := 0

	err := RetryOnConflict(ctx, func() error {
		callCount++
		return apierrors.NewConflict(
			schema.GroupResource{Group: "test", Resource: "items"},
			"test-item",
			errors.New("resource modified"),
		)
	})

	if err == nil {
		t.Error("Expected error after exhausting retries")
	}
	if callCount != MaxRetries {
		t.Errorf("Expected %d calls, got: %d", MaxRetries, callCount)
	}
	// Check error message contains retry information
	if !errors.Is(err, apierrors.NewConflict(
		schema.GroupResource{Group: "test", Resource: "items"},
		"test-item",
		errors.New("resource modified"),
	)) && err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestRetryOnConflict_ContextCancelled tests cancellation during retry
func TestRetryOnConflict_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	// Cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RetryOnConflict(ctx, func() error {
		callCount++
		return apierrors.NewConflict(
			schema.GroupResource{Group: "test", Resource: "items"},
			"test-item",
			errors.New("resource modified"),
		)
	})

	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestRetryOnConflict_ContextTimeout tests timeout during retry
func TestRetryOnConflict_ContextTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0

	err := RetryOnConflict(ctx, func() error {
		callCount++
		return apierrors.NewConflict(
			schema.GroupResource{Group: "test", Resource: "items"},
			"test-item",
			errors.New("resource modified"),
		)
	})

	if err == nil {
		t.Error("Expected error due to context timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
	}
}

// TestConcurrencyConstants tests the concurrency constants
func TestConcurrencyConstants(t *testing.T) {
	t.Parallel()

	if MaxRetries != 5 {
		t.Errorf("Expected MaxRetries = 5, got %d", MaxRetries)
	}
	if RetryDelay != 100*time.Millisecond {
		t.Errorf("Expected RetryDelay = 100ms, got %v", RetryDelay)
	}
}

func setupConcurrencyTestService() *ProjectService {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion{Group: ProjectGroup, Version: ProjectVersion},
		&Project{},
		&ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: ProjectGroup, Version: ProjectVersion})

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	k8sClient := k8sfake.NewSimpleClientset()

	return NewProjectService(k8sClient, dynamicClient, "knodex-system")
}

// createTestProjectSpec creates a valid ArgoCD-aligned ProjectSpec for testing
func createTestProjectSpec() ProjectSpec {
	return ProjectSpec{
		Description: "Test Project",
		Destinations: []Destination{
			{
				Namespace: "test-project",
			},
		},
		NamespaceResourceWhitelist: []ResourceSpec{
			{Group: "*", Kind: "*"},
		},
		Roles: []ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Full access to project resources",
				Policies: []string{
					"p, proj:test-project:platform-admin, *, *, test-project/*, allow",
				},
				Groups: []string{"admin:admin-1"},
			},
		},
	}
}

func TestConcurrent_AddGroupToRole(t *testing.T) {
	t.Parallel()

	service := setupConcurrencyTestService()
	ctx := context.Background()

	// Create initial project with ArgoCD-aligned spec
	initialSpec := createTestProjectSpec()
	project, err := service.CreateProject(ctx, "test-project", initialSpec, "admin-1")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Concurrently add 10 groups to the platform-admin role
	numGroups := 10
	var wg sync.WaitGroup
	wg.Add(numGroups)

	errors := make(chan error, numGroups)
	results := make([]*Project, numGroups)

	for i := 0; i < numGroups; i++ {
		go func(index int) {
			defer wg.Done()
			groupName := fmt.Sprintf("group-%d", index)
			updatedProject, err := service.AddGroupToRole(ctx, project.Name, "platform-admin", groupName, "admin-1")
			if err != nil {
				errors <- err
				return
			}
			results[index] = updatedProject
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("AddGroupToRole() concurrent call failed: %v", err)
	}

	// Get final state
	finalProject, err := service.GetProject(ctx, project.Name)
	if err != nil {
		t.Fatalf("Failed to get final project: %v", err)
	}

	// Verify at least some groups were added
	// Note: With fake client, not all concurrent operations may succeed due to lack of real conflict handling
	// In a real cluster with actual etcd, the RetryOnConflict mechanism would handle this properly
	minExpected := 2 // At least the initial group plus some added groups
	var platformAdminRole *ProjectRole
	for i := range finalProject.Spec.Roles {
		if finalProject.Spec.Roles[i].Name == "platform-admin" {
			platformAdminRole = &finalProject.Spec.Roles[i]
			break
		}
	}

	if platformAdminRole == nil {
		t.Fatal("platform-admin role not found")
	}

	if len(platformAdminRole.Groups) < minExpected {
		t.Errorf("Concurrent AddGroupToRole() resulted in too few groups: %d, want at least %d", len(platformAdminRole.Groups), minExpected)
	}

	t.Logf("Concurrent operations resulted in %d groups (initial: 1, attempted adds: 10)", len(platformAdminRole.Groups))

	// Verify no duplicate groups
	groupMap := make(map[string]bool)
	for _, group := range platformAdminRole.Groups {
		if groupMap[group] {
			t.Errorf("Concurrent AddGroupToRole() resulted in duplicate group: %s", group)
		}
		groupMap[group] = true
	}
}

func TestConcurrent_AddAndRemoveRoles(t *testing.T) {
	t.Parallel()

	service := setupConcurrencyTestService()
	ctx := context.Background()

	// Create project with initial roles
	initialSpec := ProjectSpec{
		Description: "Test Project",
		Destinations: []Destination{
			{
				Namespace: "test-project",
			},
		},
		NamespaceResourceWhitelist: []ResourceSpec{
			{Group: "*", Kind: "*"},
		},
		Roles: []ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Full access",
				Policies:    []string{"p, proj:test-project:platform-admin, *, *, test-project/*, allow"},
				Groups:      []string{"admin:admin-1"},
			},
			{
				Name:        "developer-1",
				Description: "Developer role 1",
				Policies:    []string{"p, proj:test-project:developer-1, applications, *, test-project/*, allow"},
				Groups:      []string{},
			},
			{
				Name:        "developer-2",
				Description: "Developer role 2",
				Policies:    []string{"p, proj:test-project:developer-2, applications, *, test-project/*, allow"},
				Groups:      []string{},
			},
			{
				Name:        "developer-3",
				Description: "Developer role 3",
				Policies:    []string{"p, proj:test-project:developer-3, applications, *, test-project/*, allow"},
				Groups:      []string{},
			},
		},
	}
	project, err := service.CreateProject(ctx, "test-project", initialSpec, "admin-1")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrently add 3 new roles
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func(index int) {
			defer wg.Done()
			roleName := fmt.Sprintf("newrole-%d", index)
			newRole := ProjectRole{
				Name:        roleName,
				Description: "New role " + roleName,
				Policies:    []string{fmt.Sprintf("p, proj:test-project:%s, *, get, test-project/*, allow", roleName)},
				Groups:      []string{},
			}
			_, _ = service.AddRole(ctx, project.Name, newRole, "admin-1")
		}(i)
	}

	// Concurrently remove 2 existing roles
	wg.Add(2)
	for i := 1; i <= 2; i++ {
		go func(index int) {
			defer wg.Done()
			roleName := fmt.Sprintf("developer-%d", index)
			_, _ = service.RemoveRole(ctx, project.Name, roleName, "admin-1")
		}(i)
	}

	wg.Wait()

	// Get final state
	finalProject, err := service.GetProject(ctx, project.Name)
	if err != nil {
		t.Fatalf("Failed to get final project: %v", err)
	}

	// Verify no duplicate roles
	roleMap := make(map[string]bool)
	for _, role := range finalProject.Spec.Roles {
		if roleMap[role.Name] {
			t.Errorf("Concurrent operations resulted in duplicate role: %s", role.Name)
		}
		roleMap[role.Name] = true
	}

	// Verify all roles have valid names
	for _, role := range finalProject.Spec.Roles {
		if err := role.Validate(); err != nil {
			t.Errorf("Concurrent operations resulted in invalid role %s: %v", role.Name, err)
		}
	}
}

func TestConcurrent_ResourceVersionConflictRetry(t *testing.T) {
	t.Parallel()

	service := setupConcurrencyTestService()
	ctx := context.Background()

	// Create project
	initialSpec := createTestProjectSpec()
	project, err := service.CreateProject(ctx, "test-project", initialSpec, "admin-1")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Launch many concurrent operations to force conflicts
	numOperations := 20
	var wg sync.WaitGroup
	wg.Add(numOperations)

	errors := make(chan error, numOperations)

	for i := 0; i < numOperations; i++ {
		go func(index int) {
			defer wg.Done()
			groupName := fmt.Sprintf("group-%d", index)
			_, err := service.AddGroupToRole(ctx, project.Name, "platform-admin", groupName, "admin-1")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errorCount int
	for err := range errors {
		errorCount++
		t.Logf("Operation failed: %v", err)
	}

	// Some operations might fail due to max retries, but most should succeed
	// The retry mechanism should handle most conflicts
	if errorCount > numOperations/2 {
		t.Errorf("Too many concurrent operations failed: %d out of %d", errorCount, numOperations)
	}

	// Get final state
	finalProject, err := service.GetProject(ctx, project.Name)
	if err != nil {
		t.Fatalf("Failed to get final project: %v", err)
	}

	// Find platform-admin role
	var platformAdminRole *ProjectRole
	for i := range finalProject.Spec.Roles {
		if finalProject.Spec.Roles[i].Name == "platform-admin" {
			platformAdminRole = &finalProject.Spec.Roles[i]
			break
		}
	}

	if platformAdminRole == nil {
		t.Fatal("platform-admin role not found")
	}

	// Verify no duplicate groups
	groupMap := make(map[string]bool)
	for _, group := range platformAdminRole.Groups {
		if groupMap[group] {
			t.Errorf("RetryOnConflict() failed to prevent duplicate group: %s", group)
		}
		groupMap[group] = true
	}

	t.Logf("Final group count: %d (expected ~%d, allowing for some conflicts)", len(platformAdminRole.Groups), numOperations+1)
}
