// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ==============================================================================
// Wrapper-RGD lifecycle E2E (project-wrapper-rgd tech spec)
//
// Prerequisites:
// - E2E_TESTS=true (shared TestMain guard)
// - kro controller running in the cluster (provided by `make qa`)
// - Knodex server running with wrapper feature wired (default)
//
// These tests exercise the full create→update→delete loop with a real kro
// reconciler. Each test cleans up after itself; ordering is independent.
// ==============================================================================

const (
	wrapperE2ENamespace = "knodex-system"
	wrapperE2ERGDName   = "wrapped-project-e2e"
	wrapperE2EProject   = "e2e-wrapped-project"
	wrapperReconcileMax = 30 * time.Second
)

// wrappedProjectGVR is the GVR kro registers for the fixture Kind
// `WrappedProjectE2E`. kro derives the resource plural by lowercasing the Kind
// and appending "s" unless the RGD's spec.schema.crd.spec.names.plural is set
// (the fixture does not set it).
var (
	wrappedProjectGVR = schema.GroupVersionResource{
		Group: "knodex.io", Version: "v1alpha1", Resource: "wrappedprojecte2es",
	}
)

// wrapperRGDGVR is the kro RGD GVR used by these tests. Mirrors the package-level
// `rgdGVR` declared in rgd_visibility_test.go but lives here for readability.
var wrapperRGDGVR = schema.GroupVersionResource{
	Group: "kro.run", Version: "v1alpha1", Resource: "resourcegraphdefinitions",
}

// wrappedProjectE2ERGD is the fixture wrapper RGD: Project + one ConfigMap
// (smaller than the deploy/examples bundle for faster reconcile).
var wrappedProjectE2ERGD = map[string]interface{}{
	"apiVersion": "kro.run/v1alpha1",
	"kind":       "ResourceGraphDefinition",
	"metadata": map[string]interface{}{
		"name":      wrapperE2ERGDName,
		"namespace": wrapperE2ENamespace,
	},
	"spec": map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "WrappedProjectE2E",
			"spec": map[string]interface{}{
				"description":  "string | default=\"\"",
				"destinations": "[]map[string]string | default=[]",
				"sourceRepos":  "[]string | default=[]",
				"roles":        "[]map[string]any | default=[]",
			},
		},
		"resources": []interface{}{
			map[string]interface{}{
				"id": "project",
				"template": map[string]interface{}{
					"apiVersion": "knodex.io/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name":      "${schema.metadata.name}",
						"namespace": wrapperE2ENamespace,
						"annotations": map[string]interface{}{
							"knodex.io/wrapper-rgd-instance": "${schema.metadata.name}",
						},
					},
					"spec": map[string]interface{}{
						"description":  "${schema.spec.description}",
						"destinations": "${schema.spec.destinations}",
						"sourceRepos":  "${schema.spec.sourceRepos}",
						"roles":        "${schema.spec.roles}",
					},
				},
			},
			map[string]interface{}{
				"id": "bootstrapConfig",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "wrapped-defaults-${schema.metadata.name}",
						"namespace": wrapperE2ENamespace,
					},
					"data": map[string]interface{}{
						"project": "${schema.metadata.name}",
					},
				},
			},
		},
	},
}

func wrapperE2EKubeClient(t *testing.T) kubernetes.Interface {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	cs, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	return cs
}

// setupWrapperFixture installs the fixture RGD and registers it in the wrapper
// ConfigMap. Returns a cleanup function. SKIPs the test when kro or Knodex
// server is unavailable.
func setupWrapperFixture(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	// Install RGD fixture.
	obj := &unstructured.Unstructured{Object: wrappedProjectE2ERGD}
	_ = dynamicClient.Resource(wrapperRGDGVR).Namespace(wrapperE2ENamespace).Delete(ctx, wrapperE2ERGDName, metav1.DeleteOptions{})
	time.Sleep(500 * time.Millisecond)
	_, err := dynamicClient.Resource(wrapperRGDGVR).Namespace(wrapperE2ENamespace).Create(ctx, obj, metav1.CreateOptions{})
	require.NoError(t, err, "create wrapper RGD fixture")

	// Wait for kro to mark the RGD Active so the watcher picks it up.
	deadline := time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		got, err := dynamicClient.Resource(wrapperRGDGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2ERGDName, metav1.GetOptions{})
		if err == nil {
			status, _, _ := unstructured.NestedString(got.Object, "status", "state")
			if status == "Active" {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Register Kind=Project → wrappedProjectE2ERGD via the settings API.
	registerWrapperEntry(t, "Project", wrapperE2ERGDName)

	return func() {
		ctx := context.Background()
		// Clean registry first to restore default project routing for other tests.
		deleteWrapperEntry(t, "Project")
		// Delete any leftover wrapped instances.
		_ = dynamicClient.Resource(wrappedProjectGVR).Namespace(wrapperE2ENamespace).Delete(ctx, wrapperE2EProject, metav1.DeleteOptions{})
		_ = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Delete(ctx, wrapperE2EProject, metav1.DeleteOptions{})
		_ = dynamicClient.Resource(wrapperRGDGVR).Namespace(wrapperE2ENamespace).Delete(ctx, wrapperE2ERGDName, metav1.DeleteOptions{})
	}
}

func registerWrapperEntry(t *testing.T, kind, rgdName string) {
	t.Helper()
	adminToken := generateTestJWT(testUserAdmin, nil, true)
	body := map[string]string{"rgdName": rgdName}
	resp, err := makeAuthenticatedRequest(http.MethodPut, "/api/v1/settings/wrappers/"+kind, adminToken, body)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("SKIP: wrapper settings API returned %d; feature may not be deployed", resp.StatusCode)
	}
}

func deleteWrapperEntry(t *testing.T, kind string) {
	t.Helper()
	adminToken := generateTestJWT(testUserAdmin, nil, true)
	resp, err := makeAuthenticatedRequest(http.MethodDelete, "/api/v1/settings/wrappers/"+kind, adminToken, nil)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestWrapper_Create_WrapperInstanceMaterializesProject(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}

	cleanup := setupWrapperFixture(t)
	defer cleanup()

	adminToken := generateTestJWT(testUserAdmin, nil, true)
	resp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        wrapperE2EProject,
		"description": "wrapped e2e project",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "POST /projects should succeed via wrapper")

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, wrapperE2ERGDName, body["wrapperRGD"], "response must echo wrapperRGD")

	// Wait for kro to materialize the Project + bootstrap ConfigMap.
	ctx := context.Background()
	cs := wrapperE2EKubeClient(t)
	deadline := time.Now().Add(wrapperReconcileMax)
	var project *unstructured.Unstructured
	for time.Now().Before(deadline) {
		project, err = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "kro should materialize Project from wrapper instance")

	// Marker annotation present.
	annotations := project.GetAnnotations()
	assert.Equal(t, wrapperE2EProject, annotations["knodex.io/wrapper-rgd-instance"], "marker annotation must be stamped by the wrapper RGD")

	// Bootstrap ConfigMap exists in the same namespace.
	cmName := "wrapped-defaults-" + wrapperE2EProject
	cm, err := cs.CoreV1().ConfigMaps(wrapperE2ENamespace).Get(ctx, cmName, metav1.GetOptions{})
	require.NoError(t, err, "bootstrap ConfigMap should be materialized by kro")
	assert.Equal(t, wrapperE2EProject, cm.Data["project"])

	// Wrapper instance also exists.
	_, err = dynamicClient.Resource(wrappedProjectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
	require.NoError(t, err, "wrapper instance should exist")

	// Trigger explicit DELETE; kro will GC the bundle.
	delResp, err := makeAuthenticatedRequest(http.MethodDelete, "/api/v1/projects/"+wrapperE2EProject, adminToken, nil)
	require.NoError(t, err)
	delResp.Body.Close()
	assert.Equal(t, http.StatusOK, delResp.StatusCode)

	// Verify the bundle is collected.
	deadline = time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		_, err := cs.CoreV1().ConfigMaps(wrapperE2ENamespace).Get(ctx, cmName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	_, err = cs.CoreV1().ConfigMaps(wrapperE2ENamespace).Get(ctx, cmName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "bootstrap ConfigMap should be GC'd; got %v", err)
}

func TestWrapper_Create_MissingRGD_Returns422(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}
	// Register a non-existent RGD.
	registerWrapperEntry(t, "Project", "this-rgd-does-not-exist")
	defer deleteWrapperEntry(t, "Project")

	adminToken := generateTestJWT(testUserAdmin, nil, true)
	resp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        wrapperE2EProject + "-bogus",
		"description": "should fail",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var body struct {
		Code    string            `json:"code"`
		Details map[string]string `json:"details"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "WRAPPER_MISCONFIGURED", body.Code)
	assert.Equal(t, "this-rgd-does-not-exist", body.Details["registeredRGD"])
}

// TestWrapper_Absent_DirectProjectCreation verifies that when no wrapper is
// registered for Kind=Project, POST /api/v1/projects creates the Project CRD
// directly and the response contains no wrapperRGD field (AC1 regression guard).
func TestWrapper_Absent_DirectProjectCreation(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}
	// Ensure no wrapper is registered for Project.
	deleteWrapperEntry(t, "Project") // idempotent: 404 is ignored

	adminToken := generateTestJWT(testUserAdmin, nil, true)
	projectName := wrapperE2EProject + "-direct"

	resp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        projectName,
		"description": "direct creation",
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "wrapper-absent POST should return 201")

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Empty(t, body["wrapperRGD"], "wrapper-absent response must not contain wrapperRGD")
	assert.Empty(t, body["status"], "wrapper-absent response must not contain status=creating")

	// Project CRD should exist directly.
	ctx := context.Background()
	_, err = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, projectName, metav1.GetOptions{})
	require.NoError(t, err, "Project CRD should be created directly for wrapper-absent path")

	// Cleanup.
	delResp, err := makeAuthenticatedRequest(http.MethodDelete, "/api/v1/projects/"+projectName, adminToken, nil)
	require.NoError(t, err)
	delResp.Body.Close()
}

// TestWrapper_Update_PatchesRGDInstance verifies that PATCH /api/v1/projects/{name}
// for a wrapper-managed project updates the wrapper-RGD instance spec, not the
// Project CRD directly (AC4).
func TestWrapper_Update_PatchesRGDInstance(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}

	cleanup := setupWrapperFixture(t)
	defer cleanup()

	adminToken := generateTestJWT(testUserAdmin, nil, true)

	// Create project via wrapper.
	createResp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        wrapperE2EProject,
		"description": "before-update",
	})
	require.NoError(t, err)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	// Wait for kro to materialize the Project (needed for the marker annotation).
	ctx := context.Background()
	deadline := time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		_, err = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "Project must be materialized before PATCH can be tested")

	// Fetch the project to get its resourceVersion for optimistic locking.
	getResp, err := makeAuthenticatedRequest(http.MethodGet, "/api/v1/projects/"+wrapperE2EProject, adminToken, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	var getBody map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getBody))
	resourceVersion, _ := getBody["resourceVersion"].(string)

	// PATCH via the wrapper path.
	patchResp, err := makeAuthenticatedRequest(http.MethodPut, "/api/v1/projects/"+wrapperE2EProject, adminToken, map[string]any{
		"description":     "after-update",
		"resourceVersion": resourceVersion,
	})
	require.NoError(t, err)
	defer patchResp.Body.Close()
	require.Equal(t, http.StatusOK, patchResp.StatusCode, "PATCH via wrapper should return 200")

	var patchBody map[string]any
	require.NoError(t, json.NewDecoder(patchResp.Body).Decode(&patchBody))
	assert.Equal(t, wrapperE2ERGDName, patchBody["wrapperRGD"], "PATCH response must echo wrapperRGD")

	// Verify the wrapper instance spec reflects the update.
	inst, err := dynamicClient.Resource(wrappedProjectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
	require.NoError(t, err, "wrapper instance must still exist after PATCH")
	desc, _, _ := unstructured.NestedString(inst.Object, "spec", "description")
	assert.Equal(t, "after-update", desc, "wrapper instance spec.description must reflect PATCH payload")

	// Wait for kro to reconcile the Project.
	deadline = time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		p, err := dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
		if err == nil {
			d, _, _ := unstructured.NestedString(p.Object, "spec", "description")
			if d == "after-update" {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	project, err := dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
	require.NoError(t, err)
	desc, _, _ = unstructured.NestedString(project.Object, "spec", "description")
	assert.Equal(t, "after-update", desc, "kro must reconcile Project spec.description after wrapper PATCH")
}

// TestWrapper_Delete_RemovesRGDInstance verifies that DELETE /api/v1/projects/{name}
// for a wrapper-managed project deletes the wrapper-RGD instance and kro garbage-collects
// the bundle (AC5).
func TestWrapper_Delete_RemovesRGDInstance(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}

	cleanup := setupWrapperFixture(t)
	defer cleanup()

	adminToken := generateTestJWT(testUserAdmin, nil, true)

	// Create project via wrapper.
	createResp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        wrapperE2EProject,
		"description": "to-be-deleted",
	})
	require.NoError(t, err)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	ctx := context.Background()
	cs := wrapperE2EKubeClient(t)
	cmName := "wrapped-defaults-" + wrapperE2EProject

	// Wait for kro to materialize (so marker annotation is present before DELETE).
	deadline := time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		_, err = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "Project must be materialized before DELETE")

	// DELETE via wrapper path.
	delResp, err := makeAuthenticatedRequest(http.MethodDelete, "/api/v1/projects/"+wrapperE2EProject, adminToken, nil)
	require.NoError(t, err)
	defer delResp.Body.Close()
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "wrapper DELETE should return 200")

	var delBody map[string]any
	require.NoError(t, json.NewDecoder(delResp.Body).Decode(&delBody))
	assert.Equal(t, "deleting", delBody["status"], "DELETE response should indicate async deletion")

	// Wrapper instance must be gone immediately.
	_, err = dynamicClient.Resource(wrappedProjectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "wrapper instance should be deleted synchronously")

	// Wait for kro GC to collect the bootstrap ConfigMap.
	deadline = time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		_, err = cs.CoreV1().ConfigMaps(wrapperE2ENamespace).Get(ctx, cmName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	_, err = cs.CoreV1().ConfigMaps(wrapperE2ENamespace).Get(ctx, cmName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "kro GC should remove bootstrap ConfigMap after wrapper instance deletion")
}

// TestWrapper_SelfHeal_RegistryRemovedAfterCreate verifies that when the registry
// entry for Project is deleted after a wrapped project already exists, PATCH on that
// project falls back to direct Project update without error (AC7).
func TestWrapper_SelfHeal_RegistryRemovedAfterCreate(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E_TESTS!=true")
	}

	cleanup := setupWrapperFixture(t)
	defer cleanup()

	adminToken := generateTestJWT(testUserAdmin, nil, true)

	// Create wrapped project.
	createResp, err := makeAuthenticatedRequest(http.MethodPost, "/api/v1/projects", adminToken, map[string]any{
		"name":        wrapperE2EProject,
		"description": "self-heal test",
	})
	require.NoError(t, err)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	ctx := context.Background()

	// Wait for Project to be materialized.
	deadline := time.Now().Add(wrapperReconcileMax)
	for time.Now().Before(deadline) {
		_, err = dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "Project must be materialized before self-heal test")

	// Remove the registry entry.
	deleteWrapperEntry(t, "Project")

	// Fetch resource version for PATCH.
	getResp, err := makeAuthenticatedRequest(http.MethodGet, "/api/v1/projects/"+wrapperE2EProject, adminToken, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	var getBody map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getBody))
	resourceVersion, _ := getBody["resourceVersion"].(string)

	// PATCH should fall back to direct Project update (self-heal).
	patchResp, err := makeAuthenticatedRequest(http.MethodPut, "/api/v1/projects/"+wrapperE2EProject, adminToken, map[string]any{
		"description":     "after-self-heal",
		"resourceVersion": resourceVersion,
	})
	require.NoError(t, err)
	defer patchResp.Body.Close()
	assert.Equal(t, http.StatusOK, patchResp.StatusCode, "self-heal PATCH must succeed with direct Project update")

	// Verify the direct update took effect on the Project CRD.
	project, err := dynamicClient.Resource(projectGVR).Namespace(wrapperE2ENamespace).Get(ctx, wrapperE2EProject, metav1.GetOptions{})
	require.NoError(t, err)
	desc, _, _ := unstructured.NestedString(project.Object, "spec", "description")
	assert.Equal(t, "after-self-heal", desc, "direct Project update should be reflected in the CRD")
}

// silence unused-import warnings when only some tests in this file are enabled
var _ = fmt.Sprintf
var _ = corev1.ConfigMap{}
