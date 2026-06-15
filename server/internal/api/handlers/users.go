// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/env"
)

const (
	// defaultUsersPageSize / maxUsersPageSize bound the GET /api/v1/users
	// ?limit param (FR-U5). The store clamps internally too, but FR-U5 mandates
	// an explicit 400 on out-of-range, so the handler validates before calling.
	defaultUsersPageSize = 50
	minUsersPageSize     = 1
	maxUsersPageSize     = 200

	// defaultInactiveThresholdDays is the IDENTITY_INACTIVE_THRESHOLD_DAYS default
	// used to derive the display-only isInactive flag (FR-U14).
	defaultInactiveThresholdDays = 30

	// reclaimNote is the FR-U13/R5-4 reclaim-semantics note carried in the DELETE
	// response body (a 204 cannot carry it — see story Dev Notes).
	reclaimNote = "Seat reclaimed. Permanent exclusion requires IdP-side revocation."
)

// FederatedIdentityResponse is the API representation of one federated identity.
// It deliberately OMITS externalId / sourceConnectorId — IdP-side opaque ids of
// the same privacy class as sub, kept off the public API surface (NFR-U2). They
// remain available internally for the future SCIM epic.
type FederatedIdentityResponse struct {
	Issuer       string    `json:"issuer"`
	Sub          string    `json:"sub"`
	ProviderKind string    `json:"providerKind"`
	SourceKind   string    `json:"sourceKind"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// UserResponse is the API representation of a canonical identity.users row with
// its federated identities expanded and the display-only isInactive flag computed.
type UserResponse struct {
	ID                  string                      `json:"id"`
	Email               string                      `json:"email"`
	DisplayName         string                      `json:"displayName"`
	State               string                      `json:"state"`
	IsInactive          bool                        `json:"isInactive"`
	FirstSeenAt         time.Time                   `json:"firstSeenAt"`
	LastSeenAt          time.Time                   `json:"lastSeenAt"`
	FederatedIdentities []FederatedIdentityResponse `json:"federatedIdentities"`
	// ApplicationRole is the user's effective two-axis application role, derived
	// from Casbin: "serveradmin" iff the user effectively holds role:serveradmin,
	// else "member" (Epic 17 / Story 17.3; same conceptual derivation as 17.1's
	// self view, queried per-roster-user instead of from the caller's JWT).
	// "member" is a DISPLAY value, NOT a Casbin subject — no role:member subject
	// or policy is created (NFR-T1, single enforcement layer).
	ApplicationRole string `json:"applicationRole"`
}

// UsersListResponse wraps a keyset-paginated page for GET /api/v1/users.
type UsersListResponse struct {
	Users         []UserResponse `json:"users"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

// DeleteUserResponse is the minimal reclaim-semantics body for DELETE
// /api/v1/users/{id} (FR-U13 / R5-4).
type DeleteUserResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Note  string `json:"note"`
}

// UsersHandler serves the operator-gated Users API (Story 15.8) over the
// canonical identity.users roster Story 15.2 persists.
//
// Users are an operator surface, NOT an authorization resource: every method is
// gated with the SAME settings/* get|update Casbin check used by Teams (10.4)
// and observed-groups (10.3) — no `users` can-i resource, no per-user permission
// check (NFR-U3 / NFR-T1 / Locked Decision #5, the single enforcement layer).
type UsersHandler struct {
	identitySvc           services.IdentityService
	enforcer              rbac.Authorizer
	inactiveThresholdDays int
	nowFn                 func() time.Time
}

// NewUsersHandler creates a Users API handler. The inactive threshold is read
// from IDENTITY_INACTIVE_THRESHOLD_DAYS (default 30); nowFn defaults to time.Now
// (overridable in tests for deterministic isInactive assertions).
func NewUsersHandler(svc services.IdentityService, enforcer rbac.Authorizer) *UsersHandler {
	return &UsersHandler{
		identitySvc:           svc,
		enforcer:              enforcer,
		inactiveThresholdDays: env.GetInt("IDENTITY_INACTIVE_THRESHOLD_DAYS", defaultInactiveThresholdDays),
		nowFn:                 time.Now,
	}
}

// requireOperator gates a handler with the shared settings/* Casbin check.
// action is "get" for reads, "update" for mutations. Returns the user context
// when access is granted, or nil after writing the 401/403/500 response.
func (h *UsersHandler) requireOperator(w http.ResponseWriter, r *http.Request, action string) *middleware.UserContext {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return nil
	}
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", action, r.Header.Get("X-Request-ID")) {
		return nil
	}
	return userCtx
}

// toUserResponse maps a UserRecord + its federated identities into the API DTO,
// computing the display-only isInactive flag against now and the derived
// application role against Casbin. ctx is threaded for the per-user role lookup.
func (h *UsersHandler) toUserResponse(ctx context.Context, u *services.UserRecord, feds []services.FederatedIdentity) UserResponse {
	fedResp := make([]FederatedIdentityResponse, 0, len(feds))
	for _, f := range feds {
		fedResp = append(fedResp, FederatedIdentityResponse{
			Issuer:       f.Issuer,
			Sub:          f.Sub,
			ProviderKind: f.ProviderKind,
			SourceKind:   f.SourceKind,
			CreatedAt:    f.CreatedAt,
			UpdatedAt:    f.UpdatedAt,
		})
	}
	return UserResponse{
		ID:                  string(u.ID),
		Email:               u.Email,
		DisplayName:         u.DisplayName,
		State:               u.State,
		IsInactive:          h.isInactive(u.LastSeenAt),
		FirstSeenAt:         u.FirstSeenAt,
		LastSeenAt:          u.LastSeenAt,
		FederatedIdentities: fedResp,
		ApplicationRole:     h.applicationRole(ctx, feds),
	}
}

// applicationRole returns "serveradmin" if ANY of the user's federated
// identities resolves to a Casbin subject holding role:serveradmin, else
// "member". "member" is a derived display value — never a Casbin subject
// (NFR-T1). This mirrors 17.1's predicate (serveradmin iff role:serveradmin,
// else member) but queries the enforcer per-roster-user rather than reading the
// caller's JWT, which is meaningless for other users.
//
// The Casbin subject for an OIDC user is DERIVED, not stored:
// fed.Sub is the stored "<provider>:<rawSub>" oidcSubject, and the login-time
// grant attaches role:serveradmin directly to auth.GenerateOIDCUserID(fed.Sub).
// HasRole does NOT normalize the subject prefix, so — mirroring
// CanAccessWithGroups' alt-prefix robustness — we check both the raw stored
// form and the "user:"-prefixed form.
func (h *UsersHandler) applicationRole(ctx context.Context, feds []services.FederatedIdentity) string {
	for _, f := range feds {
		if f.Sub == "" {
			continue // SCIM-pushed row that has not logged in (R5-6).
		}
		casbinUserID := auth.GenerateOIDCUserID(f.Sub)
		for _, subject := range []string{casbinUserID, "user:" + casbinUserID} {
			ok, err := h.enforcer.HasRole(ctx, subject, rbac.CasbinRoleServerAdmin)
			if err != nil {
				// AC#3: a per-user lookup error degrades to "member" (logged);
				// it MUST NOT turn the whole roster into a 500.
				slog.Warn("application-role lookup failed; degrading to member",
					"subject", subject, "error", err)
				continue
			}
			if ok {
				return "serveradmin"
			}
		}
	}
	return "member"
}

// isInactive reports whether lastSeenAt is older than the configured inactivity
// threshold. Display-only — orthogonal to state and never participates in
// billing (FR-U14).
func (h *UsersHandler) isInactive(lastSeenAt time.Time) bool {
	cutoff := h.nowFn().AddDate(0, 0, -h.inactiveThresholdDays)
	return lastSeenAt.Before(cutoff)
}

// ListUsers handles GET /api/v1/users — keyset-paginated, federated identities
// expanded, isInactive computed (FR-U5/FR-U14).
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}

	limit, ok := parseLimit(w, r.URL.Query().Get("limit"))
	if !ok {
		return
	}

	page, nextToken, err := h.identitySvc.List(r.Context(), services.ListOpts{
		PageSize:  limit,
		PageToken: r.URL.Query().Get("pageToken"),
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidPageToken) {
			response.BadRequest(w, "invalid pageToken", nil)
			return
		}
		response.InternalError(w, "failed to list users")
		return
	}

	ids := make([]services.UserID, 0, len(page))
	for _, u := range page {
		ids = append(ids, u.ID)
	}
	feds, err := h.identitySvc.FederatedIdentitiesFor(r.Context(), ids)
	if err != nil {
		response.InternalError(w, "failed to expand federated identities")
		return
	}

	users := make([]UserResponse, 0, len(page))
	for _, u := range page {
		users = append(users, h.toUserResponse(r.Context(), u, feds[u.ID]))
	}
	response.WriteJSON(w, http.StatusOK, UsersListResponse{Users: users, NextPageToken: nextToken})
}

// GetUser handles GET /api/v1/users/{id} (FR-U6).
func (h *UsersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}

	id := r.PathValue("id")
	u, err := h.identitySvc.GetByID(r.Context(), services.UserID(id))
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.NotFound(w, "user", id)
			return
		}
		response.InternalError(w, "failed to get user")
		return
	}

	feds, err := h.identitySvc.FederatedIdentitiesFor(r.Context(), []services.UserID{u.ID})
	if err != nil {
		response.InternalError(w, "failed to expand federated identities")
		return
	}
	response.WriteJSON(w, http.StatusOK, h.toUserResponse(r.Context(), u, feds[u.ID]))
}

// DeleteUser handles DELETE /api/v1/users/{id} — soft-delete / seat reclaim
// (FR-U13 / R5-4). Returns 200 + a reclaim-semantics note (a 204 cannot carry
// it). The underlying Remove is idempotent and drops the BilledSeatCount.
func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "update") == nil {
		return
	}

	id := r.PathValue("id")
	if err := h.identitySvc.Remove(r.Context(), services.UserID(id)); err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			response.NotFound(w, "user", id)
			return
		}
		response.InternalError(w, "failed to remove user")
		return
	}
	response.WriteJSON(w, http.StatusOK, DeleteUserResponse{
		ID:    id,
		State: services.UserStateRemoved,
		Note:  reclaimNote,
	})
}

// parseLimit validates the ?limit query param. Empty → default. Non-integer or
// out of [min,max] → writes 400 and returns ok=false (FR-U5 explicit reject, not
// silent clamp).
func parseLimit(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return defaultUsersPageSize, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		response.BadRequest(w, "limit must be an integer", map[string]string{"limit": raw})
		return 0, false
	}
	if n < minUsersPageSize || n > maxUsersPageSize {
		response.BadRequest(w, "limit out of range (1..200)", map[string]string{"limit": raw})
		return 0, false
	}
	return n, true
}
