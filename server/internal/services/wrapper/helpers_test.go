// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/models"
)

// fakeRGDResolver returns a configured RGD or false on Get.
type fakeRGDResolver struct {
	rgds map[string]*models.CatalogRGD
}

func (f *fakeRGDResolver) GetRGDByName(name string) (*models.CatalogRGD, bool) {
	r, ok := f.rgds[name]
	return r, ok
}

// fakeGVRResolver returns a deterministic GVR.
type fakeGVRResolver struct{}

func (f *fakeGVRResolver) ResolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	group, version := parseAPIVersion(apiVersion)
	plural := stringsToLower(kind) + "s"
	return schema.GroupVersionResource{Group: group, Version: version, Resource: plural}, nil
}

func stringsToLower(s string) string {
	// avoid extra import in this small helper
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		out[i] = c
	}
	return string(out)
}

func newHelpersForTest(rgds map[string]*models.CatalogRGD, seedObjs ...runtime.Object) *Helpers {
	w := NewWatcher(k8sfake.NewSimpleClientset(), testNamespace, nil)

	// Seed the watcher's in-memory cache (bypass informer for unit tests).
	w.lastValidEntries = []Entry{{Kind: KindProject, RGDName: "wrapped-project-v1"}}

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojects"}: "WrappedProjectList",
	}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, seedObjs...)

	return NewHelpers(w, &fakeRGDResolver{rgds: rgds}, &fakeGVRResolver{}, dc, testNamespace)
}

func TestHelpers_LookupWrapper(t *testing.T) {
	h := newHelpersForTest(nil)
	if rgd, ok := h.LookupWrapper(KindProject); !ok || rgd != "wrapped-project-v1" {
		t.Errorf("expected (wrapped-project-v1, true), got (%q, %v)", rgd, ok)
	}
	if _, ok := h.LookupWrapper("Secret"); ok {
		t.Error("expected false for unregistered Kind")
	}
}

func TestHelpers_ResolveInstanceGVK_Found(t *testing.T) {
	h := newHelpersForTest(map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name:       "wrapped-project-v1",
			APIVersion: "knodex.io/v1alpha1",
			Kind:       "WrappedProject",
		},
	})
	apiVersion, kind, err := h.ResolveInstanceGVK("wrapped-project-v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiVersion != "knodex.io/v1alpha1" || kind != "WrappedProject" {
		t.Errorf("unexpected GVK: %s / %s", apiVersion, kind)
	}
}

func TestHelpers_ResolveInstanceGVK_NotFound(t *testing.T) {
	h := newHelpersForTest(nil)
	_, _, err := h.ResolveInstanceGVK("bogus")
	if !errors.Is(err, ErrWrapperRGDNotFound) {
		t.Errorf("expected ErrWrapperRGDNotFound, got %v", err)
	}
}

func TestHelpers_ResolveInstanceGVK_MissingSchema(t *testing.T) {
	h := newHelpersForTest(map[string]*models.CatalogRGD{
		"wrapped-project-v1": {Name: "wrapped-project-v1"}, // no APIVersion/Kind
	})
	_, _, err := h.ResolveInstanceGVK("wrapped-project-v1")
	if !errors.Is(err, ErrWrapperRGDNotReady) {
		t.Errorf("expected ErrWrapperRGDNotReady, got %v", err)
	}
}

func TestHelpers_CreateViaWrapper_BuildsInstance(t *testing.T) {
	h := newHelpersForTest(map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name:       "wrapped-project-v1",
			APIVersion: "knodex.io/v1alpha1",
			Kind:       "WrappedProject",
		},
	})
	spec := map[string]any{"description": "hello", "destinations": []any{}}
	created, err := h.CreateViaWrapper(context.Background(), "wrapped-project-v1", "my-project", spec)
	if err != nil {
		t.Fatalf("CreateViaWrapper: %v", err)
	}

	if created.GetName() != "my-project" {
		t.Errorf("expected name=my-project, got %q", created.GetName())
	}
	if created.GetNamespace() != testNamespace {
		t.Errorf("expected namespace=%q, got %q", testNamespace, created.GetNamespace())
	}
	if created.GetAPIVersion() != "knodex.io/v1alpha1" {
		t.Errorf("apiVersion mismatch: %q", created.GetAPIVersion())
	}
	if created.GetKind() != "WrappedProject" {
		t.Errorf("kind mismatch: %q", created.GetKind())
	}
	if got := created.GetLabels()[WrapperKindLabel]; got != KindProject {
		t.Errorf("expected wrapper-kind label %q, got %q", KindProject, got)
	}
	// spec.description preserved
	gotDesc, found, err := unstructured.NestedString(created.Object, "spec", "description")
	if err != nil || !found || gotDesc != "hello" {
		t.Errorf("spec.description not preserved: %v / %v / %q", err, found, gotDesc)
	}
}

func TestHelpers_DeleteViaWrapper(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojects"}
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("knodex.io/v1alpha1")
	existing.SetKind("WrappedProject")
	existing.SetName("my-project")
	existing.SetNamespace(testNamespace)

	h := newHelpersForTest(map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name:       "wrapped-project-v1",
			APIVersion: "knodex.io/v1alpha1",
			Kind:       "WrappedProject",
		},
	}, existing)

	if err := h.DeleteViaWrapper(context.Background(), "wrapped-project-v1", "my-project"); err != nil {
		t.Fatalf("DeleteViaWrapper: %v", err)
	}
	// confirm the object is gone
	_, err := h.dynamicClient.Resource(gvr).Namespace(testNamespace).Get(context.Background(), "my-project", metav1.GetOptions{})
	if err == nil {
		t.Error("expected resource to be deleted")
	}
}

func TestHelpers_UpdateViaWrapper(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojects"}
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("knodex.io/v1alpha1")
	existing.SetKind("WrappedProject")
	existing.SetName("my-project")
	existing.SetNamespace(testNamespace)
	if err := unstructured.SetNestedField(existing.Object, map[string]any{"description": "old"}, "spec"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newHelpersForTest(map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name:       "wrapped-project-v1",
			APIVersion: "knodex.io/v1alpha1",
			Kind:       "WrappedProject",
		},
	}, existing)

	spec := map[string]any{"description": "new"}
	if _, err := h.UpdateViaWrapper(context.Background(), "wrapped-project-v1", "my-project", spec); err != nil {
		t.Fatalf("UpdateViaWrapper: %v", err)
	}
	updated, err := h.dynamicClient.Resource(gvr).Namespace(testNamespace).Get(context.Background(), "my-project", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	gotDesc, _, _ := unstructured.NestedString(updated.Object, "spec", "description")
	if gotDesc != "new" {
		t.Errorf("expected description=new, got %q", gotDesc)
	}
}

func TestIsWrapped_OwningRGDInstance(t *testing.T) {
	if IsWrapped(nil) {
		t.Error("IsWrapped(nil) should be false")
	}
	if IsWrapped(map[string]string{}) {
		t.Error("IsWrapped({}) should be false")
	}
	if !IsWrapped(map[string]string{MarkerAnnotation: "my-project"}) {
		t.Error("IsWrapped(marker present) should be true")
	}
	if OwningRGDInstance(map[string]string{MarkerAnnotation: "my-project"}) != "my-project" {
		t.Error("OwningRGDInstance should return marker value")
	}
	if OwningRGDInstance(nil) != "" {
		t.Error("OwningRGDInstance(nil) should be empty")
	}
}

// silence unused-import warnings for stdlib helpers used in only one test
var _ = json.Marshal
var _ = corev1.ConfigMap{}
