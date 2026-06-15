// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// orgAdminErrorCode is the stable error code returned when a non-admin user
// attempts an org-admin-only action (architecture :784).
const orgAdminErrorCode response.ErrorCode = "ORG_ADMIN_REQUIRED"

// OrgAdminRequired gates a route to org admins only (D-6). It reads the calling
// user's org-admin status from the authenticated user context — the same signal
// surfaced in GET /api/v1/account/info — and rejects non-admins with 403
// ORG_ADMIN_REQUIRED.
//
// This is the REAL per-user enforcement for mutating cloud-mode routes: the
// tenant proxies the control plane with a single org-scoped API key, so the
// control plane cannot see the individual end user (story 8.6 › Decision 3).
// It MUST run AFTER the Auth middleware, which populates the user context.
//
// Story 12.2 (ADR adr-cloud-team-membership-keycloak-groups): org-admin ==
// serveradmin == reserved `admins` team member collapse into ONE privileged
// tier. The admin signal is membership of role:serveradmin in CasbinRoles —
// granted at login via the operator globalAdmin mapping for the reserved
// kx-team-<org>-admins group (a real Keycloak group flowing through 12.1's
// native mapper). There is no second group-parse layer (NFR-T1): this gate just
// reads the already-assigned Casbin role. The kx-org-<slug>-serveradmin parse
// was retired with this story.
func OrgAdminRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, ok := GetUserContext(r)
		if !ok || userCtx == nil {
			response.Unauthorized(w, "authentication required")
			return
		}

		if !collection.Contains(userCtx.CasbinRoles, rbac.CasbinRoleServerAdmin) {
			response.WriteError(w, http.StatusForbidden, orgAdminErrorCode,
				"organization admin privileges required", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
