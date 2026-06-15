// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package roletemplates

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNS = "knodex"

func newTestStore() *Store {
	return NewStore(fake.NewSimpleClientset(), testNS)
}

func validTemplate(name string) RoleTemplate {
	return RoleTemplate{
		Name:        name,
		Label:       "Operator",
		Description: "ops access",
		Policies:    []string{"p, proj:{project}:{role}, instances, *, */{project}/*, allow"},
	}
}

func TestList_NoConfigMap_ReturnsDefaults(t *testing.T) {
	s := newTestStore()
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := DefaultRoleTemplates()
	if len(got) != len(want) {
		t.Fatalf("expected %d default templates, got %d", len(want), len(got))
	}
	names := map[string]bool{}
	for _, tmpl := range got {
		names[tmpl.Name] = true
	}
	for _, n := range []string{"admin", "developer", "readonly"} {
		if !names[n] {
			t.Errorf("default template %q missing", n)
		}
	}
}

func TestList_NoConfigMap_DoesNotMaterialize(t *testing.T) {
	client := fake.NewSimpleClientset()
	s := NewStore(client, testNS)
	if _, err := s.List(context.Background()); err != nil {
		t.Fatalf("List: %v", err)
	}
	// A read must not create the ConfigMap (AC #3: defaults are data, not written).
	_, err := client.CoreV1().ConfigMaps(testNS).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected ConfigMap to remain absent after List, got err=%v", err)
	}
}

func TestCreate_MaterializesConfigMapAndAppends(t *testing.T) {
	client := fake.NewSimpleClientset()
	s := NewStore(client, testNS)

	created, err := s.Create(context.Background(), validTemplate("operator"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "operator" {
		t.Fatalf("expected created name operator, got %q", created.Name)
	}

	// ConfigMap now exists and holds defaults + the new template.
	cm, err := client.CoreV1().ConfigMaps(testNS).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not materialized: %v", err)
	}
	var stored []RoleTemplate
	if err := json.Unmarshal([]byte(cm.Data["templates"]), &stored); err != nil {
		t.Fatalf("decode stored templates: %v", err)
	}
	if len(stored) != len(DefaultRoleTemplates())+1 {
		t.Fatalf("expected defaults+1 stored, got %d", len(stored))
	}

	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List after create: %v", err)
	}
	if len(list) != len(DefaultRoleTemplates())+1 {
		t.Fatalf("expected defaults+1 from List, got %d", len(list))
	}
}

func TestCreate_DuplicateName_ReturnsAlreadyExists(t *testing.T) {
	s := newTestStore()
	// "admin" is a default — creating it again must conflict.
	_, err := s.Create(context.Background(), validTemplate("admin"))
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestCreate_InvalidInput_ReturnsValidationError(t *testing.T) {
	s := newTestStore()
	cases := []struct {
		name  string
		tmpl  RoleTemplate
		field string
	}{
		{"empty name", RoleTemplate{Name: "", Policies: []string{"p"}}, "name"},
		{"bad name", RoleTemplate{Name: "Bad Name", Policies: []string{"p"}}, "name"},
		{"no policies", RoleTemplate{Name: "ok", Policies: nil}, "policies"},
		{"empty policy", RoleTemplate{Name: "ok", Policies: []string{""}}, "policies"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Create(context.Background(), tc.tmpl)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected *ValidationError, got %v", err)
			}
			if verr.Field != tc.field {
				t.Errorf("expected field %q, got %q", tc.field, verr.Field)
			}
		})
	}
}

func TestGet_FoundAndNotFound(t *testing.T) {
	s := newTestStore()
	got, err := s.Get(context.Background(), "developer")
	if err != nil {
		t.Fatalf("Get developer: %v", err)
	}
	if got.Name != "developer" {
		t.Errorf("expected developer, got %q", got.Name)
	}
	if _, err := s.Get(context.Background(), "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for ghost, got %v", err)
	}
}

func TestUpdate_ExistingAndNotFound(t *testing.T) {
	s := newTestStore()
	upd := validTemplate("developer")
	upd.Description = "changed"
	got, err := s.Update(context.Background(), "developer", upd)
	if err != nil {
		t.Fatalf("Update developer: %v", err)
	}
	if got.Description != "changed" {
		t.Errorf("expected changed description, got %q", got.Description)
	}
	reread, _ := s.Get(context.Background(), "developer")
	if reread.Description != "changed" {
		t.Errorf("update not persisted, got %q", reread.Description)
	}

	if _, err := s.Update(context.Background(), "ghost", validTemplate("ghost")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound updating ghost, got %v", err)
	}
}

func TestUpdate_PathNameIsAuthoritative(t *testing.T) {
	s := newTestStore()
	body := validTemplate("renamed") // body name differs from path
	got, err := s.Update(context.Background(), "developer", body)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "developer" {
		t.Errorf("expected path name 'developer' to win, got %q", got.Name)
	}
	if _, err := s.Get(context.Background(), "renamed"); !errors.Is(err, ErrNotFound) {
		t.Errorf("body name must not create a new template")
	}
}

func TestDelete_ExistingAndNotFound(t *testing.T) {
	s := newTestStore()
	if err := s.Delete(context.Background(), "readonly"); err != nil {
		t.Fatalf("Delete readonly: %v", err)
	}
	if _, err := s.Get(context.Background(), "readonly"); !errors.Is(err, ErrNotFound) {
		t.Errorf("readonly should be gone after delete")
	}
	// Remaining defaults persist (delete materialized the ConfigMap).
	list, _ := s.List(context.Background())
	if len(list) != len(DefaultRoleTemplates())-1 {
		t.Errorf("expected defaults-1 after delete, got %d", len(list))
	}
	if err := s.Delete(context.Background(), "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting ghost, got %v", err)
	}
}

func TestList_ExistingConfigMapWithTemplates(t *testing.T) {
	tmpls := []RoleTemplate{validTemplate("custom")}
	encoded, _ := json.Marshal(tmpls)
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNS},
		Data:       map[string]string{"templates": string(encoded)},
	})
	s := NewStore(client, testNS)
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Existing (non-empty) ConfigMap is authoritative — defaults are NOT merged in.
	if len(list) != 1 || list[0].Name != "custom" {
		t.Fatalf("expected only the stored custom template, got %+v", list)
	}
}

func TestList_EmptyTemplatesKey_ReturnsDefaults(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: testNS},
		Data:       map[string]string{"templates": ""},
	})
	s := NewStore(client, testNS)
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != len(DefaultRoleTemplates()) {
		t.Fatalf("expected defaults for empty key, got %d", len(list))
	}
}
