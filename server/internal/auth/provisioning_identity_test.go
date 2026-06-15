// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestEvaluateOIDCUser_InvokesObserverAfterGroupResolution verifies the identity
// observer is called with the issuer + sub + verified flag plumbed from the
// callback (AC17), and that the returned OIDCUserInfo carries Issuer/EmailVerified.
func TestEvaluateOIDCUser_InvokesObserverAfterGroupResolution(t *testing.T) {
	svc, _ := createTestOIDCProvisioningService()

	var gotIssuer, gotSub, gotEmail string
	var gotVerified bool
	called := false
	svc.SetIdentityObserver(func(_ context.Context, issuer, sub, email, _ string, emailVerified bool) error {
		called = true
		gotIssuer, gotSub, gotEmail, gotVerified = issuer, sub, email, emailVerified
		return nil
	})

	info, err := svc.EvaluateOIDCUser(context.Background(), "sub-xyz", "user@example.com", "User", []string{}, "https://idp.example.com", true)
	if err != nil {
		t.Fatalf("EvaluateOIDCUser failed: %v", err)
	}
	if !called {
		t.Fatal("identity observer was not invoked")
	}
	if gotIssuer != "https://idp.example.com" || gotSub != "sub-xyz" || gotEmail != "user@example.com" || !gotVerified {
		t.Errorf("observer args mismatch: issuer=%q sub=%q email=%q verified=%v", gotIssuer, gotSub, gotEmail, gotVerified)
	}
	if info.Issuer != "https://idp.example.com" || !info.EmailVerified {
		t.Errorf("OIDCUserInfo missing issuer/email_verified: %+v", info)
	}
}

// TestEvaluateOIDCUser_ObserverFailureDoesNotFailLogin proves the best-effort
// contract (AC23 / NFR-U1): a failing observer is swallowed and evaluation still
// returns a valid user.
func TestEvaluateOIDCUser_ObserverFailureDoesNotFailLogin(t *testing.T) {
	svc, _ := createTestOIDCProvisioningService()
	// "connection refused" classifies as the "unavailable" reason (a DB outage).
	svc.SetIdentityObserver(func(_ context.Context, _, _, _, _ string, _ bool) error {
		return errors.New("dial tcp: connection refused")
	})

	// AC23: the failing observer must increment knodex_identity_observe_failures_total.
	before := testutil.ToFloat64(identityObserveFailures.WithLabelValues("unavailable"))

	info, err := svc.EvaluateOIDCUser(context.Background(), "sub-1", "user@example.com", "User", []string{}, "https://idp.example.com", true)
	if err != nil {
		t.Fatalf("login must succeed even when observer fails, got: %v", err)
	}
	if info == nil || info.UserID == "" {
		t.Fatal("expected a populated OIDCUserInfo despite observer failure")
	}

	after := testutil.ToFloat64(identityObserveFailures.WithLabelValues("unavailable"))
	if delta := after - before; delta != 1 {
		t.Errorf("expected observe-failure counter to advance by 1 (reason=unavailable), got delta %v", delta)
	}
}

// TestObserveFailureReason pins the error→label classification (AC17: the
// {reason} label must carry discriminating information, not a single constant).
func TestObserveFailureReason(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"deadline":        {context.DeadlineExceeded, "timeout"},
		"canceled":        {context.Canceled, "canceled"},
		"conn refused":    {errors.New("dial tcp 1.2.3.4:5432: connection refused"), "unavailable"},
		"pool exhausted":  {errors.New("pool is closed"), "unavailable"},
		"not implemented": {errors.New("identity: Provision not implemented"), "not_implemented"},
		"generic":         {errors.New("constraint violation"), "observe_error"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := observeFailureReason(tc.err); got != tc.want {
				t.Errorf("observeFailureReason(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestEvaluateOIDCUser_NilObserverIsNoOp verifies the nil-tolerant path.
func TestEvaluateOIDCUser_NilObserverIsNoOp(t *testing.T) {
	svc, _ := createTestOIDCProvisioningService()
	// No SetIdentityObserver call.
	if _, err := svc.EvaluateOIDCUser(context.Background(), "sub-1", "user@example.com", "User", []string{}, "https://idp.example.com", false); err != nil {
		t.Fatalf("nil observer must be a no-op, got: %v", err)
	}
}
