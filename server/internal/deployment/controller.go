// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"

	"github.com/knodex/knodex/server/internal/deployment/vcs"
	"github.com/knodex/knodex/server/internal/util/retry"
)

// errResourceAlreadyExists is a sentinel used inside the retry closure to signal
// that Create returned AlreadyExists and the caller should fall through to Patch.
var errResourceAlreadyExists = errors.New("resource already exists")

// vcsClient is a narrow interface covering only the VCS operations used by the
// controller. Defined here (not in the vcs package) per Go convention: interfaces
// belong at the point of use. *vcs.GitHubClient satisfies this interface.
type vcsClient interface {
	CommitFile(ctx context.Context, req *vcs.CommitFileRequest) (*vcs.CommitResult, error)
	DeleteFile(ctx context.Context, path, branch, message string) error
	Close()
}

// Controller orchestrates instance deployments across different modes
type Controller struct {
	// Kubernetes clients
	dynamicClient   dynamic.Interface
	kubeClient      kubernetes.Interface
	discoveryClient discovery.DiscoveryInterface

	// Manifest generator
	generator *Generator

	// retryConfig controls retry behavior for K8s API calls in applyToCluster.
	retryConfig retry.RetryConfig

	// vcsClientFactory creates a VCS client for Git operations.
	// Defaults to vcs.NewGitHubClient. Override in tests to inject a mock.
	vcsClientFactory func(ctx context.Context, token, owner, repo string) (vcsClient, error)

	// Logger
	logger *slog.Logger
}

// NewController creates a new deployment controller
func NewController(dynamicClient dynamic.Interface, kubeClient kubernetes.Interface, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Controller{
		dynamicClient: dynamicClient,
		kubeClient:    kubeClient,
		generator:     NewGenerator(),
		retryConfig: retry.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   500 * time.Millisecond,
			MaxDelay:    5 * time.Second,
		},
		vcsClientFactory: func(ctx context.Context, token, owner, repo string) (vcsClient, error) {
			return vcs.NewGitHubClient(ctx, token, owner, repo)
		},
		logger: logger,
	}
	// Extract discovery client from kubeClient if available
	if kubeClient != nil {
		c.discoveryClient = kubeClient.Discovery()
	}
	return c
}

// Deploy executes a deployment based on the configured mode
func (c *Controller) Deploy(ctx context.Context, req *DeployRequest) (*DeployResult, error) {
	if req == nil {
		return nil, fmt.Errorf("deploy request cannot be nil")
	}

	result := &DeployResult{
		InstanceID: req.InstanceID,
		Name:       req.Name,
		Namespace:  req.Namespace,
		Mode:       req.DeploymentMode,
		Status:     StatusPending,
		DeployedAt: time.Now(),
	}

	c.logger.Info("starting deployment",
		"instanceId", req.InstanceID,
		"name", req.Name,
		"namespace", req.Namespace,
		"mode", req.DeploymentMode,
	)

	switch req.DeploymentMode {
	case ModeDirect:
		return c.deployDirect(ctx, req, result)
	case ModeGitOps:
		return c.deployGitOps(ctx, req, result)
	case ModeHybrid:
		return c.deployHybrid(ctx, req, result)
	default:
		// Default to direct mode for backward compatibility
		c.logger.Warn("unknown deployment mode, defaulting to direct",
			"mode", req.DeploymentMode,
			"instanceId", req.InstanceID,
		)
		req.DeploymentMode = ModeDirect
		result.Mode = ModeDirect
		return c.deployDirect(ctx, req, result)
	}
}

// deployDirect applies the manifest directly to the Kubernetes cluster
func (c *Controller) deployDirect(ctx context.Context, req *DeployRequest, result *DeployResult) (*DeployResult, error) {
	c.logger.Info("executing direct deployment",
		"instanceId", req.InstanceID,
		"namespace", req.Namespace,
	)

	result.Status = StatusCreating

	// Apply the manifest to the cluster
	if err := c.applyToCluster(ctx, req); err != nil {
		result.Status = StatusFailed
		result.ClusterError = err.Error()
		c.logger.Error("direct deployment failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("failed to apply manifest to cluster: %w", err)
	}

	result.ClusterDeployed = true
	result.Status = StatusReady

	c.logger.Info("direct deployment completed",
		"instanceId", req.InstanceID,
		"namespace", req.Namespace,
	)

	return result, nil
}

// deployGitOps applies the manifest to the cluster with suspended reconciliation,
// then pushes the clean manifest (without suspended annotation) to Git.
// If the Git push fails after a successful cluster apply, a compensating delete
// is attempted to avoid leaving a permanently suspended resource on the cluster.
func (c *Controller) deployGitOps(ctx context.Context, req *DeployRequest, result *DeployResult) (*DeployResult, error) {
	c.logger.Info("executing GitOps deployment",
		"instanceId", req.InstanceID,
		"namespace", req.Namespace,
	)

	// Validate repository configuration
	if req.Repository == nil {
		result.Status = StatusFailed
		result.GitError = "repository configuration is required for GitOps deployment"
		return result, fmt.Errorf("repository configuration is required for GitOps deployment")
	}

	// SECURITY: Validate repository config to prevent injection attacks
	if err := req.Repository.Validate(); err != nil {
		result.Status = StatusFailed
		result.GitError = fmt.Sprintf("invalid repository configuration: %v", err)
		c.logger.Error("repository validation failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("invalid repository configuration: %w", err)
	}

	// Apply suspended manifest to cluster for immediate visibility.
	// Resolve GVR once here so the compensating delete path reuses it without
	// a second discovery round-trip.
	result.Status = StatusCreating
	mb := NewInstanceMetadataBuilder(req)
	suspendedObj := mb.BuildUnstructuredSuspended(req.APIVersion, req.Kind, req.Spec)

	suspendedGVR, err := c.getGVRFromUnstructured(suspendedObj)
	if err != nil {
		result.Status = StatusFailed
		result.ClusterError = err.Error()
		c.logger.Error("GitOps deployment GVR resolution failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("failed to apply suspended manifest to cluster: %w", err)
	}

	if err := c.applyObjectToCluster(ctx, suspendedObj, req); err != nil {
		result.Status = StatusFailed
		result.ClusterError = err.Error()
		c.logger.Error("GitOps deployment cluster apply failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("failed to apply suspended manifest to cluster: %w", err)
	}

	result.ClusterDeployed = true
	result.Status = StatusManifestGenerated

	// Push clean manifest (without suspended annotation) to Git
	commitSHA, manifestPath, err := c.pushToGit(ctx, req)
	if err != nil {
		// Compensating delete: remove the suspended resource from the cluster
		// to avoid leaving a permanently unreconciled instance.
		// Use a detached context so the delete completes even if the parent
		// context was canceled (e.g., request timeout).
		delCtx, delCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer delCancel()
		if delErr := c.deleteFromClusterByGVR(delCtx, suspendedGVR, suspendedObj, req); delErr != nil {
			c.logger.Warn("failed to delete suspended resource after Git push failure",
				"instanceId", req.InstanceID,
				"deleteError", delErr,
			)
		} else {
			result.ClusterDeployed = false
		}

		result.Status = StatusFailed
		result.GitError = err.Error()
		c.logger.Error("GitOps deployment Git push failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("failed to push manifest to Git: %w", err)
	}

	result.GitPushed = true
	result.GitCommitSHA = commitSHA
	result.ManifestPath = manifestPath
	result.Status = StatusPushedToGit

	c.logger.Info("GitOps deployment completed",
		"instanceId", req.InstanceID,
		"commitSha", commitSHA,
		"manifestPath", manifestPath,
	)

	return result, nil
}

// deployHybrid applies to cluster AND pushes to Git
// Git push failures are logged but don't fail the deployment
func (c *Controller) deployHybrid(ctx context.Context, req *DeployRequest, result *DeployResult) (*DeployResult, error) {
	c.logger.Info("executing hybrid deployment",
		"instanceId", req.InstanceID,
		"namespace", req.Namespace,
	)

	// Step 1: Apply to cluster (required)
	result.Status = StatusCreating
	if err := c.applyToCluster(ctx, req); err != nil {
		result.Status = StatusFailed
		result.ClusterError = err.Error()
		c.logger.Error("hybrid deployment cluster apply failed",
			"instanceId", req.InstanceID,
			"error", err,
		)
		return result, fmt.Errorf("failed to apply manifest to cluster: %w", err)
	}
	result.ClusterDeployed = true

	// Step 2: Push to Git (optional - failure doesn't fail deployment)
	if req.Repository != nil {
		// SECURITY: Validate repository config before use
		if err := req.Repository.Validate(); err != nil {
			result.GitError = fmt.Sprintf("invalid repository configuration: %v", err)
			result.Status = StatusGitOpsFailed
			c.logger.Warn("hybrid deployment skipping Git push due to invalid repository config",
				"instanceId", req.InstanceID,
				"error", err,
			)
		} else {
			commitSHA, manifestPath, err := c.pushToGit(ctx, req)
			if err != nil {
				// Log the error but don't fail the deployment
				result.GitError = err.Error()
				result.Status = StatusGitOpsFailed
				c.logger.Warn("hybrid deployment Git push failed (deployment succeeded)",
					"instanceId", req.InstanceID,
					"error", err,
				)
			} else {
				result.GitPushed = true
				result.GitCommitSHA = commitSHA
				result.ManifestPath = manifestPath
				c.logger.Info("hybrid deployment Git push succeeded",
					"instanceId", req.InstanceID,
					"commitSha", commitSHA,
				)
			}
		}
	} else {
		c.logger.Warn("hybrid deployment skipping Git push (no repository configured)",
			"instanceId", req.InstanceID,
		)
	}

	// If Git push failed but cluster succeeded, still mark as ready
	// but with a warning status indicating GitOps component failed
	if result.Status != StatusGitOpsFailed {
		result.Status = StatusReady
	}

	c.logger.Info("hybrid deployment completed",
		"instanceId", req.InstanceID,
		"clusterDeployed", result.ClusterDeployed,
		"gitPushed", result.GitPushed,
	)

	return result, nil
}

// applyToCluster creates or updates the resource in the Kubernetes cluster.
// It builds a standard (non-suspended) object from the DeployRequest, then
// delegates to applyObjectToCluster for the actual Create-then-Patch upsert.
func (c *Controller) applyToCluster(ctx context.Context, req *DeployRequest) error {
	// Build labels, annotations, and metadata via shared builder
	// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label
	// Note: app.kubernetes.io/managed-by will be set by KRO or GitOps tool
	mb := NewInstanceMetadataBuilder(req)
	obj := mb.BuildUnstructured(req.APIVersion, req.Kind, req.Spec)
	return c.applyObjectToCluster(ctx, obj, req)
}

// applyObjectToCluster creates or updates a pre-built unstructured object in the
// Kubernetes cluster. It uses a Create-then-Patch upsert pattern: Create is attempted
// first (fast path for new resources); on AlreadyExists, it falls back to MergePatch.
// Transient errors on Create are retried with exponential backoff and jitter.
func (c *Controller) applyObjectToCluster(ctx context.Context, obj *unstructured.Unstructured, req *DeployRequest) error {
	// Get the GVR for the resource
	gvr, err := c.getGVRFromUnstructured(obj)
	if err != nil {
		return fmt.Errorf("determine GVR for %s %s/%s: %w", req.Kind, req.Namespace, req.Name, err)
	}

	// Create with retry — AlreadyExists and permanent errors exit immediately via retry.Permanent
	err = retry.Do(ctx, c.retryConfig, func() error {
		var createErr error
		if req.IsClusterScoped {
			_, createErr = c.dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		} else {
			_, createErr = c.dynamicClient.Resource(gvr).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
		}
		if createErr == nil {
			return nil
		}
		if apierrors.IsAlreadyExists(createErr) {
			return retry.Permanent(errResourceAlreadyExists)
		}
		// Retry transient errors (network, server timeout, rate limit, conflict)
		if retry.IsRetryable(createErr) || apierrors.IsServerTimeout(createErr) || apierrors.IsTooManyRequests(createErr) || apierrors.IsConflict(createErr) {
			return createErr
		}
		// Permanent K8s error (Forbidden, Invalid, etc.) — stop retrying
		return retry.Permanent(createErr)
	})

	// Upsert fallback: resource already exists, patch with new spec.
	// MergePatch with the full object we built — this is "last writer wins" since we
	// don't set resourceVersion. Server-managed fields (status, managedFields) are
	// unaffected because they're absent from our object.
	if errors.Is(err, errResourceAlreadyExists) {
		patchBytes, marshalErr := json.Marshal(obj)
		if marshalErr != nil {
			return fmt.Errorf("marshal patch for %s %s/%s: %w", req.Kind, req.Namespace, req.Name, marshalErr)
		}
		if req.IsClusterScoped {
			_, err = c.dynamicClient.Resource(gvr).Patch(ctx, obj.GetName(), k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
		} else {
			_, err = c.dynamicClient.Resource(gvr).Namespace(req.Namespace).Patch(ctx, obj.GetName(), k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
		}
		if err != nil {
			return fmt.Errorf("patch existing %s %s/%s: %w", req.Kind, req.Namespace, req.Name, err)
		}
		c.logger.Info("resource already exists, patched with new spec",
			"kind", req.Kind, "name", req.Name, "namespace", req.Namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("create %s %s/%s: %w", req.Kind, req.Namespace, req.Name, err)
	}

	return nil
}

// deleteFromCluster removes a resource from the Kubernetes cluster.
// Used as a best-effort compensating action when Git push fails after a
// successful cluster apply in GitOps mode. IsNotFound is treated as success
// (idempotent delete).
func (c *Controller) deleteFromCluster(ctx context.Context, obj *unstructured.Unstructured, req *DeployRequest) error {
	gvr, err := c.getGVRFromUnstructured(obj)
	if err != nil {
		return fmt.Errorf("determine GVR for delete: %w", err)
	}
	return c.deleteFromClusterByGVR(ctx, gvr, obj, req)
}

// deleteFromClusterByGVR deletes a resource using a pre-resolved GVR, skipping the
// discovery round-trip. Used in the GitOps compensating-delete path where the GVR
// was already resolved during cluster apply.
func (c *Controller) deleteFromClusterByGVR(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, req *DeployRequest) error {
	var err error
	if req.IsClusterScoped {
		err = c.dynamicClient.Resource(gvr).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	} else {
		err = c.dynamicClient.Resource(gvr).Namespace(req.Namespace).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s %s/%s: %w", req.Kind, req.Namespace, req.Name, err)
	}
	return nil
}

// pushToGit pushes the manifest and metadata to the Git repository
func (c *Controller) pushToGit(ctx context.Context, req *DeployRequest) (commitSHA string, manifestPath string, err error) {
	if req.Repository == nil {
		return "", "", fmt.Errorf("repository configuration is required")
	}

	// Get the GitHub token from the Kubernetes secret
	token, err := c.getGitHubToken(ctx, req.Repository)
	if err != nil {
		return "", "", fmt.Errorf("failed to get GitHub token: %w", err)
	}

	// Create GitHub client via factory (allows test injection)
	ghClient, err := c.vcsClientFactory(ctx, token, req.Repository.Owner, req.Repository.Repo)
	if err != nil {
		return "", "", fmt.Errorf("failed to create GitHub client: %w", err)
	}
	// SECURITY: Ensure client resources are cleaned up to minimize token exposure
	defer ghClient.Close()

	// Generate the manifest YAML
	manifestYAML, err := c.generator.GenerateManifest(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate manifest: %w", err)
	}

	// Generate file paths (with path traversal protection)
	manifestPath, err = c.generator.GenerateManifestPath(req, req.Repository.BasePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate manifest path: %w", err)
	}

	// Generate commit message
	commitMessage := c.generator.GenerateCommitMessage(req)

	// Determine branch — respect per-deployment GitBranch override via GetEffectiveBranch
	branch := req.GetEffectiveBranch()

	// Commit manifest file
	// Note: .metadata folder removed - all tracking info is in manifest annotations
	commitResult, err := ghClient.CommitFile(ctx, &vcs.CommitFileRequest{
		Path:    manifestPath,
		Content: manifestYAML,
		Message: commitMessage,
		Branch:  branch,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to commit manifest: %w", err)
	}

	return commitResult.SHA, manifestPath, nil
}

// getGitHubToken retrieves the GitHub token from a Kubernetes secret
// SECURITY: The returned token should be used immediately and the calling
// code should call ghClient.Close() via defer to minimize memory exposure.
func (c *Controller) getGitHubToken(ctx context.Context, repo *RepositoryConfig) (string, error) {
	secretName := repo.GetSecretName()
	if secretName == "" {
		return "", fmt.Errorf("secret name is required")
	}

	namespace := repo.GetSecretNamespace()
	key := repo.GetSecretKey()

	secret, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	tokenBytes, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain key %s", namespace, secretName, key)
	}

	// SECURITY: Copy the token and zero the original bytes to minimize exposure
	// This reduces the window where the secret is present in multiple memory locations
	tokenStr := string(tokenBytes)
	for i := range tokenBytes {
		tokenBytes[i] = 0
	}

	return tokenStr, nil
}

// Delete removes an instance deployment
func (c *Controller) Delete(ctx context.Context, namespace, name, kind, rgdName, deletedBy string, isClusterScoped bool, repo *RepositoryConfig, mode DeploymentMode) error {
	c.logger.Info("deleting instance",
		"namespace", namespace,
		"name", name,
		"isClusterScoped", isClusterScoped,
		"mode", mode,
	)

	switch mode {
	case ModeDirect:
		// Direct mode: only delete from cluster (handled by caller)
		return nil

	case ModeGitOps:
		// GitOps mode: remove from Git
		if repo == nil {
			return fmt.Errorf("repository configuration is required for GitOps deletion")
		}
		return c.deleteFromGit(ctx, namespace, name, kind, rgdName, deletedBy, isClusterScoped, repo)

	case ModeHybrid:
		// Hybrid mode: delete from Git (cluster deletion handled by caller)
		if repo != nil {
			if err := c.deleteFromGit(ctx, namespace, name, kind, rgdName, deletedBy, isClusterScoped, repo); err != nil {
				c.logger.Warn("failed to delete from Git during hybrid delete",
					"namespace", namespace,
					"name", name,
					"error", err,
				)
				// Don't fail the deletion if Git removal fails
			}
		}
		return nil

	default:
		return nil
	}
}

// deleteFromGit removes the manifest file from Git
func (c *Controller) deleteFromGit(ctx context.Context, namespace, name, kind, rgdName, deletedBy string, isClusterScoped bool, repo *RepositoryConfig) error {
	token, err := c.getGitHubToken(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	ghClient, err := c.vcsClientFactory(ctx, token, repo.Owner, repo.Repo)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	// SECURITY: Ensure client resources are cleaned up to minimize token exposure
	defer ghClient.Close()

	// Generate path to delete (consistent with create path)
	manifestPath, pathErr := ManifestPathFor(name, namespace, kind, isClusterScoped, repo.BasePath)
	if pathErr != nil {
		return fmt.Errorf("failed to generate manifest path for deletion: %w", pathErr)
	}

	// Generate delete commit message — scope-aware
	commitMessage := c.generator.GenerateDeleteCommitMessage(namespace, name, rgdName, deletedBy, isClusterScoped)

	branch := repo.GetBranch()

	// Delete manifest file
	err = ghClient.DeleteFile(ctx, manifestPath, branch, commitMessage)
	if err != nil {
		return fmt.Errorf("failed to delete manifest from Git: %w", err)
	}

	return nil
}

// getGVRFromUnstructured extracts GroupVersionResource from an unstructured object.
// Uses the discovery client for correct plural resolution; returns an error if discovery is unavailable.
func (c *Controller) getGVRFromUnstructured(obj *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	apiVersion := obj.GetAPIVersion()
	kind := obj.GetKind()

	if apiVersion == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion is required")
	}
	if kind == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("kind is required")
	}

	// Parse apiVersion (e.g., "apps/v1" -> group="apps", version="v1"
	// or "v1" -> group="", version="v1")
	var group, version string
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		group = ""
		version = parts[0]
	} else if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid apiVersion format: %s", apiVersion)
	}

	// Try discovery-based resolution first.
	// Common irregular K8s plurals that discovery handles correctly:
	// Policy→policies, Ingress→ingresses, EndpointSlice→endpointslices.
	// Without discovery we cannot safely guess plurals.
	if c.discoveryClient != nil {
		gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
		groupResources, discoveryErr := restmapper.GetAPIGroupResources(c.discoveryClient)
		if discoveryErr == nil {
			mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
			mapping, mappingErr := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if mappingErr == nil {
				return mapping.Resource, nil
			}
			c.logger.Warn("discovery REST mapping failed for kind",
				"kind", kind, "error", mappingErr)
			return schema.GroupVersionResource{}, fmt.Errorf("cannot determine plural resource name for kind %q: %w", kind, mappingErr)
		}
		c.logger.Warn("discovery API group resources failed for kind",
			"kind", kind, "error", discoveryErr)
		return schema.GroupVersionResource{}, fmt.Errorf("cannot determine plural resource name for kind %q (discovery unavailable): %w", kind, discoveryErr)
	}

	return schema.GroupVersionResource{}, fmt.Errorf("cannot determine plural resource name for kind %q: no discovery client available", kind)
}
