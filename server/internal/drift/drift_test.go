// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package drift

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestStoreDrift(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	spec := map[string]interface{}{"replicas": float64(3)}
	err := svc.StoreDrift(context.Background(), "default", "WebApp", "my-app", spec)
	if err != nil {
		t.Fatalf("StoreDrift failed: %v", err)
	}

	// Verify key exists in Redis (org defaults to "default")
	key := "drift:default/default/WebApp/my-app"
	data, err := client.Get(context.Background(), key).Bytes()
	if err != nil {
		t.Fatalf("Redis GET failed: %v", err)
	}

	var entry DriftEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if entry.DesiredSpecHash == "" {
		t.Error("expected non-empty hash")
	}
	if entry.DesiredSpec["replicas"] != float64(3) {
		t.Errorf("expected replicas=3, got %v", entry.DesiredSpec["replicas"])
	}
	if entry.PushedAt == "" {
		t.Error("expected non-empty pushedAt")
	}
}

func TestCheckDrift_Drifted(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	desiredSpec := map[string]interface{}{"replicas": float64(5)}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", desiredSpec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Live spec differs
	liveSpec := map[string]interface{}{"replicas": float64(3)}
	isDrifted, gotDesired, driftedAt, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", liveSpec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if !isDrifted {
		t.Error("expected drift to be true")
	}
	if gotDesired["replicas"] != float64(5) {
		t.Errorf("expected desired replicas=5, got %v", gotDesired["replicas"])
	}
	if driftedAt == nil {
		t.Error("expected non-nil driftedAt")
	}
}

func TestCheckDrift_Reconciled(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	spec := map[string]interface{}{"replicas": float64(3)}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Live spec matches desired → reconciled (but key is NOT auto-deleted)
	isDrifted, desiredSpec, driftedAt, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", spec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if isDrifted {
		t.Error("expected drift to be false (reconciled)")
	}
	if desiredSpec != nil {
		t.Error("expected nil desired spec after reconciliation")
	}
	if driftedAt != nil {
		t.Error("expected nil driftedAt when not drifted")
	}

	// Verify key still exists — CheckDrift no longer auto-clears.
	// Callers must use CheckAndClearIfReconciled or ClearDrift explicitly.
	exists, _ := client.Exists(context.Background(), "drift:default/ns/Kind/app").Result()
	if exists != 1 {
		t.Error("expected drift key to still exist after CheckDrift (no auto-clear)")
	}
}

func TestCheckDrift_NoDriftEntry(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	liveSpec := map[string]interface{}{"replicas": float64(3)}
	isDrifted, desiredSpec, driftedAt, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", liveSpec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if isDrifted {
		t.Error("expected no drift when no entry exists")
	}
	if desiredSpec != nil {
		t.Error("expected nil desired spec")
	}
	if driftedAt != nil {
		t.Error("expected nil driftedAt")
	}
}

func TestClearDrift(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	spec := map[string]interface{}{"key": "value"}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	if err := svc.ClearDrift(context.Background(), "ns", "Kind", "app"); err != nil {
		t.Fatalf("ClearDrift: %v", err)
	}

	exists, _ := client.Exists(context.Background(), "drift:default/ns/Kind/app").Result()
	if exists != 0 {
		t.Error("expected drift key to be deleted")
	}
}

func TestCheckAndClearIfReconciled(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	spec := map[string]interface{}{"replicas": float64(3)}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Not reconciled yet (different spec)
	cleared := svc.CheckAndClearIfReconciled(context.Background(), "ns", "Kind", "app", map[string]interface{}{"replicas": float64(1)})
	if cleared {
		t.Error("expected not cleared (spec doesn't match)")
	}

	// Now reconciled (same spec)
	cleared = svc.CheckAndClearIfReconciled(context.Background(), "ns", "Kind", "app", spec)
	if !cleared {
		t.Error("expected cleared (spec matches)")
	}

	// Verify key was cleaned up
	exists, _ := client.Exists(context.Background(), "drift:default/ns/Kind/app").Result()
	if exists != 0 {
		t.Error("expected drift key to be deleted")
	}
}

func TestGracefulDegradation_NilClient(t *testing.T) {
	svc := NewService(nil, nil, "")

	// All operations should be no-ops with nil client
	err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("StoreDrift with nil client should return nil, got: %v", err)
	}

	isDrifted, desiredSpec, driftedAt, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", map[string]interface{}{})
	if err != nil {
		t.Errorf("CheckDrift with nil client should return nil err, got: %v", err)
	}
	if isDrifted {
		t.Error("drift should default to false with nil client")
	}
	if desiredSpec != nil {
		t.Error("desired spec should be nil with nil client")
	}
	if driftedAt != nil {
		t.Error("driftedAt should be nil with nil client")
	}

	err = svc.ClearDrift(context.Background(), "ns", "Kind", "app")
	if err != nil {
		t.Errorf("ClearDrift with nil client should return nil, got: %v", err)
	}

	cleared := svc.CheckAndClearIfReconciled(context.Background(), "ns", "Kind", "app", map[string]interface{}{})
	if cleared {
		t.Error("should return false with nil client")
	}
}

func TestBatchCheckDrift(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	// Store drift entries for 3 instances
	spec1 := map[string]interface{}{"replicas": float64(3)}
	spec2 := map[string]interface{}{"replicas": float64(5)}
	spec3 := map[string]interface{}{"image": "nginx:latest"}

	if err := svc.StoreDrift(context.Background(), "ns1", "WebApp", "app1", spec1); err != nil {
		t.Fatalf("StoreDrift app1: %v", err)
	}
	if err := svc.StoreDrift(context.Background(), "ns2", "WebApp", "app2", spec2); err != nil {
		t.Fatalf("StoreDrift app2: %v", err)
	}
	if err := svc.StoreDrift(context.Background(), "ns3", "DB", "app3", spec3); err != nil {
		t.Fatalf("StoreDrift app3: %v", err)
	}

	inputs := []DriftCheckInput{
		{Namespace: "ns1", Kind: "WebApp", Name: "app1", LiveSpec: map[string]interface{}{"replicas": float64(1)}}, // drifted
		{Namespace: "ns2", Kind: "WebApp", Name: "app2", LiveSpec: spec2},                                          // reconciled
		{Namespace: "ns3", Kind: "DB", Name: "app3", LiveSpec: map[string]interface{}{"image": "nginx:latest"}},    // reconciled
		{Namespace: "ns4", Kind: "WebApp", Name: "app4", LiveSpec: map[string]interface{}{"replicas": float64(1)}}, // no entry
	}

	results := svc.BatchCheckDrift(context.Background(), inputs)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// app1: drifted (live!=desired)
	if !results[0].IsDrifted {
		t.Error("app1: expected drifted")
	}
	if results[0].DesiredSpec["replicas"] != float64(3) {
		t.Errorf("app1: expected desired replicas=3, got %v", results[0].DesiredSpec["replicas"])
	}
	if results[0].DriftedAt == nil {
		t.Error("app1: expected non-nil DriftedAt")
	}

	// app2: reconciled (live==desired)
	if results[1].IsDrifted {
		t.Error("app2: expected not drifted (reconciled)")
	}
	if results[1].DriftedAt != nil {
		t.Error("app2: expected nil DriftedAt when reconciled")
	}

	// app3: reconciled
	if results[2].IsDrifted {
		t.Error("app3: expected not drifted (reconciled)")
	}
	if results[2].DriftedAt != nil {
		t.Error("app3: expected nil DriftedAt when reconciled")
	}

	// app4: no drift entry
	if results[3].IsDrifted {
		t.Error("app4: expected not drifted (no entry)")
	}
	if results[3].DriftedAt != nil {
		t.Error("app4: expected nil DriftedAt when no entry")
	}
}

func TestBatchCheckDrift_NilClient(t *testing.T) {
	svc := NewService(nil, nil, "")
	results := svc.BatchCheckDrift(context.Background(), []DriftCheckInput{
		{Namespace: "ns", Kind: "Kind", Name: "app", LiveSpec: map[string]interface{}{}},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsDrifted {
		t.Error("expected not drifted with nil client")
	}
}

func TestBatchCheckDrift_Empty(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "")

	results := svc.BatchCheckDrift(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil inputs, got %d", len(results))
	}
}

func TestHashSpec_Deterministic(t *testing.T) {
	spec1 := map[string]interface{}{"replicas": float64(3), "image": "nginx"}
	spec2 := map[string]interface{}{"replicas": float64(3), "image": "nginx"}

	h1, err := HashSpec(spec1)
	if err != nil {
		t.Fatalf("HashSpec: %v", err)
	}
	h2, err := HashSpec(spec2)
	if err != nil {
		t.Fatalf("HashSpec: %v", err)
	}

	if h1 != h2 {
		t.Errorf("expected same hash for same spec, got %s vs %s", h1, h2)
	}

	// Different spec should produce different hash
	spec3 := map[string]interface{}{"replicas": float64(5), "image": "nginx"}
	h3, err := HashSpec(spec3)
	if err != nil {
		t.Fatalf("HashSpec: %v", err)
	}
	if h1 == h3 {
		t.Error("expected different hash for different spec")
	}
}

func TestNewService_EmptyOrg(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()

	svc := NewService(client, nil, "")
	if svc.organization != "default" {
		t.Errorf("expected organization 'default' for empty string, got %q", svc.organization)
	}

	svc2 := NewService(client, nil, "my-org")
	if svc2.organization != "my-org" {
		t.Errorf("expected organization 'my-org', got %q", svc2.organization)
	}

	// Verify keys use the org prefix
	spec := map[string]interface{}{"key": "value"}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}
	if err := svc2.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Both keys should exist with different prefixes
	exists1, _ := client.Exists(context.Background(), "drift:default/ns/Kind/app").Result()
	if exists1 != 1 {
		t.Error("expected key with default org prefix to exist")
	}
	exists2, _ := client.Exists(context.Background(), "drift:my-org/ns/Kind/app").Result()
	if exists2 != 1 {
		t.Error("expected key with my-org prefix to exist")
	}
}

// TestCheckAndClearIfReconciled_AtomicNoRace verifies that the Lua-based atomic
// check-and-delete does NOT incorrectly delete a newer drift entry.
//
// Scenario: StoreDrift(specA) stores H_A. Then StoreDrift(specB) overwrites with H_B.
// CheckAndClearIfReconciled(specA) compares H_A against the stored H_B — they differ,
// so the key must NOT be deleted and false must be returned.
//
// This mirrors the TOCTOU race fixed in STORY-363: without atomicity, a GET+compare+DEL
// sequence could delete the new entry (H_B) after comparing against the old one (H_A).
func TestCheckAndClearIfReconciled_AtomicNoRace(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil, "test-org")

	ctx := context.Background()
	specA := map[string]interface{}{"version": "1"}
	specB := map[string]interface{}{"version": "2"}

	// Store initial drift with specA
	if err := svc.StoreDrift(ctx, "ns", "Kind", "app", specA); err != nil {
		t.Fatalf("StoreDrift(specA): %v", err)
	}

	// Simulate a new push that overwrites the drift entry with specB
	if err := svc.StoreDrift(ctx, "ns", "Kind", "app", specB); err != nil {
		t.Fatalf("StoreDrift(specB): %v", err)
	}

	// Attempt to reconcile with specA — should NOT match H_B and must return false
	reconciled := svc.CheckAndClearIfReconciled(ctx, "ns", "Kind", "app", specA)
	if reconciled {
		t.Error("expected false: specA hash should not match the stored specB hash")
	}

	// The key must still exist (specB drift entry must not be deleted)
	key := svc.driftKey("ns", "Kind", "app")
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 1 {
		t.Error("drift entry for specB was incorrectly deleted")
	}

	// Now reconcile with specB — should match and delete the key
	reconciled = svc.CheckAndClearIfReconciled(ctx, "ns", "Kind", "app", specB)
	if !reconciled {
		t.Error("expected true: specB hash should match the stored hash")
	}

	exists, _ = client.Exists(ctx, key).Result()
	if exists != 0 {
		t.Error("expected drift entry to be deleted after reconciliation with specB")
	}
}
