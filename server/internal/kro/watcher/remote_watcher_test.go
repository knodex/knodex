// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

// --- RemoteResourceCache tests ---

func TestRemoteResourceCache_AddAndList(t *testing.T) {
	cache := NewRemoteResourceCache()

	obj := &unstructured.Unstructured{}
	obj.SetName("my-cert")
	obj.SetNamespace("app-ns")

	cache.Add("cluster-a", "app-ns", "certificates", "my-cert", obj)

	results := cache.List("cluster-a", "certificates")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].GetName() != "my-cert" {
		t.Errorf("expected name 'my-cert', got %q", results[0].GetName())
	}
}

func TestRemoteResourceCache_Update(t *testing.T) {
	cache := NewRemoteResourceCache()

	obj1 := &unstructured.Unstructured{}
	obj1.SetName("my-cert")
	obj1.SetNamespace("app-ns")
	obj1.SetLabels(map[string]string{"version": "1"})

	cache.Add("cluster-a", "app-ns", "certificates", "my-cert", obj1)

	obj2 := &unstructured.Unstructured{}
	obj2.SetName("my-cert")
	obj2.SetNamespace("app-ns")
	obj2.SetLabels(map[string]string{"version": "2"})

	cache.Update("cluster-a", "app-ns", "certificates", "my-cert", obj2)

	results := cache.List("cluster-a", "certificates")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].GetLabels()["version"] != "2" {
		t.Errorf("expected version '2', got %q", results[0].GetLabels()["version"])
	}
}

func TestRemoteResourceCache_Delete(t *testing.T) {
	cache := NewRemoteResourceCache()

	obj := &unstructured.Unstructured{}
	obj.SetName("my-cert")
	obj.SetNamespace("app-ns")
	cache.Add("cluster-a", "app-ns", "certificates", "my-cert", obj)

	cache.Delete("cluster-a", "app-ns", "certificates", "my-cert")

	results := cache.List("cluster-a", "certificates")
	if len(results) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(results))
	}
}

func TestRemoteResourceCache_DeleteCluster(t *testing.T) {
	cache := NewRemoteResourceCache()

	obj1 := &unstructured.Unstructured{}
	obj1.SetName("cert-1")
	obj1.SetNamespace("ns-1")
	cache.Add("cluster-a", "ns-1", "certificates", "cert-1", obj1)

	obj2 := &unstructured.Unstructured{}
	obj2.SetName("cert-2")
	obj2.SetNamespace("ns-2")
	cache.Add("cluster-a", "ns-2", "certificates", "cert-2", obj2)

	obj3 := &unstructured.Unstructured{}
	obj3.SetName("cert-3")
	obj3.SetNamespace("ns-1")
	cache.Add("cluster-b", "ns-1", "certificates", "cert-3", obj3)

	cache.SetClusterStatus("cluster-a", RemoteWatchStatusConnected)

	cache.DeleteCluster("cluster-a")

	if cache.Count() != 1 {
		t.Fatalf("expected 1 resource remaining, got %d", cache.Count())
	}
	results := cache.List("cluster-b", "certificates")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for cluster-b, got %d", len(results))
	}

	// Cluster status should be gone
	if status := cache.GetClusterStatus("cluster-a"); status != "" {
		t.Errorf("expected empty status for deleted cluster, got %q", status)
	}
}

func TestRemoteResourceCache_ListByKind(t *testing.T) {
	cache := NewRemoteResourceCache()

	cert := &unstructured.Unstructured{}
	cert.SetName("my-cert")
	cert.SetNamespace("ns")
	cache.Add("cluster-a", "ns", "certificates", "my-cert", cert)

	ingress := &unstructured.Unstructured{}
	ingress.SetName("my-ingress")
	ingress.SetNamespace("ns")
	cache.Add("cluster-a", "ns", "ingresses", "my-ingress", ingress)

	certs := cache.List("cluster-a", "certificates")
	if len(certs) != 1 {
		t.Errorf("expected 1 cert, got %d", len(certs))
	}

	ings := cache.List("cluster-a", "ingresses")
	if len(ings) != 1 {
		t.Errorf("expected 1 ingress, got %d", len(ings))
	}
}

func TestRemoteResourceCache_ClusterStatus(t *testing.T) {
	cache := NewRemoteResourceCache()

	cache.SetClusterStatus("cluster-a", RemoteWatchStatusConnected)
	if status := cache.GetClusterStatus("cluster-a"); status != RemoteWatchStatusConnected {
		t.Errorf("expected Connected, got %q", status)
	}

	cache.SetClusterStatus("cluster-a", RemoteWatchStatusUnreachable)
	if status := cache.GetClusterStatus("cluster-a"); status != RemoteWatchStatusUnreachable {
		t.Errorf("expected ClusterUnreachable, got %q", status)
	}
}

func TestRemoteResourceCache_ThreadSafety(t *testing.T) {
	cache := NewRemoteResourceCache()
	var wg sync.WaitGroup

	// Concurrent writes and reads
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			obj := &unstructured.Unstructured{}
			obj.SetName("obj")
			obj.SetNamespace("ns")
			cache.Add("cluster", "ns", "certificates", "obj", obj)
		}(i)
		go func(n int) {
			defer wg.Done()
			cache.List("cluster", "certificates")
		}(i)
	}
	wg.Wait()
}

// --- RemoteWatcher tests ---

func TestRemoteWatcher_Lifecycle(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	rw := NewRemoteWatcher(k8sClient, nil)

	if rw.IsRunning() {
		t.Fatal("expected not running before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rw.Start(ctx); err != nil {
		t.Fatalf("unexpected Start error: %v", err)
	}

	if !rw.IsRunning() {
		t.Fatal("expected running after Start")
	}
	if !rw.IsSynced() {
		t.Fatal("expected synced after Start")
	}

	// Start again should be no-op
	if err := rw.Start(ctx); err != nil {
		t.Fatalf("double Start should be no-op, got error: %v", err)
	}

	rw.Stop()
	if rw.IsRunning() {
		t.Fatal("expected not running after Stop")
	}

	// Double stop should be safe
	rw.Stop()
}

func TestRemoteWatcher_StopAndWait(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	rw := NewRemoteWatcher(k8sClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = rw.Start(ctx)
	ok := rw.StopAndWait(5 * time.Second)
	if !ok {
		t.Fatal("expected StopAndWait to return true")
	}
}

func TestRemoteWatcher_AddWatchTarget_ClusterUnreachable(t *testing.T) {
	// No kubeconfig secret exists → should mark cluster as unreachable
	k8sClient := fake.NewSimpleClientset()
	rw := NewRemoteWatcher(k8sClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = rw.Start(ctx)
	defer rw.Stop()

	err := rw.AddWatchTarget(ctx, "missing-cluster", []string{"my-ns"})
	if err == nil {
		t.Fatal("expected error for missing kubeconfig secret")
	}

	targets := rw.Targets()
	status, ok := targets["missing-cluster"]
	if !ok {
		t.Fatal("expected target to be tracked even when unreachable")
	}
	if status != RemoteWatchStatusUnreachable {
		t.Errorf("expected ClusterUnreachable status, got %q", status)
	}

	// Cache should also reflect unreachable status
	if cacheStatus := rw.Cache().GetClusterStatus("missing-cluster"); cacheStatus != RemoteWatchStatusUnreachable {
		t.Errorf("expected cache cluster status ClusterUnreachable, got %q", cacheStatus)
	}
}

func TestRemoteWatcher_RemoveWatchTarget_Idempotent(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	rw := NewRemoteWatcher(k8sClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = rw.Start(ctx)
	defer rw.Stop()

	// Remove non-existent target — should be no-op
	rw.RemoveWatchTarget("nonexistent")
	if len(rw.Targets()) != 0 {
		t.Fatal("expected 0 targets")
	}
}

func TestRemoteWatcher_AddWatchTarget_IdempotentWhenUnreachable(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	rw := NewRemoteWatcher(k8sClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = rw.Start(ctx)
	defer rw.Stop()

	// First add — unreachable
	_ = rw.AddWatchTarget(ctx, "missing-cluster", []string{"ns"})

	// Second add — should retry (since it's unreachable, not connected)
	_ = rw.AddWatchTarget(ctx, "missing-cluster", []string{"ns"})

	if len(rw.Targets()) != 1 {
		t.Errorf("expected 1 target, got %d", len(rw.Targets()))
	}
}

func TestBackoffDelay(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int
		wantMin      time.Duration
		wantMax      time.Duration
	}{
		{"first failure", 0, 5 * time.Second, 5 * time.Second},
		{"second failure", 1, 5 * time.Second, 5 * time.Second},
		{"third failure", 2, 10 * time.Second, 10 * time.Second},
		{"fourth failure", 3, 20 * time.Second, 20 * time.Second},
		{"high failure count (capped)", 100, 5 * time.Minute, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := backoffDelay(tt.failureCount)
			if delay < tt.wantMin || delay > tt.wantMax {
				t.Errorf("backoffDelay(%d) = %v, want between %v and %v", tt.failureCount, delay, tt.wantMin, tt.wantMax)
			}
		})
	}
}
