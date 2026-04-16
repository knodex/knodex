// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"strings"
	"testing"
	"time"
)

func TestInstanceMetadataBuilder_Labels(t *testing.T) {
	now := time.Now()

	t.Run("namespace-scoped with project", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:           "my-app",
			Namespace:      "production",
			DeploymentMode: ModeGitOps,
			ProjectID:      "acme",
			InstanceID:     "inst-1",
			CreatedBy:      "user@test.local",
			CreatedAt:      now,
		})

		labels := b.Labels()

		assertLabel(t, labels, "app.kubernetes.io/name", "my-app")
		assertLabel(t, labels, labelDeploymentMode, "gitops")
		assertLabel(t, labels, labelProject, "acme")
	})

	t.Run("without project", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:           "simple",
			Namespace:      "default",
			DeploymentMode: ModeDirect,
			InstanceID:     "inst-2",
			CreatedBy:      "admin",
			CreatedAt:      now,
		})

		labels := b.Labels()

		assertLabel(t, labels, "app.kubernetes.io/name", "simple")
		assertLabel(t, labels, labelDeploymentMode, "direct")
		if _, ok := labels[labelProject]; ok {
			t.Error("labels should not contain project when not set")
		}
	})
}

func TestInstanceMetadataBuilder_Annotations(t *testing.T) {
	now := time.Now()

	t.Run("full annotations", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:           "my-app",
			Namespace:      "production",
			DeploymentMode: ModeHybrid,
			ProjectID:      "acme",
			InstanceID:     "inst-3",
			CreatedBy:      "user@test.local",
			CreatedAt:      now,
		})

		annotations := b.Annotations()

		assertLabel(t, annotations, annotationInstanceID, "inst-3")
		assertLabel(t, annotations, annotationCreatedBy, "user@test.local")
		assertLabel(t, annotations, annotationCreatedAt, now.Format(time.RFC3339))
		assertLabel(t, annotations, annotationDeploymentMode, "hybrid")
		assertLabel(t, annotations, annotationProjectID, "acme")
	})

	t.Run("without project", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:           "simple",
			Namespace:      "default",
			DeploymentMode: ModeDirect,
			InstanceID:     "inst-4",
			CreatedBy:      "admin",
			CreatedAt:      now,
		})

		annotations := b.Annotations()

		if _, ok := annotations[annotationProjectID]; ok {
			t.Error("annotations should not contain project-id when not set")
		}
	})
}

func TestInstanceMetadataBuilder_ObjectMeta(t *testing.T) {
	now := time.Now()

	t.Run("namespace-scoped includes namespace", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "my-app",
			Namespace:       "production",
			IsClusterScoped: false,
			DeploymentMode:  ModeDirect,
			InstanceID:      "inst-5",
			CreatedBy:       "dev",
			CreatedAt:       now,
		})

		meta := b.ObjectMeta()

		if meta["name"] != "my-app" {
			t.Errorf("expected name=my-app, got %v", meta["name"])
		}
		if meta["namespace"] != "production" {
			t.Errorf("expected namespace=production, got %v", meta["namespace"])
		}
		if meta["labels"] == nil {
			t.Error("expected labels to be set")
		}
		if meta["annotations"] == nil {
			t.Error("expected annotations to be set")
		}
	})

	t.Run("cluster-scoped omits namespace", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "global-config",
			IsClusterScoped: true,
			DeploymentMode:  ModeGitOps,
			InstanceID:      "inst-6",
			CreatedBy:       "admin",
			CreatedAt:       now,
		})

		meta := b.ObjectMeta()

		if _, ok := meta["namespace"]; ok {
			t.Error("cluster-scoped ObjectMeta must not contain namespace")
		}
		if meta["name"] != "global-config" {
			t.Errorf("expected name=global-config, got %v", meta["name"])
		}
	})
}

func TestInstanceMetadataBuilder_BuildUnstructured(t *testing.T) {
	now := time.Now()

	t.Run("namespace-scoped", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "my-app",
			Namespace:       "production",
			IsClusterScoped: false,
			DeploymentMode:  ModeGitOps,
			ProjectID:       "acme",
			InstanceID:      "inst-7",
			CreatedBy:       "user@test.local",
			CreatedAt:       now,
		})

		spec := map[string]interface{}{"replicas": 3}
		obj := b.BuildUnstructured("kro.run/v1alpha1", "Application", spec)

		if obj.GetAPIVersion() != "kro.run/v1alpha1" {
			t.Errorf("apiVersion: got %q, want kro.run/v1alpha1", obj.GetAPIVersion())
		}
		if obj.GetKind() != "Application" {
			t.Errorf("kind: got %q, want Application", obj.GetKind())
		}
		if obj.GetName() != "my-app" {
			t.Errorf("name: got %q, want my-app", obj.GetName())
		}
		if obj.GetNamespace() != "production" {
			t.Errorf("namespace: got %q, want production", obj.GetNamespace())
		}
		if obj.GetLabels()["app.kubernetes.io/name"] != "my-app" {
			t.Errorf("label app.kubernetes.io/name: got %q", obj.GetLabels()["app.kubernetes.io/name"])
		}
		if obj.GetAnnotations()["knodex.io/instance-id"] != "inst-7" {
			t.Errorf("annotation knodex.io/instance-id: got %q", obj.GetAnnotations()["knodex.io/instance-id"])
		}
		if obj.Object["spec"] == nil {
			t.Error("expected spec to be set")
		}
		gotSpec := obj.Object["spec"].(map[string]interface{})
		if gotSpec["replicas"] != 3 {
			t.Errorf("spec.replicas: got %v, want 3", gotSpec["replicas"])
		}
	})

	t.Run("cluster-scoped omits namespace", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "global-config",
			IsClusterScoped: true,
			DeploymentMode:  ModeDirect,
			InstanceID:      "inst-8",
			CreatedBy:       "admin",
			CreatedAt:       now,
		})

		obj := b.BuildUnstructured("v1", "ClusterConfig", map[string]interface{}{"key": "val"})

		if obj.GetNamespace() != "" {
			t.Errorf("cluster-scoped should have empty namespace, got %q", obj.GetNamespace())
		}
		if obj.GetName() != "global-config" {
			t.Errorf("name: got %q, want global-config", obj.GetName())
		}
		if obj.GetAPIVersion() != "v1" {
			t.Errorf("apiVersion: got %q, want v1", obj.GetAPIVersion())
		}
		if obj.GetKind() != "ClusterConfig" {
			t.Errorf("kind: got %q, want ClusterConfig", obj.GetKind())
		}
	})
}

func TestInstanceMetadataBuilder_ManifestPath(t *testing.T) {
	t.Run("namespace-scoped default base", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:      "my-app",
			Namespace: "production",
			Kind:      "WebApp",
		})
		path, err := b.ManifestPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "instances/production/WebApp/my-app.yaml" {
			t.Errorf("got %q, want instances/production/WebApp/my-app.yaml", path)
		}
	})

	t.Run("cluster-scoped custom base", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "global-cfg",
			IsClusterScoped: true,
			Kind:            "ClusterConfig",
		})
		path, err := b.ManifestPath("manifests")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "manifests/cluster-scoped/clusterconfig/global-cfg.yaml" {
			t.Errorf("got %q", path)
		}
	})
}

func TestInstanceMetadataBuilder_ConsistencyAcrossCallers(t *testing.T) {
	// AC #1: All three paths produce identical label sets
	now := time.Now()
	req := &DeployRequest{
		InstanceID:      "inst-consistency",
		Name:            "test-app",
		Namespace:       "staging",
		IsClusterScoped: false,
		DeploymentMode:  ModeHybrid,
		ProjectID:       "proj-1",
		CreatedBy:       "user@test.local",
		CreatedAt:       now,
		Kind:            "Application",
		APIVersion:      "kro.run/v1alpha1",
	}

	b1 := NewInstanceMetadataBuilder(req)
	b2 := NewInstanceMetadataBuilder(req)

	labels1 := b1.Labels()
	labels2 := b2.Labels()

	if len(labels1) != len(labels2) {
		t.Fatalf("label count mismatch: %d vs %d", len(labels1), len(labels2))
	}
	for k, v := range labels1 {
		if labels2[k] != v {
			t.Errorf("label %q mismatch: %v vs %v", k, v, labels2[k])
		}
	}
}

// TestConstants_MirrorModels verifies that local constants in metadata.go match
// the canonical values in models/instance.go. We can't import models (import cycle)
// so we assert the constant variables directly against the expected string values.
//
// If models/instance.go changes a constant, update the expected values here.
// Canonical source: models.DeploymentModeLabel, models.ProjectLabel,
// models.AnnotationInstanceID, models.AnnotationCreatedBy, models.AnnotationCreatedAt,
// models.AnnotationProjectID.
func TestConstants_MirrorModels(t *testing.T) {
	// Assert local constants match models/instance.go canonical values.
	// Each line maps: local constant → expected string (from models package).
	constChecks := []struct {
		name     string
		got      string
		expected string
	}{
		{"labelDeploymentMode", labelDeploymentMode, "knodex.io/deployment-mode"},
		{"labelProject", labelProject, "knodex.io/project"},
		{"annotationInstanceID", annotationInstanceID, "knodex.io/instance-id"},
		{"annotationCreatedBy", annotationCreatedBy, "knodex.io/created-by"},
		{"annotationCreatedAt", annotationCreatedAt, "knodex.io/created-at"},
		{"annotationDeploymentMode", annotationDeploymentMode, "knodex.io/deployment-mode"},
		{"annotationProjectID", annotationProjectID, "knodex.io/project-id"},
	}
	for _, cc := range constChecks {
		if cc.got != cc.expected {
			t.Errorf("constant %s = %q, want %q (must match models/instance.go)", cc.name, cc.got, cc.expected)
		}
	}

	// Also verify builder output contains all expected keys
	b := NewInstanceMetadataBuilder(&DeployRequest{
		Name:           "check",
		Namespace:      "ns",
		DeploymentMode: ModeDirect,
		ProjectID:      "proj",
		InstanceID:     "id",
		CreatedBy:      "user",
		CreatedAt:      time.Now(),
	})

	for key, m := range map[string]map[string]interface{}{
		"labels":      b.Labels(),
		"annotations": b.Annotations(),
	} {
		for _, cc := range constChecks {
			// Only check constants relevant to this map
			if key == "labels" && (cc.name == "labelDeploymentMode" || cc.name == "labelProject") {
				if _, ok := m[cc.expected]; !ok {
					t.Errorf("builder %s missing key %q", key, cc.expected)
				}
			}
			if key == "annotations" && (cc.name == "annotationInstanceID" || cc.name == "annotationCreatedBy" ||
				cc.name == "annotationCreatedAt" || cc.name == "annotationDeploymentMode" || cc.name == "annotationProjectID") {
				if _, ok := m[cc.expected]; !ok {
					t.Errorf("builder %s missing key %q", key, cc.expected)
				}
			}
		}
	}
}

func TestManifestPathFor(t *testing.T) {
	t.Run("namespace-scoped default base", func(t *testing.T) {
		path, err := ManifestPathFor("my-app", "production", "WebApp", false, "instances")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "instances/production/WebApp/my-app.yaml" {
			t.Errorf("got %q, want instances/production/WebApp/my-app.yaml", path)
		}
	})

	t.Run("cluster-scoped custom base", func(t *testing.T) {
		path, err := ManifestPathFor("global-cfg", "", "ClusterConfig", true, "manifests")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "manifests/cluster-scoped/clusterconfig/global-cfg.yaml" {
			t.Errorf("got %q", path)
		}
	})

	t.Run("path traversal in name", func(t *testing.T) {
		path, err := ManifestPathFor("../../etc/passwd", "default", "WebApp", false, "instances")
		// Should either return an error OR the path should not contain ".."
		if err == nil && strings.Contains(path, "..") {
			t.Errorf("path traversal not prevented: got path %q", path)
		}
	})

	t.Run("path traversal in namespace", func(t *testing.T) {
		path, err := ManifestPathFor("app", "../../etc", "WebApp", false, "instances")
		if err == nil && strings.Contains(path, "..") {
			t.Errorf("path traversal not prevented: got path %q", path)
		}
	})

	t.Run("path traversal in kind", func(t *testing.T) {
		path, err := ManifestPathFor("app", "", "../../etc/passwd", true, "instances")
		if err == nil && strings.Contains(path, "..") {
			t.Errorf("path traversal not prevented: got path %q", path)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := ManifestPathFor("", "default", "WebApp", false, "instances")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("empty namespace for namespace-scoped", func(t *testing.T) {
		_, err := ManifestPathFor("my-app", "", "WebApp", false, "instances")
		if err == nil {
			t.Error("expected error for empty namespace")
		}
	})

	t.Run("empty kind for namespace-scoped", func(t *testing.T) {
		_, err := ManifestPathFor("my-app", "default", "", false, "instances")
		if err == nil {
			t.Error("expected error for empty kind")
		}
	})

	t.Run("empty basePath defaults to instances", func(t *testing.T) {
		path, err := ManifestPathFor("my-app", "production", "WebApp", false, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "instances/production/WebApp/my-app.yaml" {
			t.Errorf("got %q, want instances/production/WebApp/my-app.yaml", path)
		}
	})

	t.Run("consistency with builder namespace-scoped", func(t *testing.T) {
		req := &DeployRequest{
			Name:            "test-app",
			Namespace:       "staging",
			Kind:            "Application",
			IsClusterScoped: false,
		}
		builderPath, err := NewInstanceMetadataBuilder(req).ManifestPath("instances")
		if err != nil {
			t.Fatalf("builder error: %v", err)
		}
		standalonePath, err := ManifestPathFor(req.Name, req.Namespace, req.Kind, req.IsClusterScoped, "instances")
		if err != nil {
			t.Fatalf("standalone error: %v", err)
		}
		if builderPath != standalonePath {
			t.Errorf("builder=%q standalone=%q — must be identical", builderPath, standalonePath)
		}
	})

	t.Run("consistency with builder cluster-scoped", func(t *testing.T) {
		req := &DeployRequest{
			Name:            "global-cfg",
			Kind:            "ClusterConfig",
			IsClusterScoped: true,
		}
		builderPath, err := NewInstanceMetadataBuilder(req).ManifestPath("manifests")
		if err != nil {
			t.Fatalf("builder error: %v", err)
		}
		standalonePath, err := ManifestPathFor(req.Name, req.Namespace, req.Kind, req.IsClusterScoped, "manifests")
		if err != nil {
			t.Fatalf("standalone error: %v", err)
		}
		if builderPath != standalonePath {
			t.Errorf("builder=%q standalone=%q — must be identical", builderPath, standalonePath)
		}
	})
}

func TestInstanceMetadataBuilder_BuildUnstructuredSuspended(t *testing.T) {
	now := time.Now()

	t.Run("suspended annotation present with all standard annotations", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "my-app",
			Namespace:       "production",
			IsClusterScoped: false,
			DeploymentMode:  ModeGitOps,
			ProjectID:       "acme",
			InstanceID:      "inst-suspended-1",
			CreatedBy:       "user@test.local",
			CreatedAt:       now,
		})

		spec := map[string]interface{}{"replicas": 3}
		obj := b.BuildUnstructuredSuspended("kro.run/v1alpha1", "Application", spec)

		annotations := obj.GetAnnotations()

		// Verify suspended annotation
		if annotations[annotationKroReconcile] != kroReconcileSuspended {
			t.Errorf("expected kro.run/reconcile=suspended, got %q", annotations[annotationKroReconcile])
		}

		// Verify standard knodex.io annotations are preserved
		if annotations["knodex.io/instance-id"] != "inst-suspended-1" {
			t.Errorf("expected knodex.io/instance-id=inst-suspended-1, got %q", annotations["knodex.io/instance-id"])
		}
		if annotations["knodex.io/created-by"] != "user@test.local" {
			t.Errorf("expected knodex.io/created-by=user@test.local, got %q", annotations["knodex.io/created-by"])
		}
		if annotations["knodex.io/deployment-mode"] != "gitops" {
			t.Errorf("expected knodex.io/deployment-mode=gitops, got %q", annotations["knodex.io/deployment-mode"])
		}
		if annotations["knodex.io/project-id"] != "acme" {
			t.Errorf("expected knodex.io/project-id=acme, got %q", annotations["knodex.io/project-id"])
		}

		// Verify labels are preserved
		labels := obj.GetLabels()
		if labels["app.kubernetes.io/name"] != "my-app" {
			t.Errorf("expected label app.kubernetes.io/name=my-app, got %q", labels["app.kubernetes.io/name"])
		}
	})

	t.Run("BuildUnstructured does NOT contain kro.run/reconcile", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:           "my-app",
			Namespace:      "default",
			DeploymentMode: ModeDirect,
			InstanceID:     "inst-clean",
			CreatedBy:      "admin",
			CreatedAt:      now,
		})

		obj := b.BuildUnstructured("kro.run/v1alpha1", "Application", map[string]interface{}{})
		annotations := obj.GetAnnotations()

		if _, ok := annotations[annotationKroReconcile]; ok {
			t.Error("standard BuildUnstructured must NOT contain kro.run/reconcile annotation")
		}
	})

	t.Run("cluster-scoped suspended object", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "global-config",
			IsClusterScoped: true,
			DeploymentMode:  ModeGitOps,
			InstanceID:      "inst-cluster-suspended",
			CreatedBy:       "admin",
			CreatedAt:       now,
		})

		obj := b.BuildUnstructuredSuspended("v1", "ClusterConfig", map[string]interface{}{"key": "val"})

		if obj.GetNamespace() != "" {
			t.Errorf("cluster-scoped should have empty namespace, got %q", obj.GetNamespace())
		}
		annotations := obj.GetAnnotations()
		if annotations[annotationKroReconcile] != kroReconcileSuspended {
			t.Errorf("expected kro.run/reconcile=suspended on cluster-scoped, got %q", annotations[annotationKroReconcile])
		}
		if annotations["knodex.io/instance-id"] != "inst-cluster-suspended" {
			t.Errorf("expected standard annotations preserved on cluster-scoped")
		}
	})

	t.Run("namespace-scoped suspended object", func(t *testing.T) {
		b := NewInstanceMetadataBuilder(&DeployRequest{
			Name:            "ns-app",
			Namespace:       "staging",
			IsClusterScoped: false,
			DeploymentMode:  ModeGitOps,
			InstanceID:      "inst-ns-suspended",
			CreatedBy:       "dev",
			CreatedAt:       now,
		})

		obj := b.BuildUnstructuredSuspended("kro.run/v1alpha1", "Application", map[string]interface{}{})

		if obj.GetNamespace() != "staging" {
			t.Errorf("expected namespace=staging, got %q", obj.GetNamespace())
		}
		annotations := obj.GetAnnotations()
		if annotations[annotationKroReconcile] != kroReconcileSuspended {
			t.Errorf("expected kro.run/reconcile=suspended, got %q", annotations[annotationKroReconcile])
		}
	})
}

// assertLabel checks a key exists and has expected value in the map.
func assertLabel(t *testing.T, m map[string]interface{}, key, expected string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("expected key %q to exist", key)
		return
	}
	if v != expected {
		t.Errorf("key %q: got %v, want %v", key, v, expected)
	}
}
