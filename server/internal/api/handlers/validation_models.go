package handlers

import "github.com/knodex/knodex/server/internal/rbac"

// ValidateProjectRequest represents the request body for validating a new project
type ValidateProjectRequest struct {
	// Project is the full project specification to validate
	Project *ProjectToValidate `json:"project"`
}

// ProjectToValidate represents a project for validation (without full K8s metadata)
type ProjectToValidate struct {
	// Name is the unique identifier for the project (DNS-1123 subdomain format)
	Name string `json:"name"`
	// Description is a human-readable description of the project
	Description string `json:"description,omitempty"`
	// Destinations defines allowed deployment destinations
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	// Roles is the list of roles defined in this project
	Roles []RoleToValidate `json:"roles,omitempty"`
}

// RoleToValidate represents a role for validation
type RoleToValidate struct {
	// Name is the unique name of the role within the project
	Name string `json:"name"`
	// Description is a human-readable description of the role
	Description string `json:"description,omitempty"`
	// Policies is the list of policy strings defining permissions
	Policies []string `json:"policies,omitempty"`
	// Groups is the list of OIDC groups assigned to this role
	Groups []string `json:"groups,omitempty"`
}

// ValidateProjectUpdateRequest represents the request body for validating project updates
type ValidateProjectUpdateRequest struct {
	// Description is a human-readable description of the project
	Description *string `json:"description,omitempty"`
	// Destinations defines allowed deployment destinations
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	// Roles is the list of roles defined in this project
	Roles []RoleToValidate `json:"roles,omitempty"`
}

// ValidationResult represents the outcome of policy validation
type ValidationResult struct {
	// Valid indicates whether the project/policies are valid
	Valid bool `json:"valid"`
	// Errors is a list of validation errors and warnings
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a single validation error or warning
type ValidationError struct {
	// Field is the JSON path to the field with the error
	Field string `json:"field"`
	// Message is a human-readable error message
	Message string `json:"message"`
	// Severity indicates if this is an "error" or "warning"
	Severity string `json:"severity"`
}

// toRbacProject converts a ProjectToValidate to an rbac.Project for validation
func toRbacProject(p *ProjectToValidate) *rbac.Project {
	if p == nil {
		return nil
	}

	project := &rbac.Project{}
	project.Name = p.Name
	project.Spec = rbac.ProjectSpec{
		Description:  p.Description,
		Destinations: make([]rbac.Destination, 0, len(p.Destinations)),
		Roles:        make([]rbac.ProjectRole, 0, len(p.Roles)),
	}

	// Convert destinations
	for _, d := range p.Destinations {
		project.Spec.Destinations = append(project.Spec.Destinations, rbac.Destination{
			Namespace: d.Namespace,
			Name:      d.Name,
		})
	}

	// Convert roles
	for _, r := range p.Roles {
		project.Spec.Roles = append(project.Spec.Roles, rbac.ProjectRole{
			Name:        r.Name,
			Description: r.Description,
			Policies:    r.Policies,
			Groups:      r.Groups,
		})
	}

	return project
}

// applyValidationUpdateToProject applies update request fields to an existing project
func applyValidationUpdateToProject(current *rbac.Project, req *ValidateProjectUpdateRequest) *rbac.Project {
	// Deep copy current project using DeepCopyObject and cast
	updated := current.DeepCopyObject().(*rbac.Project)

	// Apply updates
	if req.Description != nil {
		updated.Spec.Description = *req.Description
	}
	if req.Destinations != nil {
		updated.Spec.Destinations = make([]rbac.Destination, 0, len(req.Destinations))
		for _, d := range req.Destinations {
			updated.Spec.Destinations = append(updated.Spec.Destinations, rbac.Destination{
				Namespace: d.Namespace,
				Name:      d.Name,
			})
		}
	}
	if req.Roles != nil {
		updated.Spec.Roles = make([]rbac.ProjectRole, 0, len(req.Roles))
		for _, r := range req.Roles {
			updated.Spec.Roles = append(updated.Spec.Roles, rbac.ProjectRole{
				Name:        r.Name,
				Description: r.Description,
				Policies:    r.Policies,
				Groups:      r.Groups,
			})
		}
	}

	return updated
}
