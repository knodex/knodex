// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/rbac"
)

const (
	// DefaultProjectDescription is the description for the default project
	DefaultProjectDescription = "Default Project"

	// DefaultProjectNamespace is the Kubernetes namespace for the default project
	DefaultProjectNamespace = "default-project"

	// DefaultProjectName is the resource name for the default project (DNS-1123 compliant)
	DefaultProjectName = "default-project"
)

// ProjectBootstrapService handles default project creation for admin users
type ProjectBootstrapService struct {
	projectService AuthProjectService
	k8sClient      kubernetes.Interface
	// projectName / projectNamespace name the auto-created default project.
	// They are configurable (audit G-15) so a cloud tenant can rename it; empty
	// inputs fall back to the DefaultProject* constants for OSS/EE parity.
	projectName      string
	projectNamespace string
}

// NewProjectBootstrapService creates a new bootstrap service. projectName and
// projectNamespace are configurable (audit G-15); pass empty strings to use the
// DefaultProjectName / DefaultProjectNamespace constants (today's behavior).
func NewProjectBootstrapService(projectService AuthProjectService, k8sClient kubernetes.Interface, projectName, projectNamespace string) *ProjectBootstrapService {
	if projectName == "" {
		projectName = DefaultProjectName
	}
	if projectNamespace == "" {
		projectNamespace = DefaultProjectNamespace
	}
	return &ProjectBootstrapService{
		projectService:   projectService,
		k8sClient:        k8sClient,
		projectName:      projectName,
		projectNamespace: projectNamespace,
	}
}

// EnsureDefaultProject ensures the default project exists and returns it
// This is idempotent - safe to call multiple times
func (s *ProjectBootstrapService) EnsureDefaultProject(ctx context.Context, adminUserID string) (*rbac.Project, error) {
	// Try to get existing default project
	project, err := s.projectService.GetProject(ctx, s.projectName)
	if err == nil {
		slog.Info("default project already exists",
			"project_id", project.Name,
			"description", project.Spec.Description,
		)

		// Ensure admin role exists with admin user's group
		project, err = s.ensureAdminRole(ctx, project, adminUserID)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure admin role: %w", err)
		}

		return project, nil
	}

	// If error is not "not found", return it
	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check default project: %w", err)
	}

	// Default project doesn't exist - create it
	slog.Info("creating default project",
		"project_name", s.projectName,
		"namespace", s.projectNamespace,
		"admin_user_id", adminUserID,
	)

	// Create ArgoCD-aligned project spec with default configuration
	projectSpec := rbac.ProjectSpec{
		Description: DefaultProjectDescription,
		// Allow deployments to the default-project namespace
		Destinations: []rbac.Destination{
			{
				Namespace: s.projectNamespace,
			},
		},
		// Allow common namespace-scoped resources
		NamespaceResourceWhitelist: []rbac.ResourceSpec{
			{Group: "*", Kind: "*"}, // Allow all namespace-scoped resources
		},
		// Define default roles with admin as platform-admin
		Roles: []rbac.ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Full access to project resources",
				Policies: []string{
					fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", s.projectName, s.projectName),
				},
				// Team binding is the operator's responsibility (Teams-only binding, Story 10.6).
			},
			{
				Name:        "developer",
				Description: "Deploy and manage instances within the project",
				Policies: []string{
					fmt.Sprintf("p, proj:%s:developer, applications, *, %s/*, allow", s.projectName, s.projectName),
					fmt.Sprintf("p, proj:%s:developer, repositories, get, %s/*, allow", s.projectName, s.projectName),
				},
			},
			{
				Name:        "viewer",
				Description: "Read-only access to project resources",
				Policies: []string{
					fmt.Sprintf("p, proj:%s:viewer, *, get, %s/*, allow", s.projectName, s.projectName),
				},
			},
		},
	}

	// Create the project
	project, err = s.projectService.CreateProject(ctx, s.projectName, projectSpec, adminUserID)
	if err != nil {
		// Check if it was created in a race condition
		if errors.IsAlreadyExists(err) {
			// Another concurrent request created it - get it
			project, getErr := s.projectService.GetProject(ctx, s.projectName)
			if getErr != nil {
				return nil, fmt.Errorf("default project created by another request but failed to retrieve: %w", getErr)
			}

			// Ensure admin role exists
			project, err = s.ensureAdminRole(ctx, project, adminUserID)
			if err != nil {
				return nil, fmt.Errorf("failed to ensure admin role after race: %w", err)
			}

			return project, nil
		}
		return nil, fmt.Errorf("failed to create default project: %w", err)
	}

	slog.Info("default project created successfully",
		"project_id", project.Name,
		"description", project.Spec.Description,
		"admin_user_id", adminUserID,
	)

	return project, nil
}

// ensureAdminRole ensures the platform-admin role exists in the project.
// Team binding is the operator's responsibility (via Teams-only binding, Story 10.6).
func (s *ProjectBootstrapService) ensureAdminRole(ctx context.Context, project *rbac.Project, adminUserID string) (*rbac.Project, error) {
	// Check if platform-admin role already exists
	for _, role := range project.Spec.Roles {
		if role.Name == "platform-admin" {
			return project, nil
		}
	}

	// platform-admin role doesn't exist - create it (no group/team binding; operator configures Teams)
	slog.Warn("platform-admin role not found in project, creating it",
		"project_id", project.Name,
		"admin_user_id", adminUserID,
	)

	adminRole := rbac.ProjectRole{
		Name:        "platform-admin",
		Description: "Full access to project resources",
		Policies: []string{
			fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", project.Name, project.Name),
		},
	}

	updatedProject, err := s.projectService.AddRole(ctx, project.Name, adminRole, "system-bootstrap")
	if err != nil {
		return nil, fmt.Errorf("failed to add platform-admin role to project: %w", err)
	}

	return updatedProject, nil
}
