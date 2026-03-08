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
}

// NewProjectBootstrapService creates a new bootstrap service
func NewProjectBootstrapService(projectService AuthProjectService, k8sClient kubernetes.Interface) *ProjectBootstrapService {
	return &ProjectBootstrapService{
		projectService: projectService,
		k8sClient:      k8sClient,
	}
}

// EnsureDefaultProject ensures the default project exists and returns it
// This is idempotent - safe to call multiple times
func (s *ProjectBootstrapService) EnsureDefaultProject(ctx context.Context, adminUserID string) (*rbac.Project, error) {
	// Try to get existing default project
	project, err := s.projectService.GetProject(ctx, DefaultProjectName)
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
		"project_name", DefaultProjectName,
		"namespace", DefaultProjectNamespace,
		"admin_user_id", adminUserID,
	)

	// Create ArgoCD-aligned project spec with default configuration
	projectSpec := rbac.ProjectSpec{
		Description: DefaultProjectDescription,
		// Allow deployments to the default-project namespace
		Destinations: []rbac.Destination{
			{
				Namespace: DefaultProjectNamespace,
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
					fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", DefaultProjectName, DefaultProjectName),
				},
				Groups: []string{
					fmt.Sprintf("admin:%s", adminUserID), // Admin user's group
				},
			},
			{
				Name:        "developer",
				Description: "Deploy and manage instances within the project",
				Policies: []string{
					fmt.Sprintf("p, proj:%s:developer, applications, *, %s/*, allow", DefaultProjectName, DefaultProjectName),
					fmt.Sprintf("p, proj:%s:developer, repositories, get, %s/*, allow", DefaultProjectName, DefaultProjectName),
				},
				Groups: []string{}, // No groups by default
			},
			{
				Name:        "viewer",
				Description: "Read-only access to project resources",
				Policies: []string{
					fmt.Sprintf("p, proj:%s:viewer, *, get, %s/*, allow", DefaultProjectName, DefaultProjectName),
				},
				Groups: []string{}, // No groups by default
			},
		},
	}

	// Create the project
	project, err = s.projectService.CreateProject(ctx, DefaultProjectName, projectSpec, adminUserID)
	if err != nil {
		// Check if it was created in a race condition
		if errors.IsAlreadyExists(err) {
			// Another concurrent request created it - get it
			project, getErr := s.projectService.GetProject(ctx, DefaultProjectName)
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

// ensureAdminRole ensures the admin user has the platform-admin role in the project
// In the ArgoCD model, users are bound to roles via OIDC groups
func (s *ProjectBootstrapService) ensureAdminRole(ctx context.Context, project *rbac.Project, adminUserID string) (*rbac.Project, error) {
	adminGroup := fmt.Sprintf("admin:%s", adminUserID)

	// Check if platform-admin role exists with admin's group
	for _, role := range project.Spec.Roles {
		if role.Name == "platform-admin" {
			for _, group := range role.Groups {
				if group == adminGroup {
					// Admin group is already in the role
					return project, nil
				}
			}

			// Admin group not in role - add it
			slog.Warn("admin user not in platform-admin role, adding their group",
				"project_id", project.Name,
				"admin_user_id", adminUserID,
				"admin_group", adminGroup,
			)

			updatedProject, err := s.projectService.AddGroupToRole(ctx, project.Name, "platform-admin", adminGroup, "system-bootstrap")
			if err != nil {
				return nil, fmt.Errorf("failed to add admin group to platform-admin role: %w", err)
			}

			return updatedProject, nil
		}
	}

	// platform-admin role doesn't exist - create it
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
		Groups: []string{adminGroup},
	}

	updatedProject, err := s.projectService.AddRole(ctx, project.Name, adminRole, "system-bootstrap")
	if err != nil {
		return nil, fmt.Errorf("failed to add platform-admin role to project: %w", err)
	}

	return updatedProject, nil
}
