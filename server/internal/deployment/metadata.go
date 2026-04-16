// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// Label and annotation keys for instance metadata.
// Duplicated from models/ to avoid an import cycle (models imports deployment).
// Any changes here MUST be mirrored in models/instance.go constants.
const (
	labelAppName        = "app.kubernetes.io/name"
	labelDeploymentMode = "knodex.io/deployment-mode"
	labelProject        = "knodex.io/project"

	annotationInstanceID     = "knodex.io/instance-id"
	annotationCreatedBy      = "knodex.io/created-by"
	annotationCreatedAt      = "knodex.io/created-at"
	annotationDeploymentMode = "knodex.io/deployment-mode"
	annotationProjectID      = "knodex.io/project-id"

	// Git source annotations — set at deploy time for GitOps/Hybrid modes.
	// Read back by the instance tracker to populate GitInfo for the UI.
	annotationGitRepository = "knodex.io/git-repository"
	annotationGitBranch     = "knodex.io/git-branch"
	annotationGitPath       = "knodex.io/git-path"

	// KRO-owned annotation for suspending reconciliation.
	// When set to "suspended", KRO skips reconciling child resources.
	annotationKroReconcile = "kro.run/reconcile"
	kroReconcileSuspended  = "suspended"
)

// InstanceMetadataBuilder constructs consistent labels, annotations,
// metadata maps, and manifest/metadata paths for deployment instances.
// All three deployment code paths (directDeploy, applyToCluster,
// GenerateManifest) should use this builder to avoid duplication.
type InstanceMetadataBuilder struct {
	req *DeployRequest
}

// NewInstanceMetadataBuilder creates a builder from a DeployRequest.
func NewInstanceMetadataBuilder(req *DeployRequest) *InstanceMetadataBuilder {
	return &InstanceMetadataBuilder{req: req}
}

// Labels returns the standard label set for this instance.
// All callers get identical labels: app.kubernetes.io/name,
// knodex.io/deployment-mode, and optionally knodex.io/project.
func (b *InstanceMetadataBuilder) Labels() map[string]interface{} {
	labels := map[string]interface{}{
		labelAppName:        b.req.Name,
		labelDeploymentMode: string(b.req.DeploymentMode),
	}
	if b.req.ProjectID != "" {
		labels[labelProject] = b.req.ProjectID
	}
	return labels
}

// Annotations returns the standard annotation set for this instance.
func (b *InstanceMetadataBuilder) Annotations() map[string]interface{} {
	annotations := map[string]interface{}{
		annotationInstanceID:     b.req.InstanceID,
		annotationCreatedBy:      b.req.CreatedBy,
		annotationCreatedAt:      b.req.CreatedAt.Format(time.RFC3339),
		annotationDeploymentMode: string(b.req.DeploymentMode),
	}
	if b.req.ProjectID != "" {
		annotations[annotationProjectID] = b.req.ProjectID
	}
	// For GitOps/Hybrid deployments, store the git source metadata so the
	// instance tracker can populate GitInfo from the cluster object without
	// requiring a separate API call.
	if b.req.Repository != nil {
		annotations[annotationGitRepository] = b.req.Repository.Owner + "/" + b.req.Repository.Repo
		annotations[annotationGitBranch] = b.req.GetEffectiveBranch()
		if path, err := b.ManifestPath(b.req.Repository.BasePath); err == nil {
			annotations[annotationGitPath] = path
		}
	}
	return annotations
}

// ObjectMeta returns the metadata map for an unstructured Kubernetes object.
// Namespace is included only for namespace-scoped instances.
func (b *InstanceMetadataBuilder) ObjectMeta() map[string]interface{} {
	meta := map[string]interface{}{
		"name":        b.req.Name,
		"labels":      b.Labels(),
		"annotations": b.Annotations(),
	}
	if !b.req.IsClusterScoped {
		meta["namespace"] = b.req.Namespace
	}
	return meta
}

// BuildUnstructured constructs a complete Kubernetes manifest as an
// *unstructured.Unstructured, combining apiVersion, kind, the builder's
// ObjectMeta(), and the provided spec. This consolidates the identical
// manifest construction previously duplicated across 4 call sites.
func (b *InstanceMetadataBuilder) BuildUnstructured(apiVersion, kind string, spec interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata":   b.ObjectMeta(),
			"spec":       spec,
		},
	}
}

// BuildUnstructuredSuspended constructs a Kubernetes manifest identical to
// BuildUnstructured but with the kro.run/reconcile: suspended annotation added.
// This creates an instance that KRO will not reconcile until the annotation is
// removed (e.g., by a GitOps tool overwriting with the clean manifest).
func (b *InstanceMetadataBuilder) BuildUnstructuredSuspended(apiVersion, kind string, spec interface{}) *unstructured.Unstructured {
	obj := b.BuildUnstructured(apiVersion, kind, spec)
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationKroReconcile] = kroReconcileSuspended
	obj.SetAnnotations(annotations)
	return obj
}

// ManifestPath returns the file path for a manifest in a Git repository.
// basePath is parameterized (e.g. "instances", "manifests").
// Cluster-scoped: {basePath}/cluster-scoped/{kind}/{name}.yaml
// Namespace-scoped: {basePath}/{namespace}/{kind}/{name}.yaml
func (b *InstanceMetadataBuilder) ManifestPath(basePath string) (string, error) {
	return ManifestPathFor(b.req.Name, b.req.Namespace, b.req.Kind, b.req.IsClusterScoped, basePath)
}

// ManifestPathFor returns the file path for a manifest in a Git repository
// without requiring a DeployRequest or InstanceMetadataBuilder.
// Cluster-scoped: {basePath}/cluster-scoped/{kind}/{name}.yaml
// Namespace-scoped: {basePath}/{namespace}/{kind}/{name}.yaml
// If basePath is empty, defaults to "instances".
// SECURITY: Validates inputs to prevent path traversal attacks (CWE-22).
func ManifestPathFor(name, namespace, kind string, isClusterScoped bool, basePath string) (string, error) {
	if basePath == "" {
		basePath = "instances"
	}
	return buildPathFromPrimitives(name, namespace, kind, isClusterScoped, basePath, "")
}

// buildPathFromPrimitives constructs a scope-aware file path from primitive
// values. If metadataDir is non-empty it is inserted before the filename
// (e.g. ".metadata").
func buildPathFromPrimitives(name, namespace, kind string, isClusterScoped bool, basePath, metadataDir string) (string, error) {
	// Validate basePath does not traverse above root
	if strings.HasPrefix(filepath.Clean(basePath), "..") {
		return "", fmt.Errorf("invalid basePath: must not traverse above root")
	}

	sanitizedName, err := sanitize.PathComponent(name)
	if err != nil {
		return "", fmt.Errorf("invalid name: %w", err)
	}

	var path string
	if isClusterScoped {
		sanitizedKind, kindErr := sanitize.PathComponent(strings.ToLower(kind))
		if kindErr != nil {
			return "", fmt.Errorf("invalid kind: %w", kindErr)
		}
		if metadataDir != "" {
			path = filepath.Join(basePath, "cluster-scoped", sanitizedKind, metadataDir, sanitizedName+".yaml")
		} else {
			path = filepath.Join(basePath, "cluster-scoped", sanitizedKind, sanitizedName+".yaml")
		}
	} else {
		sanitizedNS, nsErr := sanitize.PathComponent(namespace)
		if nsErr != nil {
			return "", fmt.Errorf("invalid namespace: %w", nsErr)
		}
		sanitizedKind, kindErr := sanitize.PathComponent(kind)
		if kindErr != nil {
			return "", fmt.Errorf("invalid kind: %w", kindErr)
		}
		if metadataDir != "" {
			path = filepath.Join(basePath, sanitizedNS, sanitizedKind, metadataDir, sanitizedName+".yaml")
		} else {
			path = filepath.Join(basePath, sanitizedNS, sanitizedKind, sanitizedName+".yaml")
		}
	}

	// Defense in depth: ensure path stays within basePath
	cleanBasePath := filepath.Clean(basePath)
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, cleanBasePath) {
		return "", fmt.Errorf("path traversal detected: path escapes base directory")
	}

	return filepath.ToSlash(path), nil
}
