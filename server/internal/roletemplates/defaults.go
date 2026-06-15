// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package roletemplates

// DefaultRoleTemplates returns the three built-in project-role templates used
// when the catalog ConfigMap is absent or empty, preserving the out-of-the-box
// UX with zero config.
//
// The policy strings are copied VERBATIM from web/src/lib/role-presets.ts
// (ADMIN_POLICIES, developer, readonly) and keep the {project}/{role}
// placeholders. The enterprise-only compliance policy
// ("compliance, get, {project}/*") is deliberately NOT included here: it is
// injected client-side at apply time for the `admin` template only when
// isEnterprise() (resolvePresetPolicies). OSS operators must not see or store
// an enterprise-only policy in the catalog.
//
// Returns a fresh slice on every call so callers can mutate it safely.
func DefaultRoleTemplates() []RoleTemplate {
	return []RoleTemplate{
		{
			Name:        "admin",
			Label:       "Admin",
			Description: "Full project management access",
			Policies: []string{
				"p, proj:{project}:{role}, projects, *, {project}, allow",
				"p, proj:{project}:{role}, instances, *, */{project}/*, allow",
				"p, proj:{project}:{role}, rgds, get, *, allow",
				"p, proj:{project}:{role}, rgds, list, *, allow",
				"p, proj:{project}:{role}, repositories, *, {project}/*, allow",
			},
		},
		{
			Name:        "developer",
			Label:       "Developer",
			Description: "Deploy and manage instances",
			Policies: []string{
				"p, proj:{project}:{role}, projects, get, {project}, allow",
				"p, proj:{project}:{role}, instances, *, */{project}/*, allow",
				"p, proj:{project}:{role}, rgds, get, *, allow",
				"p, proj:{project}:{role}, rgds, list, *, allow",
				"p, proj:{project}:{role}, repositories, get, {project}/*, allow",
				"p, proj:{project}:{role}, repositories, list, {project}/*, allow",
			},
		},
		{
			Name:        "readonly",
			Label:       "Readonly",
			Description: "View-only access to project resources",
			Policies: []string{
				"p, proj:{project}:{role}, projects, get, {project}, allow",
				"p, proj:{project}:{role}, instances, get, */{project}/*, allow",
				"p, proj:{project}:{role}, instances, list, */{project}/*, allow",
				"p, proj:{project}:{role}, rgds, get, *, allow",
				"p, proj:{project}:{role}, rgds, list, *, allow",
				"p, proj:{project}:{role}, repositories, get, {project}/*, allow",
				"p, proj:{project}:{role}, repositories, list, {project}/*, allow",
			},
		},
	}
}
