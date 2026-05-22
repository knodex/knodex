// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// startWatcherInBackground starts w in a goroutine and waits briefly for cache sync.
// Tests must call w.Stop() when done.
func startWatcherInBackground(t *testing.T, w *Watcher) (cancel func()) {
	t.Helper()
	ctx, cancelFn := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = w.Start(ctx)
	}()
	// Give informers a moment to do the initial list and signal cache sync.
	time.Sleep(50 * time.Millisecond)
	return func() {
		cancelFn()
		w.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("watcher did not stop")
		}
	}
}

func makeWrapperCM(entries []Entry) *corev1.ConfigMap {
	data, _ := json.Marshal(entries)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: testNamespace,
		},
		Data: map[string]string{ConfigMapKey: string(data)},
	}
}

func TestWatcher_InitialSync(t *testing.T) {
	cm := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	cs := fake.NewSimpleClientset(cm)
	w := NewWatcher(cs, testNamespace, nil)

	stop := startWatcherInBackground(t, w)
	defer stop()

	if rgd, ok := w.Lookup(KindProject); !ok || rgd != "wrapped-project-v1" {
		t.Errorf("expected Lookup(Project) → (wrapped-project-v1, true), got (%q, %v)", rgd, ok)
	}
}

func TestWatcher_AddTriggersCallback(t *testing.T) {
	cs := fake.NewSimpleClientset()
	w := NewWatcher(cs, testNamespace, nil)
	var fired int32
	w.OnEntriesChanged(func(entries []Entry) {
		atomic.AddInt32(&fired, 1)
	})

	stop := startWatcherInBackground(t, w)
	defer stop()

	cm := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	if _, err := cs.CoreV1().ConfigMaps(testNamespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create ConfigMap: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&fired) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if atomic.LoadInt32(&fired) == 0 {
		t.Fatal("expected callback to fire after ConfigMap add")
	}
	if rgd, _ := w.Lookup(KindProject); rgd != "wrapped-project-v1" {
		t.Errorf("Lookup after add: got %q", rgd)
	}
}

func TestWatcher_DeleteKeepsLastValid(t *testing.T) {
	cm := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	cs := fake.NewSimpleClientset(cm)
	w := NewWatcher(cs, testNamespace, nil)

	stop := startWatcherInBackground(t, w)
	defer stop()

	if _, ok := w.Lookup(KindProject); !ok {
		t.Fatal("Lookup should succeed before delete")
	}

	if err := cs.CoreV1().ConfigMaps(testNamespace).Delete(context.Background(), ConfigMapName, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete ConfigMap: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if _, ok := w.Lookup(KindProject); !ok {
		t.Error("after ConfigMap delete, watcher should retain last valid entries")
	}
}

func TestWatcher_MalformedJSONKeepsLastValid(t *testing.T) {
	good := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	cs := fake.NewSimpleClientset(good)
	w := NewWatcher(cs, testNamespace, nil)

	stop := startWatcherInBackground(t, w)
	defer stop()

	bad := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
		Data:       map[string]string{ConfigMapKey: "{not json"},
	}
	if _, err := cs.CoreV1().ConfigMaps(testNamespace).Update(context.Background(), bad, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update ConfigMap to malformed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if rgd, ok := w.Lookup(KindProject); !ok || rgd != "wrapped-project-v1" {
		t.Errorf("expected last-valid preserved on malformed JSON, got Lookup → (%q, %v)", rgd, ok)
	}
}

func TestWatcher_HashDedupSuppressesNoOpCallbacks(t *testing.T) {
	cm := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	cs := fake.NewSimpleClientset(cm)
	w := NewWatcher(cs, testNamespace, nil)

	var fired int32
	w.OnEntriesChanged(func(entries []Entry) {
		atomic.AddInt32(&fired, 1)
	})

	stop := startWatcherInBackground(t, w)
	defer stop()

	// Touch the ConfigMap with the same data — informer fires Update but hash
	// matches lastEntriesHash so callbacks must not fire.
	got, err := cs.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.ResourceVersion = "999" // force the informer to see this as new
	if _, err := cs.CoreV1().ConfigMaps(testNamespace).Update(context.Background(), got, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(&fired); got > 0 {
		t.Errorf("expected zero callbacks on same-data update, got %d", got)
	}
}

func TestWatcher_StopIsIdempotent(t *testing.T) {
	cs := fake.NewSimpleClientset()
	w := NewWatcher(cs, testNamespace, nil)
	stop := startWatcherInBackground(t, w)
	stop()
	// Second Stop must not panic.
	w.Stop()
	w.Stop()
}

func TestWatcher_EntriesReturnsCopy(t *testing.T) {
	cm := makeWrapperCM([]Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}})
	cs := fake.NewSimpleClientset(cm)
	w := NewWatcher(cs, testNamespace, nil)
	stop := startWatcherInBackground(t, w)
	defer stop()

	entries := w.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entries[0].RGDName = "tampered"
	again := w.Entries()
	if again[0].RGDName != "wrapped-project-v1" {
		t.Error("Entries() should return a defensive copy")
	}
}
