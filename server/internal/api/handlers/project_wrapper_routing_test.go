// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services/wrapper"
)

// fakeRGDResolver implements wrapper.RGDResolver for unit tests.
type fakeRGDResolver struct{ rgds map[string]*models.CatalogRGD }

func (f *fakeRGDResolver) GetRGDByName(name string) (*models.CatalogRGD, bool) {
	r, ok := f.rgds[name]
	return r, ok
}

// fakeGVRResolver implements wrapper.GVRResolver via simple pluralization.
type fakeGVRResolver struct{}

func (fakeGVRResolver) ResolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	var group, version string
	if i := indexOfSlash(apiVersion); i >= 0 {
		group, version = apiVersion[:i], apiVersion[i+1:]
	} else {
		version = apiVersion
	}
	return schema.GroupVersionResource{Group: group, Version: version, Resource: toLower(kind) + "s"}, nil
}

func indexOfSlash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

type wrapperRoutingSetup struct {
	handler  *ProjectHandler
	service  *mockProjectService
	recorder *mockAuditRecorder
	dynamic  *dynamicfake.FakeDynamicClient
	watcher  *wrapper.Watcher
}

func newWrapperRoutingHandler(
	t *testing.T,
	rgds map[string]*models.CatalogRGD,
	seedObjs ...runtime.Object,
) *wrapperRoutingSetup {
	t.Helper()

	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}

	w := wrapper.NewWatcher(k8sfake.NewSimpleClientset(), "knodex-system", nil)
	wrapper.SetWatcherEntriesForTest(w, []wrapper.Entry{{Kind: wrapper.KindProject, RGDName: "wrapped-project-v1"}})

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojects"}: "WrappedProjectList",
	}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, seedObjs...)

	helpers := wrapper.NewHelpers(w, &fakeRGDResolver{rgds: rgds}, fakeGVRResolver{}, dc, "knodex-system")
	handler := NewProjectHandler(svc, enforcer, recorder)
	handler.SetWrapperHelpers(helpers)

	return &wrapperRoutingSetup{handler: handler, service: svc, recorder: recorder, dynamic: dc, watcher: w}
}

func wrappedProjectGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojects"}
}

func TestProjectHandler_CreateProject_WrapperRoute_Success(t *testing.T) {
	t.Parallel()
	s := newWrapperRoutingHandler(t, map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name: "wrapped-project-v1", APIVersion: "knodex.io/v1alpha1", Kind: "WrappedProject",
		},
	})

	body, _ := json.Marshal(CreateProjectRequest{Name: "team-alpha", Description: "hello"})
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	rec := httptest.NewRecorder()
	s.handler.CreateProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, exists := s.service.projects["team-alpha"]; exists {
		t.Error("wrapper path should not invoke projectService.CreateProject")
	}

	inst, err := s.dynamic.Resource(wrappedProjectGVR()).Namespace("knodex-system").Get(
		context.Background(), "team-alpha", metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("wrapper instance not found: %v", err)
	}
	desc, _, _ := unstructured.NestedString(inst.Object, "spec", "description")
	if desc != "hello" {
		t.Errorf("expected description=hello, got %q", desc)
	}

	if len(s.recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(s.recorder.events))
	}
	ev := s.recorder.events[0]
	if ev.Details["wrapperUsed"] != true {
		t.Errorf("wrapperUsed missing in audit details: %+v", ev.Details)
	}
	if ev.Details["wrapperRGD"] != "wrapped-project-v1" {
		t.Errorf("wrapperRGD mismatch: %v", ev.Details["wrapperRGD"])
	}
}

func TestProjectHandler_CreateProject_WrapperRoute_MissingRGD_Returns422(t *testing.T) {
	t.Parallel()
	s := newWrapperRoutingHandler(t, nil)

	body, _ := json.Marshal(CreateProjectRequest{Name: "team-alpha"})
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	rec := httptest.NewRecorder()
	s.handler.CreateProject(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code    string            `json:"code"`
		Details map[string]string `json:"details"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "WRAPPER_MISCONFIGURED" {
		t.Errorf("expected code WRAPPER_MISCONFIGURED, got %q", resp.Code)
	}
	if resp.Details["registeredRGD"] != "wrapped-project-v1" {
		t.Errorf("registeredRGD missing in details: %+v", resp.Details)
	}
	if len(s.recorder.events) != 1 || s.recorder.events[0].Result != "error" {
		t.Errorf("expected one error audit event, got %+v", s.recorder.events)
	}
}

func TestProjectHandler_UpdateProject_WrapperRoute_Success(t *testing.T) {
	t.Parallel()
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("knodex.io/v1alpha1")
	existing.SetKind("WrappedProject")
	existing.SetName("team-alpha")
	existing.SetNamespace("knodex-system")
	_ = unstructured.SetNestedField(existing.Object, map[string]any{"description": "old"}, "spec")

	s := newWrapperRoutingHandler(t, map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name: "wrapped-project-v1", APIVersion: "knodex.io/v1alpha1", Kind: "WrappedProject",
		},
	}, existing)

	s.service.addProject("team-alpha", rbac.ProjectSpec{Description: "old"})
	s.service.projects["team-alpha"].ObjectMeta.Annotations = map[string]string{
		wrapper.MarkerAnnotation: "team-alpha",
	}

	body, _ := json.Marshal(UpdateProjectRequest{Description: "new", ResourceVersion: "1"})
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/team-alpha", body, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	req.SetPathValue("name", "team-alpha")
	rec := httptest.NewRecorder()
	s.handler.UpdateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	got, err := s.dynamic.Resource(wrappedProjectGVR()).Namespace("knodex-system").Get(
		context.Background(), "team-alpha", metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("wrapper instance disappeared: %v", err)
	}
	desc, _, _ := unstructured.NestedString(got.Object, "spec", "description")
	if desc != "new" {
		t.Errorf("expected spec.description=new, got %q", desc)
	}
}

func TestProjectHandler_DeleteProject_WrapperRoute_Success(t *testing.T) {
	t.Parallel()
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("knodex.io/v1alpha1")
	existing.SetKind("WrappedProject")
	existing.SetName("team-alpha")
	existing.SetNamespace("knodex-system")

	s := newWrapperRoutingHandler(t, map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name: "wrapped-project-v1", APIVersion: "knodex.io/v1alpha1", Kind: "WrappedProject",
		},
	}, existing)

	s.service.addProject("team-alpha", rbac.ProjectSpec{})
	s.service.projects["team-alpha"].ObjectMeta.Annotations = map[string]string{
		wrapper.MarkerAnnotation: "team-alpha",
	}

	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/team-alpha", nil, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	req.SetPathValue("name", "team-alpha")
	rec := httptest.NewRecorder()
	s.handler.DeleteProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := s.dynamic.Resource(wrappedProjectGVR()).Namespace("knodex-system").Get(
		context.Background(), "team-alpha", metav1.GetOptions{},
	); err == nil {
		t.Error("wrapper instance should have been deleted")
	}
	if _, exists := s.service.projects["team-alpha"]; !exists {
		t.Error("wrapper-path delete should not invoke projectService.DeleteProject")
	}
}

func TestProjectHandler_DeleteProject_WrapperSelfHeal_RegistryRemoved(t *testing.T) {
	t.Parallel()
	s := newWrapperRoutingHandler(t, map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name: "wrapped-project-v1", APIVersion: "knodex.io/v1alpha1", Kind: "WrappedProject",
		},
	})
	// Operator removed the registry entry after the project was already created.
	wrapper.SetWatcherEntriesForTest(s.watcher, nil)

	s.service.addProject("team-alpha", rbac.ProjectSpec{})
	s.service.projects["team-alpha"].ObjectMeta.Annotations = map[string]string{
		wrapper.MarkerAnnotation: "team-alpha",
	}

	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/team-alpha", nil, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	req.SetPathValue("name", "team-alpha")
	rec := httptest.NewRecorder()
	s.handler.DeleteProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 via direct-delete self-heal, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, exists := s.service.projects["team-alpha"]; exists {
		t.Error("expected direct-delete fallback to remove the project")
	}
}

// TestProjectHandler_UpdateProject_WrapperSelfHeal_RegistryRemoved covers AC7
// for the PATCH path: when the project carries the marker annotation but the
// registry entry has been removed since create-time, the handler falls back to
// direct Project update without returning an error to the caller.
func TestProjectHandler_UpdateProject_WrapperSelfHeal_RegistryRemoved(t *testing.T) {
	t.Parallel()
	s := newWrapperRoutingHandler(t, map[string]*models.CatalogRGD{
		"wrapped-project-v1": {
			Name: "wrapped-project-v1", APIVersion: "knodex.io/v1alpha1", Kind: "WrappedProject",
		},
	})
	// Operator removed the registry entry after the project was already created.
	wrapper.SetWatcherEntriesForTest(s.watcher, nil)

	s.service.addProject("team-alpha", rbac.ProjectSpec{Description: "old"})
	s.service.projects["team-alpha"].ObjectMeta.Annotations = map[string]string{
		wrapper.MarkerAnnotation: "team-alpha",
	}

	body, _ := json.Marshal(UpdateProjectRequest{Description: "new", ResourceVersion: "1"})
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/team-alpha", body, &middleware.UserContext{
		UserID: "admin-user", Email: "admin@test.local",
	})
	req.SetPathValue("name", "team-alpha")
	rec := httptest.NewRecorder()
	s.handler.UpdateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 via direct-update self-heal, got %d: %s", rec.Code, rec.Body.String())
	}
	// Direct path should have called projectService.UpdateProject, changing the description.
	if s.service.projects["team-alpha"].Spec.Description != "new" {
		t.Errorf("expected description updated to 'new' via direct path, got %q",
			s.service.projects["team-alpha"].Spec.Description)
	}
}
