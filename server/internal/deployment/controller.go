// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/deployment/vcs"
)

// Controller orchestrates instance deployments across different modes
type Controller struct {
	// Kubernetes clients
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface

	// Manifest generator
	generator *Generator

	// Logger
	logger *slog.Logger
}

// NewController creates a new deployment controller
func NewController(dynamicClient dynamic.Interface, kubeClient kubernetes.Interface, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Controller{
		dynamicClient: dynamicClient,
		kubeClient:    kubeClient,
		generator:     NewGenerator(),
		logger:        logger,
	}
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

// deployGitOps pushes the manifest to Git without applying to cluster
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

	result.Status = StatusManifestGenerated

	// Push manifest to Git
	commitSHA, manifestPath, err := c.pushToGit(ctx, req)
	if err != nil {
		result.Status = StatusFailed
		result.GitError = err.Error()
		c.logger.Error("GitOps deployment failed",
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

// applyToCluster creates the resource in the Kubernetes cluster
func (c *Controller) applyToCluster(ctx context.Context, req *DeployRequest) error {
	// Build the unstructured object
	// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label
	// Note: app.kubernetes.io/managed-by will be set by KRO or GitOps tool
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": req.APIVersion,
			"kind":       req.Kind,
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": req.Namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":    req.Name,
					"knodex.io/deployment-mode": string(req.DeploymentMode),
				},
				"annotations": map[string]interface{}{
					"knodex.io/instance-id": req.InstanceID,
					"knodex.io/created-by":  req.CreatedBy,
					"knodex.io/created-at":  req.CreatedAt.Format(time.RFC3339),
				},
			},
			"spec": req.Spec,
		},
	}

	// Add project labels/annotations if present
	if req.ProjectID != "" {
		labels := obj.GetLabels()
		labels["knodex.io/project"] = req.ProjectID
		obj.SetLabels(labels)
		annotations := obj.GetAnnotations()
		annotations["knodex.io/project-id"] = req.ProjectID
		obj.SetAnnotations(annotations)
	}

	// Get the GVR for the resource
	gvr, err := getGVRFromUnstructured(obj)
	if err != nil {
		return fmt.Errorf("failed to determine GroupVersionResource: %w", err)
	}

	// Create the resource
	_, err = c.dynamicClient.Resource(gvr).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
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

	// Create GitHub client
	ghClient, err := vcs.NewGitHubClient(ctx, token, req.Repository.Owner, req.Repository.Repo)
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

	// Determine branch
	branch := req.Repository.DefaultBranch
	if branch == "" {
		branch = "main"
	}

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
	if repo.SecretName == "" {
		return "", fmt.Errorf("secret name is required")
	}

	namespace := repo.SecretNamespace
	if namespace == "" {
		namespace = "kro-system"
	}

	key := repo.SecretKey
	if key == "" {
		key = "token"
	}

	secret, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, repo.SecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, repo.SecretName, err)
	}

	tokenBytes, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain key %s", namespace, repo.SecretName, key)
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
func (c *Controller) Delete(ctx context.Context, namespace, name, rgdName, deletedBy string, repo *RepositoryConfig, mode DeploymentMode) error {
	c.logger.Info("deleting instance",
		"namespace", namespace,
		"name", name,
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
		return c.deleteFromGit(ctx, namespace, name, rgdName, deletedBy, repo)

	case ModeHybrid:
		// Hybrid mode: delete from Git (cluster deletion handled by caller)
		if repo != nil {
			if err := c.deleteFromGit(ctx, namespace, name, rgdName, deletedBy, repo); err != nil {
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
func (c *Controller) deleteFromGit(ctx context.Context, namespace, name, rgdName, deletedBy string, repo *RepositoryConfig) error {
	token, err := c.getGitHubToken(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	ghClient, err := vcs.NewGitHubClient(ctx, token, repo.Owner, repo.Repo)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	// SECURITY: Ensure client resources are cleaned up to minimize token exposure
	defer ghClient.Close()

	// Generate path to delete
	basePath := repo.BasePath
	if basePath == "" {
		basePath = "instances"
	}
	manifestPath := fmt.Sprintf("%s/%s/%s.yaml", basePath, namespace, name)

	// Generate delete commit message
	commitMessage := c.generator.GenerateDeleteCommitMessage(namespace, name, rgdName, deletedBy)

	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	// Delete manifest file
	err = ghClient.DeleteFile(ctx, manifestPath, branch, commitMessage)
	if err != nil {
		return fmt.Errorf("failed to delete manifest from Git: %w", err)
	}

	return nil
}

// getGVRFromUnstructured extracts GroupVersionResource from an unstructured object
func getGVRFromUnstructured(obj *unstructured.Unstructured) (schema.GroupVersionResource, error) {
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
		// Core API (e.g., "v1")
		group = ""
		version = parts[0]
	} else if len(parts) == 2 {
		// Group API (e.g., "apps/v1")
		group = parts[0]
		version = parts[1]
	} else {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid apiVersion format: %s", apiVersion)
	}

	// Simple pluralization: lowercase kind + "s"
	// This works for most CRDs (e.g., "Application" -> "applications")
	resource := strings.ToLower(kind) + "s"

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}, nil
}

// SecretReader interface for getting secrets (used for testing)
type SecretReader interface {
	GetSecret(ctx context.Context, namespace, name, key string) (string, error)
}

// KubernetesSecretReader implements SecretReader using Kubernetes client
type KubernetesSecretReader struct {
	client kubernetes.Interface
}

// NewKubernetesSecretReader creates a new KubernetesSecretReader
func NewKubernetesSecretReader(client kubernetes.Interface) *KubernetesSecretReader {
	return &KubernetesSecretReader{client: client}
}

// GetSecret retrieves a secret value from Kubernetes
func (r *KubernetesSecretReader) GetSecret(ctx context.Context, namespace, name, key string) (string, error) {
	secret, err := r.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain key %s", namespace, name, key)
	}

	return string(value), nil
}

// Ensure KubernetesSecretReader implements SecretReader
var _ SecretReader = (*KubernetesSecretReader)(nil)

// unused import guard
var _ = corev1.Secret{}
