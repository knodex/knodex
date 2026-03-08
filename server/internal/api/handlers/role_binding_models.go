// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

// RoleBinding represents a user or group assignment to a project role
type RoleBinding struct {
	// Role is the role name within the project
	Role string `json:"role"`
	// Subject is the user email/ID or group name
	Subject string `json:"subject"`
	// Type indicates whether this is a "user" or "group" binding
	Type string `json:"type"`
}

// RoleBindingResponse represents a successful role assignment response
type RoleBindingResponse struct {
	// Project is the project name
	Project string `json:"project"`
	// Role is the role name
	Role string `json:"role"`
	// Subject is the user or group identifier
	Subject string `json:"subject"`
	// Type is "user" or "group"
	Type string `json:"type"`
}

// ListRoleBindingsResponse represents a list of role bindings for a project
type ListRoleBindingsResponse struct {
	// Project is the project name
	Project string `json:"project"`
	// Bindings is the list of role bindings
	Bindings []RoleBinding `json:"bindings"`
}
