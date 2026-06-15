// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// orgAdminTestRequest builds a request carrying the given user context (nil for none).
func orgAdminTestRequest(userCtx *UserContext) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/acme/billing/portal-session", nil)
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

func TestOrgAdminRequired(t *testing.T) {
	t.Parallel()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("org admin passes through", func(t *testing.T) {
		// Story 12.2: org-admin == serveradmin == reserved `admins` team, one tier.
		// The admin signal is role:serveradmin in CasbinRoles (granted at login via
		// the operator globalAdmin mapping for kx-team-<org>-admins), NOT a group parse.
		nextCalled = false
		rr := httptest.NewRecorder()
		req := orgAdminTestRequest(&UserContext{
			UserID:      "oidc:ada",
			CasbinRoles: []string{"role:serveradmin"},
		})
		OrgAdminRequired(next).ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("expected next handler to be called for org admin")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("non-admin is rejected with 403 ORG_ADMIN_REQUIRED", func(t *testing.T) {
		// A non-admin org member carries a team group but NOT role:serveradmin.
		nextCalled = false
		rr := httptest.NewRecorder()
		req := orgAdminTestRequest(&UserContext{
			UserID:      "oidc:grace",
			Groups:      []string{"kx-team-acme-platform-eng"},
			CasbinRoles: []string{"proj:acme:developer"},
		})
		OrgAdminRequired(next).ServeHTTP(rr, req)

		if nextCalled {
			t.Error("expected next handler NOT to be called for non-admin")
		}
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode error body: %v", err)
		}
		if body.Code != "ORG_ADMIN_REQUIRED" {
			t.Errorf("expected code ORG_ADMIN_REQUIRED, got %q", body.Code)
		}
	})

	t.Run("missing user context is rejected with 401", func(t *testing.T) {
		nextCalled = false
		rr := httptest.NewRecorder()
		req := orgAdminTestRequest(nil)
		OrgAdminRequired(next).ServeHTTP(rr, req)

		if nextCalled {
			t.Error("expected next handler NOT to be called without user context")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}
