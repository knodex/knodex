// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "knodex-system"

func newStoreForTest(objs ...interface{}) *Store {
	clientObjs := make([]interface{}, len(objs))
	copy(clientObjs, objs)
	runtimeObjs := []interface{}{}
	_ = runtimeObjs
	cs := fake.NewSimpleClientset()
	for _, o := range objs {
		switch v := o.(type) {
		case *corev1.ConfigMap:
			_, _ = cs.CoreV1().ConfigMaps(v.Namespace).Create(context.Background(), v, metav1.CreateOptions{})
		}
	}
	return NewStore(cs, testNamespace)
}

func TestStore_List_NoConfigMap(t *testing.T) {
	store := newStoreForTest()
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(entries))
	}
}

func TestStore_List_EmptyData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
		Data:       map[string]string{ConfigMapKey: ""},
	}
	store := newStoreForTest(cm)
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(entries))
	}
}

func TestStore_List_MalformedJSON(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNamespace},
		Data:       map[string]string{ConfigMapKey: "{not json"},
	}
	store := newStoreForTest(cm)
	_, err := store.List(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestStore_Put_CreatesConfigMap(t *testing.T) {
	store := newStoreForTest()
	entry := Entry{Kind: KindProject, RGDName: "wrapped-project-v1"}

	if err := store.Put(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm, err := store.k8sClient.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap should exist after Put: %v", err)
	}
	if cm.Labels[LabelConfigType] != LabelConfigTypeVal {
		t.Errorf("expected config-type label %q, got %q", LabelConfigTypeVal, cm.Labels[LabelConfigType])
	}
	if cm.Labels[LabelManagedBy] != LabelManagedByVal {
		t.Errorf("expected managed-by label %q, got %q", LabelManagedByVal, cm.Labels[LabelManagedBy])
	}

	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List after Put: %v", err)
	}
	if len(entries) != 1 || entries[0] != entry {
		t.Errorf("entry mismatch: %+v", entries)
	}
}

func TestStore_Put_UpsertReplacesExisting(t *testing.T) {
	store := newStoreForTest()
	ctx := context.Background()

	if err := store.Put(ctx, Entry{Kind: KindProject, RGDName: "wrapped-project-v1"}); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := store.Put(ctx, Entry{Kind: KindProject, RGDName: "wrapped-project-v2"}); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	entries, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(entries))
	}
	if entries[0].RGDName != "wrapped-project-v2" {
		t.Errorf("expected upserted rgdName, got %q", entries[0].RGDName)
	}
}

func TestStore_Put_RejectsUnsupportedKind(t *testing.T) {
	store := newStoreForTest()
	err := store.Put(context.Background(), Entry{Kind: "Secret", RGDName: "wrapped-secret"})
	if err == nil {
		t.Fatal("expected error for unsupported Kind")
	}
	if !strings.Contains(err.Error(), "Secret") {
		t.Errorf("error should mention the unsupported kind: %v", err)
	}
}

func TestStore_Put_RejectsInvalidRGDName(t *testing.T) {
	cases := []struct {
		name string
		rgd  string
	}{
		{"empty", ""},
		{"uppercase", "BadName"},
		{"underscores", "bad_name"},
		{"starts-with-hyphen", "-bad"},
	}
	store := newStoreForTest()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Put(context.Background(), Entry{Kind: KindProject, RGDName: tc.rgd}); err == nil {
				t.Errorf("expected error for invalid RGD name %q", tc.rgd)
			}
		})
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	store := newStoreForTest()
	_, err := store.Get(context.Background(), KindProject)
	if !IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestStore_Get_Found(t *testing.T) {
	store := newStoreForTest()
	ctx := context.Background()
	if err := store.Put(ctx, Entry{Kind: KindProject, RGDName: "wrapped-project-v1"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	entry, err := store.Get(ctx, KindProject)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.RGDName != "wrapped-project-v1" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestStore_Delete_Found(t *testing.T) {
	store := newStoreForTest()
	ctx := context.Background()
	if err := store.Put(ctx, Entry{Kind: KindProject, RGDName: "wrapped-project-v1"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.Delete(ctx, KindProject); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	entries, _ := store.List(ctx)
	if len(entries) != 0 {
		t.Errorf("expected empty entries after Delete, got %d", len(entries))
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	store := newStoreForTest()
	err := store.Delete(context.Background(), KindProject)
	if !IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestValidateRGDName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"simple", "wrapped-project-v1", false},
		{"dotted", "v1.wrapped.project", false},
		{"uppercase", "WrappedProject", true},
		{"too-long", strings.Repeat("a", MaxRGDNameLength+1), true},
		{"ends-with-hyphen", "wrapped-", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRGDName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateRGDName(%q) wantErr=%v got err=%v", tc.input, tc.wantErr, err)
			}
		})
	}
}
