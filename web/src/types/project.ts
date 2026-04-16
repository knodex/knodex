// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project types for the dashboard
 * Projects are ArgoCD-compatible constructs that define deployment boundaries
 */

/**
 * Destination defines an allowed deployment target
 * Simplified for single-cluster deployments
 */
export interface Destination {
  /** Target namespace (supports wildcards: "*", "dev-*", "team-*") */
  namespace?: string;
  /** Optional friendly name for the destination */
  name?: string;
}

/**
 * Role defines a set of policies for a project
 */
export interface ProjectRole {
  /** Unique name of the role within the project */
  name: string;
  /** Human-readable description of the role */
  description?: string;
  /** Policy strings defining permissions */
  policies?: string[];
  /** OIDC groups assigned to this role */
  groups?: string[];
  /** Optional namespace patterns scoping this role to specific destinations */
  destinations?: string[];
}

/**
 * ClusterBinding references a CAPI Cluster bound to a project
 */
export interface ClusterBinding {
  /** Name of the CAPI Cluster resource */
  clusterRef: string;
}

/**
 * ClusterPhase represents the provisioning state of a cluster binding
 */
export type ClusterPhase =
  | "Pending"
  | "Provisioning"
  | "Provisioned"
  | "Deleting"
  | "Deleted"
  | "Failed"
  | "Unknown"
  | "ClusterUnreachable";

/**
 * ClusterState tracks the provisioning status of a single cluster binding
 */
export interface ClusterState {
  clusterRef: string;
  phase: ClusterPhase;
  message?: string;
}

/**
 * Returns true when the project has bound clusters (multi-cluster mode)
 */
export function isMultiCluster(project: Project): boolean {
  return Array.isArray(project.clusters) && project.clusters.length > 0;
}

/**
 * Aggregated resource from a remote cluster
 */
export interface AggregatedResource {
  name: string;
  kind: string;
  cluster: string;
  namespace: string;
  status: string;
  age: string;
}

/**
 * Cluster resource status in the aggregation response
 */
export interface ClusterResourceStatus {
  phase: "ready" | "unreachable" | "error";
  message?: string;
}

/**
 * Response from the resource aggregation endpoint
 */
export interface ResourceAggregationResponse {
  items: AggregatedResource[];
  totalCount: number;
  clusterStatus?: Record<string, ClusterResourceStatus>;
}

/**
 * Project represents an ArgoCD-compatible project
 */
export interface Project {
  /** Unique identifier for the project (DNS-1123 subdomain format) */
  name: string;
  /** Project tier: "platform" or "app" */
  type: "platform" | "app";
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles defined in this project */
  roles?: ProjectRole[];
  /** CAPI cluster bindings (App Projects only). Empty for monocluster mode. */
  clusters?: ClusterBinding[];
  /** Namespace claim across bound clusters (App Projects only) */
  namespace?: string;
  /** Per-cluster provisioning state tracking */
  clusterStates?: ClusterState[];
  /** Version used for optimistic locking */
  resourceVersion: string;
  /** When the project was created */
  createdAt: string;
  /** Who created the project */
  createdBy?: string;
  /** When the project was last updated */
  updatedAt?: string;
  /** Who last updated the project */
  updatedBy?: string;
}

/**
 * Request body for creating a project
 */
export interface CreateProjectRequest {
  /** Unique identifier for the project (DNS-1123 subdomain format) */
  name: string;
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles to create with the project */
  roles?: ProjectRole[];
}

/**
 * Request body for updating a project
 * Added roles for project admin to update policies and groups
 */
export interface UpdateProjectRequest {
  /** Human-readable description of the project */
  description?: string;
  /** Allowed deployment destinations */
  destinations?: Destination[];
  /** Roles to update (project admins can update role policies and groups) */
  roles?: ProjectRole[];
  /** Resource version for optimistic locking */
  resourceVersion: string;
}

/**
 * Response from listing projects
 */
export interface ProjectListResponse {
  /** List of projects */
  items: Project[];
  /** Total number of projects */
  totalCount: number;
}
