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
	svc := NewService(client, nil)

	spec := map[string]interface{}{"replicas": float64(3)}
	err := svc.StoreDrift(context.Background(), "default", "WebApp", "my-app", spec)
	if err != nil {
		t.Fatalf("StoreDrift failed: %v", err)
	}

	// Verify key exists in Redis
	key := "drift:default/WebApp/my-app"
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
	svc := NewService(client, nil)

	desiredSpec := map[string]interface{}{"replicas": float64(5)}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", desiredSpec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Live spec differs
	liveSpec := map[string]interface{}{"replicas": float64(3)}
	isDrifted, gotDesired, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", liveSpec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if !isDrifted {
		t.Error("expected drift to be true")
	}
	if gotDesired["replicas"] != float64(5) {
		t.Errorf("expected desired replicas=5, got %v", gotDesired["replicas"])
	}
}

func TestCheckDrift_Reconciled(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil)

	spec := map[string]interface{}{"replicas": float64(3)}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	// Live spec matches desired → reconciled
	isDrifted, desiredSpec, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", spec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if isDrifted {
		t.Error("expected drift to be false (reconciled)")
	}
	if desiredSpec != nil {
		t.Error("expected nil desired spec after reconciliation")
	}

	// Verify key was cleaned up
	exists, _ := client.Exists(context.Background(), "drift:ns/Kind/app").Result()
	if exists != 0 {
		t.Error("expected drift key to be deleted after reconciliation")
	}
}

func TestCheckDrift_NoDriftEntry(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil)

	liveSpec := map[string]interface{}{"replicas": float64(3)}
	isDrifted, desiredSpec, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", liveSpec)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if isDrifted {
		t.Error("expected no drift when no entry exists")
	}
	if desiredSpec != nil {
		t.Error("expected nil desired spec")
	}
}

func TestClearDrift(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil)

	spec := map[string]interface{}{"key": "value"}
	if err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", spec); err != nil {
		t.Fatalf("StoreDrift: %v", err)
	}

	if err := svc.ClearDrift(context.Background(), "ns", "Kind", "app"); err != nil {
		t.Fatalf("ClearDrift: %v", err)
	}

	exists, _ := client.Exists(context.Background(), "drift:ns/Kind/app").Result()
	if exists != 0 {
		t.Error("expected drift key to be deleted")
	}
}

func TestCheckAndClearIfReconciled(t *testing.T) {
	client, mr := newTestRedis(t)
	defer mr.Close()
	svc := NewService(client, nil)

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
	exists, _ := client.Exists(context.Background(), "drift:ns/Kind/app").Result()
	if exists != 0 {
		t.Error("expected drift key to be deleted")
	}
}

func TestGracefulDegradation_NilClient(t *testing.T) {
	svc := NewService(nil, nil)

	// All operations should be no-ops with nil client
	err := svc.StoreDrift(context.Background(), "ns", "Kind", "app", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("StoreDrift with nil client should return nil, got: %v", err)
	}

	isDrifted, desiredSpec, err := svc.CheckDrift(context.Background(), "ns", "Kind", "app", map[string]interface{}{})
	if err != nil {
		t.Errorf("CheckDrift with nil client should return nil err, got: %v", err)
	}
	if isDrifted {
		t.Error("drift should default to false with nil client")
	}
	if desiredSpec != nil {
		t.Error("desired spec should be nil with nil client")
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
