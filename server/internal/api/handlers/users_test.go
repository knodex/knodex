// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
)

// fixedNow anchors the isInactive computation so tests are deterministic.
var fixedNow = time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)

// fakeIdentityService is an in-memory services.IdentityService for handler tests.
// Only the methods the Users API exercises (List, GetByID, Remove,
// FederatedIdentitiesFor) carry behaviour; the rest are inert.
type fakeIdentityService struct {
	listPage     []*services.UserRecord
	listNext     string
	listErr      error
	lastListOpts services.ListOpts

	byID   map[services.UserID]*services.UserRecord
	getErr error

	feds    map[services.UserID][]services.FederatedIdentity
	fedsErr error

	removeErr error
	removed   []services.UserID
}

func (f *fakeIdentityService) ObserveLogin(context.Context, services.ObserveLoginParams) (services.ObserveLoginResult, error) {
	return services.ObserveLoginResult{}, nil
}
func (f *fakeIdentityService) Provision(context.Context, services.ProvisionParams) (services.ObserveLoginResult, error) {
	return services.ObserveLoginResult{}, services.ErrNotImplemented
}
func (f *fakeIdentityService) Deactivate(context.Context, services.UserID) error {
	return services.ErrNotImplemented
}
func (f *fakeIdentityService) GetByFederation(context.Context, string, string, string) (*services.UserRecord, error) {
	return nil, services.ErrUserNotFound
}
func (f *fakeIdentityService) BilledSeatCount(context.Context) (int64, error) { return 0, nil }

func (f *fakeIdentityService) List(_ context.Context, opts services.ListOpts) ([]*services.UserRecord, string, error) {
	f.lastListOpts = opts
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listPage, f.listNext, nil
}

func (f *fakeIdentityService) GetByID(_ context.Context, id services.UserID) (*services.UserRecord, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, services.ErrUserNotFound
}

func (f *fakeIdentityService) Remove(_ context.Context, id services.UserID) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removed = append(f.removed, id)
	return nil
}

func (f *fakeIdentityService) FederatedIdentitiesFor(_ context.Context, _ []services.UserID) (map[services.UserID][]services.FederatedIdentity, error) {
	if f.fedsErr != nil {
		return nil, f.fedsErr
	}
	if f.feds == nil {
		return map[services.UserID][]services.FederatedIdentity{}, nil
	}
	return f.feds, nil
}

// newTestUsersHandler builds a handler with a deterministic clock + 30d threshold.
func newTestUsersHandler(svc services.IdentityService, enf rbac.Authorizer) *UsersHandler {
	h := NewUsersHandler(svc, enf)
	h.inactiveThresholdDays = 30
	h.nowFn = func() time.Time { return fixedNow }
	return h
}

func usersReq(method, target string) *http.Request {
	return setAdminContext(httptest.NewRequest(method, target, nil))
}

func sampleUser(id string, lastSeen time.Time, state string) *services.UserRecord {
	return &services.UserRecord{
		ID:          services.UserID(id),
		OrgID:       "default",
		Email:       id + "@example.com",
		DisplayName: id,
		State:       state,
		FirstSeenAt: fixedNow.AddDate(0, 0, -100),
		LastSeenAt:  lastSeen,
	}
}

// ----- AC1 / AC12: list happy path + expansion + nextPageToken -----

func TestUsersHandler_List_HappyPath(t *testing.T) {
	t.Parallel()
	u := sampleUser("alice", fixedNow.AddDate(0, 0, -1), services.UserStateActive)
	svc := &fakeIdentityService{
		listPage: []*services.UserRecord{u},
		listNext: "eyJsIjoi...",
		feds: map[services.UserID][]services.FederatedIdentity{
			u.ID: {{
				OrgID:             "default",
				Issuer:            "https://idp.example.com",
				Sub:               "okta|abc123",
				ExternalID:        "EXTERNAL_SHOULD_NOT_LEAK",
				SourceConnectorID: "CONNECTOR_SHOULD_NOT_LEAK",
				InternalUserID:    u.ID,
				ProviderKind:      "oidc",
				SourceKind:        services.SourceKindOIDCJIT,
				CreatedAt:         fixedNow,
				UpdatedAt:         fixedNow,
			}},
		},
	}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(body.Users))
	}
	if body.NextPageToken != "eyJsIjoi..." {
		t.Errorf("nextPageToken not propagated: %q", body.NextPageToken)
	}
	if len(body.Users[0].FederatedIdentities) != 1 {
		t.Fatalf("expected federated identity expanded, got %d", len(body.Users[0].FederatedIdentities))
	}
	if body.Users[0].FederatedIdentities[0].Sub != "okta|abc123" {
		t.Errorf("federated sub mismatch: %+v", body.Users[0].FederatedIdentities[0])
	}
	// AC NFR-U2: IdP opaque ids must not appear in the API surface.
	if raw := w.Body.String(); strings.Contains(raw, "EXTERNAL_SHOULD_NOT_LEAK") ||
		strings.Contains(raw, "CONNECTOR_SHOULD_NOT_LEAK") ||
		strings.Contains(raw, "externalId") || strings.Contains(raw, "sourceConnectorId") {
		t.Errorf("PII floor violated — opaque IdP id leaked into response: %s", raw)
	}
}

func TestUsersHandler_List_EmptyOrgNeverNull(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{listPage: nil}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"users":[]`) {
		t.Errorf("expected users:[] (never null), got %s", w.Body.String())
	}
}

// ----- AC2 / AC12: limit validation + pageToken -----

func TestUsersHandler_List_LimitValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		limit      string
		wantStatus int
		wantPage   int // expected ListOpts.PageSize when 200
	}{
		{"default", "", http.StatusOK, 50},
		{"valid", "100", http.StatusOK, 100},
		{"min", "1", http.StatusOK, 1},
		{"max", "200", http.StatusOK, 200},
		{"zero", "0", http.StatusBadRequest, 0},
		{"negative", "-5", http.StatusBadRequest, 0},
		{"over-max", "201", http.StatusBadRequest, 0},
		{"non-integer", "abc", http.StatusBadRequest, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeIdentityService{}
			h := newTestUsersHandler(svc, operatorEnforcer())
			target := "/api/v1/users"
			if tc.limit != "" {
				target += "?limit=" + tc.limit
			}
			w := httptest.NewRecorder()
			h.ListUsers(w, usersReq("GET", target))
			if w.Code != tc.wantStatus {
				t.Fatalf("limit=%q: expected %d, got %d (%s)", tc.limit, tc.wantStatus, w.Code, w.Body.String())
			}
			if tc.wantStatus == http.StatusOK && svc.lastListOpts.PageSize != tc.wantPage {
				t.Errorf("limit=%q: expected PageSize %d passed to store, got %d", tc.limit, tc.wantPage, svc.lastListOpts.PageSize)
			}
		})
	}
}

func TestUsersHandler_List_MalformedPageToken400(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{listErr: services.ErrInvalidPageToken}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users?pageToken=!!!bad"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed pageToken, got %d", w.Code)
	}
}

// AC2: the opaque ?pageToken must be threaded verbatim into ListOpts.PageToken.
func TestUsersHandler_List_PageTokenPassthrough(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users?pageToken=eyJsIjoiYWJjIn0"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if svc.lastListOpts.PageToken != "eyJsIjoiYWJjIn0" {
		t.Errorf("pageToken not propagated to store: got %q", svc.lastListOpts.PageToken)
	}
}

func TestUsersHandler_List_StoreError500(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{listErr: context.DeadlineExceeded}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on generic store error, got %d", w.Code)
	}
}

// ----- AC3 / FR-U14: isInactive (and independence from state) -----

func TestUsersHandler_List_IsInactiveFlag(t *testing.T) {
	t.Parallel()
	active := sampleUser("fresh", fixedNow.AddDate(0, 0, -1), services.UserStateActive)     // 1d → not inactive
	stale := sampleUser("stale", fixedNow.AddDate(0, 0, -40), services.UserStateActive)     // 40d → inactive
	removedFresh := sampleUser("rm", fixedNow.AddDate(0, 0, -1), services.UserStateRemoved) // removed but seen yesterday

	svc := &fakeIdentityService{listPage: []*services.UserRecord{active, stale, removedFresh}}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]UserResponse{}
	for _, u := range body.Users {
		got[u.ID] = u
	}
	if got["fresh"].IsInactive {
		t.Error("recently-seen active user should not be inactive")
	}
	if !got["stale"].IsInactive {
		t.Error("40d-stale user should be inactive (threshold 30)")
	}
	// Independence: a removed user can be isInactive=false; the flags are orthogonal.
	if got["rm"].State != services.UserStateRemoved {
		t.Errorf("expected removed state, got %q", got["rm"].State)
	}
	if got["rm"].IsInactive {
		t.Error("a removed-but-recently-seen user must report isInactive=false (orthogonal to state)")
	}
}

// ----- Story 17.3: per-user application-role join (AC #1, #2, #3) -----

// fedFor builds a single OIDC federated identity for the given stored sub, so
// the test pins the SAME roster-identity → Casbin-subject mapping the handler
// uses (auth.GenerateOIDCUserID(fed.Sub)).
func fedFor(sub string) services.FederatedIdentity {
	return services.FederatedIdentity{
		OrgID:        "default",
		Issuer:       "https://idp.example.com",
		Sub:          sub,
		ProviderKind: "oidc",
		SourceKind:   services.SourceKindOIDCJIT,
		CreatedAt:    fixedNow,
		UpdatedAt:    fixedNow,
	}
}

// AC#1/#2: one page with a serveradmin-subject user and a plain user; the
// derived applicationRole is "serveradmin" vs "member". The expected subject is
// built the SAME way the handler does (auth.GenerateOIDCUserID), pinning the
// real mapping rather than a hard-coded hash. No role:member subject exists.
func TestUsersHandler_List_ApplicationRole(t *testing.T) {
	t.Parallel()
	const adminSub, memberSub = "okta:admin-sub", "okta:member-sub"
	admin := sampleUser("admin", fixedNow.AddDate(0, 0, -1), services.UserStateActive)
	member := sampleUser("member", fixedNow.AddDate(0, 0, -1), services.UserStateActive)

	svc := &fakeIdentityService{
		listPage: []*services.UserRecord{admin, member},
		feds: map[services.UserID][]services.FederatedIdentity{
			admin.ID:  {fedFor(adminSub)},
			member.ID: {fedFor(memberSub)},
		},
	}
	enf := operatorEnforcer()
	// The login-time grant attaches role:serveradmin to the RAW user-oidc-… form.
	enf.serveradminSubjects = map[string]bool{
		auth.GenerateOIDCUserID(adminSub): true,
	}
	h := newTestUsersHandler(svc, enf)

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]string{}
	for _, u := range body.Users {
		got[u.ID] = u.ApplicationRole
	}
	if got["admin"] != "serveradmin" {
		t.Errorf("serveradmin-subject user should derive applicationRole=serveradmin, got %q", got["admin"])
	}
	if got["member"] != "member" {
		t.Errorf("plain user should derive applicationRole=member, got %q", got["member"])
	}
	// member is a derived display value — never a Casbin subject (NFR-T1).
	if strings.Contains(w.Body.String(), "role:member") {
		t.Errorf("response must not surface a role:member subject: %s", w.Body.String())
	}
}

// AC#3: a transient per-user Casbin lookup error degrades that single user to
// "member" — the list still returns 200 and other users are unaffected.
func TestUsersHandler_List_ApplicationRole_DegradesOnLookupError(t *testing.T) {
	t.Parallel()
	const errSub, adminSub = "okta:err-sub", "okta:admin-sub"
	flaky := sampleUser("flaky", fixedNow.AddDate(0, 0, -1), services.UserStateActive)
	admin := sampleUser("admin", fixedNow.AddDate(0, 0, -1), services.UserStateActive)

	svc := &fakeIdentityService{
		listPage: []*services.UserRecord{flaky, admin},
		feds: map[services.UserID][]services.FederatedIdentity{
			flaky.ID: {fedFor(errSub)},
			admin.ID: {fedFor(adminSub)},
		},
	}
	enf := operatorEnforcer()
	errSubject := auth.GenerateOIDCUserID(errSub)
	enf.roleErrSubjects = map[string]bool{
		errSubject:           true,
		"user:" + errSubject: true, // both-prefix probe must also error
	}
	enf.serveradminSubjects = map[string]bool{auth.GenerateOIDCUserID(adminSub): true}
	h := newTestUsersHandler(svc, enf)

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))
	if w.Code != http.StatusOK {
		t.Fatalf("a per-user lookup error must NOT 500 the list; got %d (%s)", w.Code, w.Body.String())
	}
	var body UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]string{}
	for _, u := range body.Users {
		got[u.ID] = u.ApplicationRole
	}
	if got["flaky"] != "member" {
		t.Errorf("user whose lookup errored should degrade to member, got %q", got["flaky"])
	}
	if got["admin"] != "serveradmin" {
		t.Errorf("a sibling user must be unaffected by another user's lookup error, got %q", got["admin"])
	}
}

// AC#3 edge: a federated identity with an empty Sub (SCIM-pushed, never logged
// in — R5-6) is skipped, so the user derives "member" without a lookup.
func TestUsersHandler_List_ApplicationRole_SkipsEmptySub(t *testing.T) {
	t.Parallel()
	u := sampleUser("scim", fixedNow.AddDate(0, 0, -1), services.UserStateActive)
	svc := &fakeIdentityService{
		listPage: []*services.UserRecord{u},
		feds: map[services.UserID][]services.FederatedIdentity{
			u.ID: {fedFor("")},
		},
	}
	h := newTestUsersHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListUsers(w, usersReq("GET", "/api/v1/users"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].ApplicationRole != "member" {
		t.Errorf("empty-sub user should derive member, got %+v", body.Users)
	}
}

// ----- AC5 / AC6: detail happy + not found -----

func TestUsersHandler_Get_HappyPath(t *testing.T) {
	t.Parallel()
	u := sampleUser("bob", fixedNow.AddDate(0, 0, -2), services.UserStateActive)
	svc := &fakeIdentityService{
		byID: map[services.UserID]*services.UserRecord{u.ID: u},
		feds: map[services.UserID][]services.FederatedIdentity{
			u.ID: {{Issuer: "https://idp", Sub: "s", ProviderKind: "oidc", SourceKind: services.SourceKindOIDCJIT, CreatedAt: fixedNow, UpdatedAt: fixedNow}},
		},
	}
	h := newTestUsersHandler(svc, operatorEnforcer())

	req := usersReq("GET", "/api/v1/users/bob")
	req.SetPathValue("id", "bob")
	w := httptest.NewRecorder()
	h.GetUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body UserResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "bob" || len(body.FederatedIdentities) != 1 {
		t.Errorf("unexpected detail body: %+v", body)
	}
}

func TestUsersHandler_Get_NotFound404(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{byID: map[services.UserID]*services.UserRecord{}}
	h := newTestUsersHandler(svc, operatorEnforcer())

	req := usersReq("GET", "/api/v1/users/ghost")
	req.SetPathValue("id", "ghost")
	w := httptest.NewRecorder()
	h.GetUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ----- AC7 / AC8: delete success body + not found -----

func TestUsersHandler_Delete_Success200WithNote(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{}
	h := newTestUsersHandler(svc, operatorEnforcer())

	req := usersReq("DELETE", "/api/v1/users/carol")
	req.SetPathValue("id", "carol")
	w := httptest.NewRecorder()
	h.DeleteUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body DeleteUserResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "carol" || body.State != services.UserStateRemoved || body.Note == "" {
		t.Errorf("unexpected delete body: %+v", body)
	}
	if len(svc.removed) != 1 || svc.removed[0] != services.UserID("carol") {
		t.Errorf("Remove not called with the right id: %+v", svc.removed)
	}
}

func TestUsersHandler_Delete_NotFound404(t *testing.T) {
	t.Parallel()
	svc := &fakeIdentityService{removeErr: services.ErrUserNotFound}
	h := newTestUsersHandler(svc, operatorEnforcer())

	req := usersReq("DELETE", "/api/v1/users/ghost")
	req.SetPathValue("id", "ghost")
	w := httptest.NewRecorder()
	h.DeleteUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ----- AC4 / AC8: settings/* gate per method (401/403, get-vs-update) -----

func TestUsersHandler_Gate(t *testing.T) {
	t.Parallel()

	// 401: no user context on any method (AC12 — per-method gate assertion).
	t.Run("list-unauthenticated-401", func(t *testing.T) {
		h := newTestUsersHandler(&fakeIdentityService{}, operatorEnforcer())
		w := httptest.NewRecorder()
		h.ListUsers(w, httptest.NewRequest("GET", "/api/v1/users", nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("get-unauthenticated-401", func(t *testing.T) {
		h := newTestUsersHandler(&fakeIdentityService{}, operatorEnforcer())
		req := httptest.NewRequest("GET", "/api/v1/users/x", nil)
		req.SetPathValue("id", "x")
		w := httptest.NewRecorder()
		h.GetUser(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("delete-unauthenticated-401", func(t *testing.T) {
		h := newTestUsersHandler(&fakeIdentityService{}, operatorEnforcer())
		req := httptest.NewRequest("DELETE", "/api/v1/users/x", nil)
		req.SetPathValue("id", "x")
		w := httptest.NewRecorder()
		h.DeleteUser(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	// 403: authenticated but no settings/* grant.
	t.Run("list-denied-403", func(t *testing.T) {
		h := newTestUsersHandler(&fakeIdentityService{}, deniedEnforcer())
		w := httptest.NewRecorder()
		h.ListUsers(w, usersReq("GET", "/api/v1/users"))
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	// reads need settings/* get — a read-only operator can GET.
	t.Run("get-readonly-allowed", func(t *testing.T) {
		u := sampleUser("ann", fixedNow, services.UserStateActive)
		svc := &fakeIdentityService{byID: map[services.UserID]*services.UserRecord{u.ID: u}}
		h := newTestUsersHandler(svc, readOnlyEnforcer())
		req := usersReq("GET", "/api/v1/users/ann")
		req.SetPathValue("id", "ann")
		w := httptest.NewRecorder()
		h.GetUser(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("read-only operator should read, got %d", w.Code)
		}
	})

	// DELETE needs settings/* update — a read-only operator is forbidden.
	t.Run("delete-readonly-403", func(t *testing.T) {
		h := newTestUsersHandler(&fakeIdentityService{}, readOnlyEnforcer())
		req := usersReq("DELETE", "/api/v1/users/ann")
		req.SetPathValue("id", "ann")
		w := httptest.NewRecorder()
		h.DeleteUser(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for read-only DELETE, got %d", w.Code)
		}
	})
}

// Compile-time assertion that the fake satisfies the port (guards drift).
var _ services.IdentityService = (*fakeIdentityService)(nil)
