// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

// Permission represents an authorization permission
type Permission string

const (
	// Project permissions
	PermissionProjectCreate       Permission = "project:create"
	PermissionProjectRead         Permission = "project:read"
	PermissionProjectUpdate       Permission = "project:update"
	PermissionProjectDelete       Permission = "project:delete"
	PermissionProjectForceDelete  Permission = "project:force-delete"
	PermissionProjectMemberAdd    Permission = "project:member:add"
	PermissionProjectMemberRemove Permission = "project:member:remove"
	PermissionProjectMemberUpdate Permission = "project:member:update"

	// Repository permissions
	PermissionRepoManage Permission = "repo:manage"
)
