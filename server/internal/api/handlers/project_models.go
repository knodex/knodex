package handlers

import (
	"time"

	"github.com/knodex/knodex/server/internal/rbac"
)

// CreateProjectRequest represents the request body for creating a project
type CreateProjectRequest struct {
	// Name is the unique identifier for the project (DNS-1123 subdomain format)
	Name string `json:"name"`
	// Description is a human-readable description of the project
	Description string `json:"description,omitempty"`
	// Destinations defines allowed deployment destinations
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	// Roles defines initial roles to create with the project
	Roles []RoleRequest `json:"roles,omitempty"`
}

// DestinationRequest represents a deployment destination in requests
type DestinationRequest struct {
	// Namespace is the target namespace (supports wildcards like "*", "dev-*")
	Namespace string `json:"namespace,omitempty"`
	// Name is an optional friendly name for the destination
	Name string `json:"name,omitempty"`
}

// UpdateProjectRequest represents the request body for updating a project
type UpdateProjectRequest struct {
	// Description is a human-readable description of the project
	Description string `json:"description,omitempty"`
	// Destinations defines allowed deployment destinations
	Destinations []DestinationRequest `json:"destinations,omitempty"`
	// Roles is the list of roles to update (project admins can update roles)
	Roles []RoleRequest `json:"roles,omitempty"`
	// ResourceVersion is required for optimistic locking
	ResourceVersion string `json:"resourceVersion"`
}

// RoleRequest represents a role in API requests
type RoleRequest struct {
	// Name is the unique name of the role within the project
	Name string `json:"name"`
	// Description is a human-readable description of the role
	Description string `json:"description,omitempty"`
	// Policies is the list of Casbin policy strings defining permissions
	// Format: p, proj:{project}:{role}, {resource}, {action}, {object}, {effect}
	Policies []string `json:"policies,omitempty"`
	// Groups is the list of OIDC groups assigned to this role
	Groups []string `json:"groups,omitempty"`
}

// ProjectResponse represents a project in API responses
type ProjectResponse struct {
	// Name is the unique identifier for the project
	Name string `json:"name"`
	// Description is a human-readable description of the project
	Description string `json:"description,omitempty"`
	// Destinations defines allowed deployment destinations
	Destinations []DestinationResponse `json:"destinations,omitempty"`
	// Roles is the list of roles defined in this project
	Roles []RoleResponse `json:"roles,omitempty"`
	// ResourceVersion is the version used for optimistic locking
	ResourceVersion string `json:"resourceVersion"`
	// CreatedAt is when the project was created
	CreatedAt time.Time `json:"createdAt"`
	// CreatedBy is who created the project
	CreatedBy string `json:"createdBy,omitempty"`
	// UpdatedAt is when the project was last updated
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	// UpdatedBy is who last updated the project
	UpdatedBy string `json:"updatedBy,omitempty"`
}

// DestinationResponse represents a deployment destination in responses
type DestinationResponse struct {
	// Namespace is the target namespace
	Namespace string `json:"namespace,omitempty"`
	// Name is an optional friendly name for the destination
	Name string `json:"name,omitempty"`
}

// RoleResponse represents a role in API responses
type RoleResponse struct {
	// Name is the unique name of the role within the project
	Name string `json:"name"`
	// Description is a human-readable description of the role
	Description string `json:"description,omitempty"`
	// Policies is the list of policy strings defining permissions
	Policies []string `json:"policies,omitempty"`
	// Groups is the list of OIDC groups assigned to this role
	Groups []string `json:"groups,omitempty"`
}

// ProjectListResponse represents a list of projects in API responses
type ProjectListResponse struct {
	// Items is the list of projects
	Items []ProjectResponse `json:"items"`
	// TotalCount is the total number of projects
	TotalCount int `json:"totalCount"`
}

// toProjectResponse converts an rbac.Project to a ProjectResponse
func toProjectResponse(p *rbac.Project) ProjectResponse {
	resp := ProjectResponse{
		Name:            p.Name,
		Description:     p.Spec.Description,
		Destinations:    make([]DestinationResponse, 0, len(p.Spec.Destinations)),
		Roles:           make([]RoleResponse, 0, len(p.Spec.Roles)),
		ResourceVersion: p.ResourceVersion,
		CreatedAt:       p.CreationTimestamp.Time,
	}

	// Convert destinations
	for _, d := range p.Spec.Destinations {
		resp.Destinations = append(resp.Destinations, DestinationResponse{
			Namespace: d.Namespace,
			Name:      d.Name,
		})
	}

	// Convert roles
	for _, r := range p.Spec.Roles {
		resp.Roles = append(resp.Roles, RoleResponse{
			Name:        r.Name,
			Description: r.Description,
			Policies:    r.Policies,
			Groups:      r.Groups,
		})
	}

	// Extract metadata from annotations
	if p.Annotations != nil {
		if createdBy, ok := p.Annotations["knodex.io/created-by"]; ok {
			resp.CreatedBy = createdBy
		}
		if updatedBy, ok := p.Annotations["knodex.io/updated-by"]; ok {
			resp.UpdatedBy = updatedBy
		}
		if updatedAt, ok := p.Annotations["knodex.io/updated-at"]; ok {
			if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				resp.UpdatedAt = &t
			}
		}
	}

	return resp
}

// toProjectSpec converts a CreateProjectRequest to an rbac.ProjectSpec
func toProjectSpec(req *CreateProjectRequest) rbac.ProjectSpec {
	spec := rbac.ProjectSpec{
		Description:  req.Description,
		Destinations: make([]rbac.Destination, 0, len(req.Destinations)),
		Roles:        make([]rbac.ProjectRole, 0, len(req.Roles)),
	}

	for _, d := range req.Destinations {
		spec.Destinations = append(spec.Destinations, rbac.Destination{
			Namespace: d.Namespace,
			Name:      d.Name,
		})
	}

	for _, r := range req.Roles {
		spec.Roles = append(spec.Roles, rbac.ProjectRole{
			Name:        r.Name,
			Description: r.Description,
			Policies:    r.Policies,
			Groups:      r.Groups,
		})
	}

	return spec
}

// applyUpdateToProject applies an UpdateProjectRequest to an existing Project
// Only updates fields that are explicitly provided in the request (partial update pattern)
func applyUpdateToProject(project *rbac.Project, req *UpdateProjectRequest) {
	// Update description if provided (empty string is a valid update)
	// Note: Description is always updated as it can legitimately be empty
	project.Spec.Description = req.Description

	// Update destinations only if provided
	if len(req.Destinations) > 0 {
		project.Spec.Destinations = make([]rbac.Destination, 0, len(req.Destinations))
		for _, d := range req.Destinations {
			project.Spec.Destinations = append(project.Spec.Destinations, rbac.Destination{
				Namespace: d.Namespace,
				Name:      d.Name,
			})
		}
	}

	// Update roles if provided
	// Note: Only updates roles that are provided in the request
	// Project admins can update role policies and groups
	if len(req.Roles) > 0 {
		project.Spec.Roles = make([]rbac.ProjectRole, 0, len(req.Roles))
		for _, r := range req.Roles {
			project.Spec.Roles = append(project.Spec.Roles, rbac.ProjectRole{
				Name:        r.Name,
				Description: r.Description,
				Policies:    r.Policies,
				Groups:      r.Groups,
			})
		}
	}
}
