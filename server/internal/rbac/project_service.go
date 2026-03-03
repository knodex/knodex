package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	// ProjectGVR is the GroupVersionResource for Project CRD
	ProjectGVR = schema.GroupVersionResource{
		Group:    ProjectGroup,
		Version:  ProjectVersion,
		Resource: ProjectResource,
	}
)

// ProjectService provides operations on Project CRDs
type ProjectService struct {
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
}

// NewProjectService creates a new ProjectService
func NewProjectService(k8sClient kubernetes.Interface, dynamicClient dynamic.Interface) *ProjectService {
	return &ProjectService{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}
}

// CreateProject creates a new Project CRD
func (s *ProjectService) CreateProject(ctx context.Context, name string, spec ProjectSpec, createdBy string) (*Project, error) {
	// Validate project name
	if err := ValidateProjectName(name); err != nil {
		return nil, fmt.Errorf("invalid project name: %w", err)
	}

	// Validate project spec
	if err := ValidateProjectSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid project spec: %w", err)
	}

	// Create Project object
	project := &Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ProjectGroup + "/" + ProjectVersion,
			Kind:       ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "knodex",
			},
			Annotations: map[string]string{
				"knodex.io/created-by": createdBy,
				"knodex.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
		Spec: spec,
		Status: ProjectStatus{
			Conditions: []ProjectCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             "Created",
					Message:            "Project created successfully",
				},
			},
		},
	}

	// Convert to unstructured for dynamic client
	unstructuredProject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(project)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project to unstructured: %w", err)
	}

	// Create resource
	result, err := s.dynamicClient.Resource(ProjectGVR).Create(
		ctx,
		&unstructured.Unstructured{Object: unstructuredProject},
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Convert back to Project
	var createdProject Project
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &createdProject); err != nil {
		return nil, fmt.Errorf("failed to convert result to project: %w", err)
	}

	return &createdProject, nil
}

// CreateProjectWithDescription creates a project with a description, generating a unique ID
func (s *ProjectService) CreateProjectWithDescription(ctx context.Context, description string, spec ProjectSpec, createdBy string) (*Project, error) {
	// Generate project ID from description
	projectID := GenerateProjectID(description)

	// Add description to spec if not already set
	if spec.Description == "" {
		spec.Description = description
	}

	return s.CreateProject(ctx, projectID, spec, createdBy)
}

// GetProject retrieves a Project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*Project, error) {
	result, err := s.dynamicClient.Resource(ProjectGVR).Get(
		ctx,
		projectID,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}

	var project Project
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &project); err != nil {
		return nil, fmt.Errorf("failed to convert result to project: %w", err)
	}

	return &project, nil
}

// GetProjectByDestinationNamespace retrieves a Project that has a destination with the specified namespace
func (s *ProjectService) GetProjectByDestinationNamespace(ctx context.Context, namespace string) (*Project, error) {
	// List all projects and filter by destination namespace
	projects, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	for _, project := range projects.Items {
		for _, dest := range project.Spec.Destinations {
			if dest.Namespace == namespace || dest.Namespace == "*" {
				return &project, nil
			}
			// Check for wildcard patterns
			if IsWildcard(dest.Namespace) && matchesWildcard(dest.Namespace, namespace) {
				return &project, nil
			}
		}
	}

	return nil, fmt.Errorf("project with destination namespace %s not found", namespace)
}

// matchesWildcard checks if a namespace matches a wildcard pattern
func matchesWildcard(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
	}
	return pattern == value
}

// ListProjects lists all Project resources
func (s *ProjectService) ListProjects(ctx context.Context) (*ProjectList, error) {
	result, err := s.dynamicClient.Resource(ProjectGVR).List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var projectList ProjectList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.UnstructuredContent(), &projectList); err != nil {
		return nil, fmt.Errorf("failed to convert result to project list: %w", err)
	}

	return &projectList, nil
}

// UpdateProject updates an existing Project
func (s *ProjectService) UpdateProject(ctx context.Context, project *Project, updatedBy string) (*Project, error) {
	// Validate the updated spec
	if err := ValidateProjectSpec(project.Spec); err != nil {
		return nil, fmt.Errorf("invalid project spec: %w", err)
	}

	// Update annotations to track who updated it
	if project.Annotations == nil {
		project.Annotations = make(map[string]string)
	}
	project.Annotations["knodex.io/updated-by"] = updatedBy
	project.Annotations["knodex.io/updated-at"] = time.Now().Format(time.RFC3339)

	// Convert to unstructured
	unstructuredProject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(project)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project to unstructured: %w", err)
	}

	// Update resource
	result, err := s.dynamicClient.Resource(ProjectGVR).Update(
		ctx,
		&unstructured.Unstructured{Object: unstructuredProject},
		metav1.UpdateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update project %s: %w", project.Name, err)
	}

	// Convert back to Project
	var updatedProject Project
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &updatedProject); err != nil {
		return nil, fmt.Errorf("failed to convert result to project: %w", err)
	}

	return &updatedProject, nil
}

// UpdateProjectStatus updates only the status subresource
func (s *ProjectService) UpdateProjectStatus(ctx context.Context, project *Project) (*Project, error) {
	// Convert to unstructured
	unstructuredProject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(project)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project to unstructured: %w", err)
	}

	// Update status subresource
	result, err := s.dynamicClient.Resource(ProjectGVR).UpdateStatus(
		ctx,
		&unstructured.Unstructured{Object: unstructuredProject},
		metav1.UpdateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update project status %s: %w", project.Name, err)
	}

	// Convert back to Project
	var updatedProject Project
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &updatedProject); err != nil {
		return nil, fmt.Errorf("failed to convert result to project: %w", err)
	}

	return &updatedProject, nil
}

// DeleteProject deletes a Project by ID
func (s *ProjectService) DeleteProject(ctx context.Context, projectID string) error {
	err := s.dynamicClient.Resource(ProjectGVR).Delete(
		ctx,
		projectID,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete project %s: %w", projectID, err)
	}

	return nil
}

// GetUserRole returns the role of a user in a specific project
// In the ArgoCD model, users are bound to roles via OIDC groups in ProjectRole.Groups
func (s *ProjectService) GetUserRole(ctx context.Context, projectID string, userID string) (string, error) {
	// Get the project
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}

	// In the ArgoCD model, role membership is determined by OIDC groups
	// Users are assigned to roles via group membership (e.g., "user:{userID}")
	userGroup := fmt.Sprintf("user:%s", userID)

	// Check each role to see if the user's group is in its Groups list
	for _, role := range project.Spec.Roles {
		for _, group := range role.Groups {
			if group == userGroup {
				return role.Name, nil
			}
		}
	}

	// User is not a member of any role in this project
	return "", nil
}

// GetProjectRoles returns all roles defined in a project
func (s *ProjectService) GetProjectRoles(ctx context.Context, projectID string) ([]ProjectRole, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return project.Spec.Roles, nil
}

// AddRole adds a new role to a project
func (s *ProjectService) AddRole(ctx context.Context, projectID string, role ProjectRole, updatedBy string) (*Project, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Validate the role
	if err := role.Validate(); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	// Check if role already exists
	for _, r := range project.Spec.Roles {
		if r.Name == role.Name {
			return nil, fmt.Errorf("role %s already exists in project %s", role.Name, projectID)
		}
	}

	// Add the new role
	project.Spec.Roles = append(project.Spec.Roles, role)

	return s.UpdateProject(ctx, project, updatedBy)
}

// RemoveRole removes a role from a project
func (s *ProjectService) RemoveRole(ctx context.Context, projectID string, roleName string, updatedBy string) (*Project, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Find and remove the role
	found := false
	newRoles := make([]ProjectRole, 0, len(project.Spec.Roles))
	for _, role := range project.Spec.Roles {
		if role.Name == roleName {
			found = true
			continue
		}
		newRoles = append(newRoles, role)
	}

	if !found {
		return nil, fmt.Errorf("role %s not found in project %s", roleName, projectID)
	}

	project.Spec.Roles = newRoles
	return s.UpdateProject(ctx, project, updatedBy)
}

// UpdateRole updates a role in a project
func (s *ProjectService) UpdateRole(ctx context.Context, projectID string, roleName string, updatedRole ProjectRole, updatedBy string) (*Project, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Validate the updated role
	if err := updatedRole.Validate(); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	// Find and update the role
	found := false
	for i, role := range project.Spec.Roles {
		if role.Name == roleName {
			project.Spec.Roles[i] = updatedRole
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("role %s not found in project %s", roleName, projectID)
	}

	return s.UpdateProject(ctx, project, updatedBy)
}

// AddGroupToRole adds an OIDC group to a project role
func (s *ProjectService) AddGroupToRole(ctx context.Context, projectID string, roleName string, groupName string, updatedBy string) (*Project, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Find the role and add the group
	found := false
	for i, role := range project.Spec.Roles {
		if role.Name == roleName {
			// Check if group already exists
			for _, g := range role.Groups {
				if g == groupName {
					return nil, fmt.Errorf("group %s already in role %s", groupName, roleName)
				}
			}
			project.Spec.Roles[i].Groups = append(project.Spec.Roles[i].Groups, groupName)
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("role %s not found in project %s", roleName, projectID)
	}

	return s.UpdateProject(ctx, project, updatedBy)
}

// GetUserProjectsByGroup returns all projects a user has access to based on their OIDC groups
func (s *ProjectService) GetUserProjectsByGroup(ctx context.Context, userGroups []string) ([]*Project, error) {
	allProjects, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var userProjects []*Project
	for i := range allProjects.Items {
		project := &allProjects.Items[i]
		// Check if any user group matches any role's groups
		for _, role := range project.Spec.Roles {
			for _, roleGroup := range role.Groups {
				for _, userGroup := range userGroups {
					if roleGroup == userGroup {
						userProjects = append(userProjects, project)
						goto nextProject
					}
				}
			}
		}
	nextProject:
	}

	return userProjects, nil
}

// GetUserProjectRolesByGroup returns a map of projectID -> roleName for a user based on their OIDC groups.
// This is used to populate the "roles" claim in the JWT token, enabling frontend permission checks.
// For each project where the user's OIDC group matches a role's groups, the role name is returned.
// If a user matches multiple roles in the same project, the first matching role is used.
func (s *ProjectService) GetUserProjectRolesByGroup(ctx context.Context, userGroups []string) (map[string]string, error) {
	if len(userGroups) == 0 {
		slog.Debug("GetUserProjectRolesByGroup called with empty groups")
		return make(map[string]string), nil
	}

	allProjects, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("checking OIDC groups against projects",
		"user_groups_count", len(userGroups),
		"projects_count", len(allProjects.Items),
	)

	// Create a set of user groups for O(1) lookup
	userGroupSet := make(map[string]struct{}, len(userGroups))
	for _, group := range userGroups {
		userGroupSet[group] = struct{}{}
	}

	projectRoles := make(map[string]string)
	for i := range allProjects.Items {
		project := &allProjects.Items[i]

		// Log the project's configured role groups for debugging
		var configuredGroups []string
		for _, role := range project.Spec.Roles {
			for _, g := range role.Groups {
				configuredGroups = append(configuredGroups, g)
			}
		}
		if len(configuredGroups) > 0 {
			slog.Debug("project role groups configuration",
				"project", project.Name,
				"configured_groups", configuredGroups,
			)
		}

		// Check each role to see if any user group matches
		for _, role := range project.Spec.Roles {
			for _, roleGroup := range role.Groups {
				if _, ok := userGroupSet[roleGroup]; ok {
					// Found a matching group, record this role for the project
					// First matching role wins (maintains deterministic behavior)
					if _, exists := projectRoles[project.Name]; !exists {
						slog.Info("OIDC group matched project role",
							"project", project.Name,
							"role", role.Name,
							"matched_group", roleGroup,
						)
						projectRoles[project.Name] = role.Name
					}
					break // Move to next role (we already found a match for this role)
				}
			}
		}
	}

	if len(projectRoles) == 0 {
		slog.Warn("no OIDC groups matched any project roles",
			"user_groups_count", len(userGroups),
			"projects_checked", len(allProjects.Items),
		)
	}

	return projectRoles, nil
}
