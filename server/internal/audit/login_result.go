// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"context"
	"net/http"
)

// loginResultKey is the context key for the login result signal.
type loginResultKey struct{}

// loginIdentityKey is the context key for the login identity signal.
type loginIdentityKey struct{}

// loginIdentity holds the user identity communicated from handler to middleware.
type loginIdentity struct {
	UserID string
	Email  string
}

// SetLoginResult stores the login result for audit middleware consumption.
// The handler calls this instead of setting a response header, preventing
// internal audit metadata from leaking to browser clients.
//
// No-op if no signal was injected into context (e.g., OSS builds without
// the EE audit login middleware).
func SetLoginResult(r *http.Request, result string) {
	if ptr, ok := r.Context().Value(loginResultKey{}).(*string); ok {
		*ptr = result
	}
}

// PrepareLoginResult injects a login result signal into the request context.
// Returns the modified request and a reader function to retrieve the result
// after the handler completes.
//
// Called by the EE audit login middleware before invoking the handler.
func PrepareLoginResult(r *http.Request) (*http.Request, func() string) {
	signal := new(string)
	ctx := context.WithValue(r.Context(), loginResultKey{}, signal)
	return r.WithContext(ctx), func() string { return *signal }
}

// SetLoginIdentity stores the authenticated user's identity for audit middleware
// consumption. Handlers call this when they have verified user identity (e.g.,
// after successful OIDC token exchange or local login) so the middleware can
// record the correct userID and email in the audit event.
//
// IMPORTANT: Must be called from the request-handling goroutine only (not from
// spawned goroutines). The underlying struct is not synchronized.
//
// No-op if no signal was injected into context (e.g., OSS builds without
// the EE audit login middleware).
func SetLoginIdentity(r *http.Request, userID, email string) {
	if ptr, ok := r.Context().Value(loginIdentityKey{}).(*loginIdentity); ok {
		ptr.UserID = userID
		ptr.Email = email
	}
}

// PrepareLoginIdentity injects a login identity signal into the request context.
// Returns the modified request and a reader function to retrieve the identity
// after the handler completes.
//
// Called by the EE audit login middleware before invoking the handler.
func PrepareLoginIdentity(r *http.Request) (*http.Request, func() (string, string)) {
	signal := &loginIdentity{}
	ctx := context.WithValue(r.Context(), loginIdentityKey{}, signal)
	return r.WithContext(ctx), func() (string, string) { return signal.UserID, signal.Email }
}
