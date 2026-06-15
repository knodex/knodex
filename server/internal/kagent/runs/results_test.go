// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestResultStore(t *testing.T) (*RedisResultStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisResultStore(client), mr
}

func TestResultStore_PutGetRoundtrip(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	completedAt := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	in := &Result{
		RunID:          "run-1",
		AgentNamespace: "",
		Status:         StatusCompleted,
		Response:       "Here is your spec:\n```yaml\nkind: ResourceGraphDefinition\n```",
		CompletedAt:    completedAt,
	}
	require.NoError(t, store.Put(ctx, in))

	out, err := store.Get(ctx, "run-1")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "run-1", out.RunID)
	assert.Equal(t, StatusCompleted, out.Status)
	assert.Equal(t, in.Response, out.Response)
	assert.Empty(t, out.Error)
	assert.True(t, completedAt.Equal(out.CompletedAt))
}

func TestResultStore_FailedResultRoundtrip(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	in := &Result{
		RunID:       "run-fail",
		Status:      StatusFailed,
		Error:       "a2a endpoint returned status 502",
		CompletedAt: time.Now().UTC(),
	}
	require.NoError(t, store.Put(ctx, in))

	out, err := store.Get(ctx, "run-fail")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, StatusFailed, out.Status)
	assert.Equal(t, "a2a endpoint returned status 502", out.Error)
	assert.Empty(t, out.Response)
}

func TestResultStore_GetMissingKey_NilNil(t *testing.T) {
	store, _ := newTestResultStore(t)

	out, err := store.Get(context.Background(), "nope")
	require.NoError(t, err, "missing key is NOT an error — it is the 404 in-flight signal")
	assert.Nil(t, out)
}

func TestResultStore_TTLSet(t *testing.T) {
	store, mr := newTestResultStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, &Result{RunID: "run-ttl", Status: StatusCompleted, CompletedAt: time.Now().UTC()}))

	ttl := mr.TTL("agentruns:result:run-ttl")
	assert.Equal(t, resultTTL, ttl)

	// Past the TTL the result is gone — back to the (nil, nil) signal.
	mr.FastForward(resultTTL + time.Minute)
	out, err := store.Get(ctx, "run-ttl")
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestResultStore_SanitizesOnWrite(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, &Result{
		RunID:       "run-dirty",
		Status:      StatusCompleted,
		Response:    "spec\x1b[31mred\x1b[0m\x00text",
		Error:       "err\x07bell",
		CompletedAt: time.Now().UTC(),
	}))

	out, err := store.Get(ctx, "run-dirty")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotContains(t, out.Response, "\x1b")
	assert.NotContains(t, out.Response, "\x00")
	assert.NotContains(t, out.Error, "\x07")
}

// TestResultStore_PreservesNewlines guards the multi-line contract: the web
// extracts a fenced ```yaml block from Response — flattening newlines on
// write would break spec rendering entirely.
func TestResultStore_PreservesNewlines(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	response := "Here you go:\n```yaml\nkind: ResourceGraphDefinition\nmetadata:\n\tname: demo\n```\nDone."
	require.NoError(t, store.Put(ctx, &Result{
		RunID:       "run-multiline",
		Status:      StatusCompleted,
		Response:    response,
		CompletedAt: time.Now().UTC(),
	}))

	out, err := store.Get(ctx, "run-multiline")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, response, out.Response)
}

func TestResultStore_PutValidation(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	assert.Error(t, store.Put(ctx, nil))
	assert.Error(t, store.Put(ctx, &Result{RunID: ""}))
}

func TestResultStore_NilClient(t *testing.T) {
	store := NewRedisResultStore(nil)
	ctx := context.Background()

	assert.Error(t, store.Put(ctx, &Result{RunID: "x"}))
	_, err := store.Get(ctx, "x")
	assert.Error(t, err)
}

// --- Story 50.3: PolicyValidation on the result ---

func TestResultStore_PolicyValidationRoundtrip(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	in := &Result{
		RunID:       "run-policy",
		Status:      StatusCompleted,
		Response:    "spec text",
		CompletedAt: time.Now().UTC(),
		PolicyValidation: &PolicyValidation{
			Status: PolicyStatusFailed,
			Violations: []PolicyViolation{
				{
					Constraint:        "require-team-label",
					ConstraintKind:    "K8sRequiredLabels",
					EnforcementAction: "deny",
					Message:           "you must provide labels: {\"team\"}",
					ResourceID:        "deployment",
				},
				{Constraint: "warn-latest-tag", EnforcementAction: "warn", Message: "image uses :latest"},
			},
			RevisedResponse:   "Revised:\n```yaml\nkind: ResourceGraphDefinition\n```",
			RevisedStatus:     PolicyStatusPassed,
			RevisedViolations: nil,
		},
	}
	require.NoError(t, store.Put(ctx, in))

	out, err := store.Get(ctx, "run-policy")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.PolicyValidation)
	assert.Equal(t, PolicyStatusFailed, out.PolicyValidation.Status)
	require.Len(t, out.PolicyValidation.Violations, 2)
	assert.Equal(t, "require-team-label", out.PolicyValidation.Violations[0].Constraint)
	assert.Equal(t, "K8sRequiredLabels", out.PolicyValidation.Violations[0].ConstraintKind)
	assert.Equal(t, "deny", out.PolicyValidation.Violations[0].EnforcementAction)
	assert.Equal(t, "deployment", out.PolicyValidation.Violations[0].ResourceID)
	assert.Equal(t, "warn", out.PolicyValidation.Violations[1].EnforcementAction)
	assert.Equal(t, PolicyStatusPassed, out.PolicyValidation.RevisedStatus)
	assert.Contains(t, out.PolicyValidation.RevisedResponse, "ResourceGraphDefinition")
}

// TestResultStore_NilPolicyValidationOmitted pins the OSS contract: a result
// without policy validation serializes WITHOUT the policyValidation key —
// the web keys its Enterprise notice on that absence.
func TestResultStore_NilPolicyValidationOmitted(t *testing.T) {
	store, mr := newTestResultStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, &Result{
		RunID:       "run-oss",
		Status:      StatusCompleted,
		Response:    "spec",
		CompletedAt: time.Now().UTC(),
	}))

	raw, err := mr.Get("agentruns:result:run-oss")
	require.NoError(t, err)
	assert.NotContains(t, raw, "policyValidation",
		"nil PolicyValidation must be omitted from the stored JSON")

	out, err := store.Get(ctx, "run-oss")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Nil(t, out.PolicyValidation)
}

// TestResultStore_SanitizesPolicyViolations proves the untrusted-input
// defense: constraint names/messages are cluster-sourced and must not
// smuggle terminal escapes into stored records.
func TestResultStore_SanitizesPolicyViolations(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, &Result{
		RunID:       "run-hostile",
		Status:      StatusCompleted,
		Response:    "spec",
		CompletedAt: time.Now().UTC(),
		PolicyValidation: &PolicyValidation{
			Status: PolicyStatusFailed,
			Reason: "rea\x1bson",
			Violations: []PolicyViolation{{
				Constraint:        "bad\x1b[31mname",
				ConstraintKind:    "Kind\x00",
				EnforcementAction: "deny\x07",
				Message:           "msg\x1b]0;evil\x07with\nnewline",
				ResourceID:        "res\x00id",
			}},
			RevisedViolations: []PolicyViolation{{Constraint: "c\x1b", Message: "m\x00"}},
		},
	}))

	out, err := store.Get(ctx, "run-hostile")
	require.NoError(t, err)
	require.NotNil(t, out)
	pv := out.PolicyValidation
	require.NotNil(t, pv)
	assert.Equal(t, "reason", pv.Reason)
	v := pv.Violations[0]
	assert.Equal(t, "bad[31mname", v.Constraint)
	assert.Equal(t, "Kind", v.ConstraintKind)
	assert.Equal(t, "deny", v.EnforcementAction)
	assert.NotContains(t, v.Message, "\x1b")
	assert.NotContains(t, v.Message, "\x07")
	assert.NotContains(t, v.Message, "\n", "violation messages are strictly sanitized — newlines removed")
	assert.Equal(t, "resid", v.ResourceID)
	assert.Equal(t, "c", pv.RevisedViolations[0].Constraint)
	assert.Equal(t, "m", pv.RevisedViolations[0].Message)
}

// TestResultStore_RevisedResponsePreservesNewlines mirrors the Response
// contract for the revised text: it is multi-line agent YAML the web
// extracts a fenced block from.
func TestResultStore_RevisedResponsePreservesNewlines(t *testing.T) {
	store, _ := newTestResultStore(t)
	ctx := context.Background()

	revised := "Revised spec:\n```yaml\nkind: ResourceGraphDefinition\nmetadata:\n\tname: demo\n```"
	require.NoError(t, store.Put(ctx, &Result{
		RunID:       "run-revised",
		Status:      StatusCompleted,
		Response:    "original",
		CompletedAt: time.Now().UTC(),
		PolicyValidation: &PolicyValidation{
			Status:          PolicyStatusFailed,
			Violations:      []PolicyViolation{{Constraint: "c", Message: "m"}},
			RevisedResponse: revised + "\x00",
		},
	}))

	out, err := store.Get(ctx, "run-revised")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.PolicyValidation)
	assert.Equal(t, revised, out.PolicyValidation.RevisedResponse,
		"newlines/tabs preserved, control chars stripped")
}
