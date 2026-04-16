// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package integration

import "github.com/knodex/knodex/server/internal/rbac"

// Test user constants
const (
	GlobalAdminID    = "user-global-admin"
	GlobalAdminEmail = "admin@example.com"

	RegularUserID    = "user-regular"
	RegularUserEmail = "user@example.com"

	ProjectAdminID    = "user-project-admin"
	ProjectAdminEmail = "project-admin@example.com"

	ViewerUserID    = "user-viewer"
	ViewerUserEmail = "viewer@example.com"
)

// Test project constants
const (
	TestProject1Name = "test-project-1"
	TestProject2Name = "test-project-2"
	DefaultProject   = "default"
)

// FixtureProject represents a test project fixture
type FixtureProject struct {
	Name         string
	Description  string
	Roles        []rbac.ProjectRole
	Destinations []rbac.Destination
}

// CreateProjectRequestBody creates a request body for project creation
type CreateProjectRequestBody struct {
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	Destinations []DestinationRequest `json:"destinations,omitempty"`
}

// DestinationRequest represents a deployment destination in requests
type DestinationRequest struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// UpdateProjectRequestBody creates a request body for project update
type UpdateProjectRequestBody struct {
	Description     string               `json:"description,omitempty"`
	Destinations    []DestinationRequest `json:"destinations,omitempty"`
	ResourceVersion string               `json:"resourceVersion"`
}

// ValidateProjectRequestBody creates a request body for project validation
type ValidateProjectRequestBody struct {
	Project *ProjectToValidate `json:"project"`
}

// ProjectToValidate represents project data for validation
type ProjectToValidate struct {
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	Roles        []RoleToValidate     `json:"roles,omitempty"`
}

// RoleToValidate represents role data for validation
type RoleToValidate struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Policies     []string `json:"policies,omitempty"`
	Groups       []string `json:"groups,omitempty"`
	Destinations []string `json:"destinations,omitempty"`
}

// ValidateUpdateRequestBody creates a request body for project update validation
type ValidateUpdateRequestBody struct {
	Description  *string              `json:"description,omitempty"`
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	Roles        []RoleToValidate     `json:"roles,omitempty"`
}

// ProjectResponse represents the response from project API endpoints
type ProjectResponse struct {
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	Destinations    []DestinationResponse `json:"destinations,omitempty"`
	Roles           []RoleResponse        `json:"roles,omitempty"`
	ResourceVersion string                `json:"resourceVersion"`
	CreatedAt       string                `json:"createdAt"`
	CreatedBy       string                `json:"createdBy"`
	UpdatedBy       string                `json:"updatedBy"`
}

// DestinationResponse represents a deployment destination in responses
type DestinationResponse struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// RoleResponse represents a role in responses
type RoleResponse struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Policies     []string `json:"policies,omitempty"`
	Groups       []string `json:"groups,omitempty"`
	Destinations []string `json:"destinations,omitempty"`
}

// ProjectListResponse represents the response from list projects endpoint
type ProjectListResponse struct {
	Items      []ProjectResponse `json:"items"`
	TotalCount int               `json:"totalCount"`
}

// ValidationResponse represents the response from validation endpoints
type ValidationResponse struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// RoleBindingListResponse represents the response from list role bindings endpoint
type RoleBindingListResponse struct {
	Bindings []RoleBinding `json:"bindings"`
}

// RoleBinding represents a role binding in list responses
type RoleBinding struct {
	Role    string `json:"role"`
	Subject string `json:"subject"`
	Type    string `json:"type"`
}

// RoleBindingResponse represents a successful role assignment response
type RoleBindingAssignmentResponse struct {
	Project string `json:"project"`
	Role    string `json:"role"`
	Subject string `json:"subject"`
	Type    string `json:"type"`
}

// ListRoleBindingsResponse represents a list of role bindings for a project
type ListRoleBindingsResponseIntegration struct {
	Project  string        `json:"project"`
	Bindings []RoleBinding `json:"bindings"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

// Default test destination for projects
var DefaultTestDestination = rbac.Destination{
	Namespace: "default",
}

// Fixture projects for testing
var TestProjectWithRoles = FixtureProject{
	Name:        TestProject1Name,
	Description: "Test project 1 with roles",
	Roles: []rbac.ProjectRole{
		{
			Name:        "admin",
			Description: "Project administrator",
			Policies: []string{
				"p, proj:test-project-1:admin, test-project-1, *",
			},
		},
		{
			Name:        "developer",
			Description: "Developer role",
			Policies: []string{
				"p, proj:test-project-1:developer, test-project-1, get",
				"p, proj:test-project-1:developer, test-project-1, list",
			},
		},
		{
			Name:        "viewer",
			Description: "Viewer role",
			Policies: []string{
				"p, proj:test-project-1:viewer, test-project-1, get",
			},
		},
	},
	Destinations: []rbac.Destination{
		{Namespace: "test-project-1"},
	},
}

var TestProjectMinimal = FixtureProject{
	Name:        TestProject2Name,
	Description: "Minimal test project",
	Roles:       []rbac.ProjectRole{},
	Destinations: []rbac.Destination{
		{Namespace: "test-project-2"},
	},
}

// CreateTestProjectBody creates a create project request body from a fixture
func CreateTestProjectBody(fixture FixtureProject) CreateProjectRequestBody {
	destinations := make([]DestinationRequest, len(fixture.Destinations))
	for i, dest := range fixture.Destinations {
		destinations[i] = DestinationRequest{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		}
	}

	return CreateProjectRequestBody{
		Name:         fixture.Name,
		Description:  fixture.Description,
		Destinations: destinations,
	}
}

// CreateValidationBody creates a validation request body from a fixture
func CreateValidationBody(fixture FixtureProject) ValidateProjectRequestBody {
	roles := make([]RoleToValidate, len(fixture.Roles))
	for i, role := range fixture.Roles {
		roles[i] = RoleToValidate{
			Name:        role.Name,
			Description: role.Description,
			Policies:    role.Policies,
			Groups:      role.Groups,
		}
	}

	destinations := make([]DestinationRequest, len(fixture.Destinations))
	for i, dest := range fixture.Destinations {
		destinations[i] = DestinationRequest{
			Namespace: dest.Namespace,
			Name:      dest.Name,
		}
	}

	return ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:         fixture.Name,
			Description:  fixture.Description,
			Destinations: destinations,
			Roles:        roles,
		},
	}
}
