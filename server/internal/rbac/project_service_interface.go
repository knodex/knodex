// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import "context"

// ProjectServiceInterface defines the contract for project CRUD operations.
// This interface allows handlers to depend on an abstraction rather than
// the concrete ProjectService, enabling easier testing with mocks.
type ProjectServiceInterface interface {
	// CreateProject creates a new Project CRD
	CreateProject(ctx context.Context, name string, spec ProjectSpec, createdBy string) (*Project, error)

	// GetProject retrieves a Project by name
	GetProject(ctx context.Context, name string) (*Project, error)

	// ListProjects lists all Project resources
	ListProjects(ctx context.Context) (*ProjectList, error)

	// UpdateProject updates an existing Project
	UpdateProject(ctx context.Context, project *Project, updatedBy string) (*Project, error)

	// DeleteProject deletes a Project by name
	DeleteProject(ctx context.Context, name string) error

	// Exists checks if a project exists by name
	Exists(ctx context.Context, name string) (bool, error)

	// UpdateProjectStatus updates only the status subresource of a Project
	UpdateProjectStatus(ctx context.Context, project *Project) (*Project, error)
}

// Ensure ProjectService implements ProjectServiceInterface
var _ ProjectServiceInterface = (*ProjectService)(nil)

// Exists checks if a project exists by name.
// This method is added to satisfy the ProjectServiceInterface.
func (s *ProjectService) Exists(ctx context.Context, name string) (bool, error) {
	_, err := s.GetProject(ctx, name)
	if err != nil {
		// Check if error indicates not found
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isNotFoundError checks if an error indicates a resource was not found
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common "not found" patterns in Kubernetes errors
	errStr := err.Error()
	return contains(errStr, "not found") || contains(errStr, "NotFound")
}

// contains is a simple string contains check without importing strings
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
