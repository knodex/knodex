// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Deployment mode types for the dashboard
 */

/**
 * Deployment mode determines how instances are deployed
 */
export type DeploymentMode = "direct" | "gitops" | "hybrid";

/**
 * Git push status for tracking async Git operations
 */
export type GitPushStatus = "not_applicable" | "pending" | "in_progress" | "completed" | "success" | "failed";

/**
 * Git information for an instance deployment
 */
export interface GitInfo {
  repositoryId?: string;
  commitSha?: string;
  commitUrl?: string;
  branch?: string;
  path?: string;
  pushStatus: GitPushStatus;
  pushError?: string;
  pushedAt?: string;
}

/**
 * Repository configuration for GitOps deployments
 */
export interface Repository {
  id: string;
  name: string;
  provider: string;
  owner: string;
  repo: string;
  branch: string;
  basePath?: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * Repository list response
 */
export interface RepositoryListResponse {
  items: Repository[];
  count: number;
}

/**
 * Request body for creating a repository configuration
 */
export interface CreateRepositoryRequest {
  name: string;
  provider: string;
  owner: string;
  repo: string;
  branch: string;
  basePath?: string;
  secretRef: {
    namespace: string;
    name: string;
    key: string;
  };
}

/**
 * Extended create instance request with deployment mode
 */
export interface CreateInstanceWithDeploymentRequest {
  name: string;
  namespace: string;
  rgdName: string;
  rgdNamespace?: string;
  spec: Record<string, unknown>;
  deploymentMode?: DeploymentMode;
  repositoryId?: string;
}

/**
 * Extended create instance response with deployment info
 */
export interface CreateInstanceWithDeploymentResponse {
  name: string;
  namespace: string;
  rgdName: string;
  apiGroup: string;
  kind: string;
  version: string;
  status: string;
  createdAt: string;
  deploymentMode?: DeploymentMode;
  gitInfo?: GitInfo;
}

/**
 * Deployment mode metadata for display
 */
export const DEPLOYMENT_MODE_INFO: Record<DeploymentMode, {
  label: string;
  description: string;
  icon: string;
  requiresRepository: boolean;
}> = {
  direct: {
    label: "Direct",
    description: "Deploy directly to the Kubernetes cluster via API",
    icon: "zap",
    requiresRepository: false,
  },
  gitops: {
    label: "GitOps",
    description: "Push manifest to Git repository (ArgoCD/Flux will sync)",
    icon: "git-branch",
    requiresRepository: true,
  },
  hybrid: {
    label: "Hybrid",
    description: "Deploy to cluster immediately AND push to Git repository",
    icon: "layers",
    requiresRepository: true,
  },
};

/**
 * Git push status display metadata
 */
export const GIT_PUSH_STATUS_INFO: Record<GitPushStatus, {
  label: string;
  color: string;
  icon: string;
}> = {
  not_applicable: {
    label: "N/A",
    color: "gray",
    icon: "minus",
  },
  pending: {
    label: "Pending",
    color: "yellow",
    icon: "clock",
  },
  in_progress: {
    label: "In Progress",
    color: "blue",
    icon: "loader",
  },
  completed: {
    label: "Completed",
    color: "green",
    icon: "check-circle",
  },
  failed: {
    label: "Failed",
    color: "red",
    icon: "x-circle",
  },
};
