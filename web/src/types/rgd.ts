// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { SecretRef } from "./secret";

/**
 * RGD (ResourceGraphDefinition) types for the catalog
 */

/**
 * GVKRef identifies a Kubernetes resource by Group, Version, and Kind.
 */
export interface GVKRef {
  group: string;
  version: string;
  kind: string;
}

/**
 * CatalogRGD represents an RGD as returned by the catalog API
 */
export interface CatalogRGD {
  name: string;
  /** Human-readable display title from knodex.io/title annotation (falls back to name) */
  title?: string;
  namespace: string;
  description: string;
  tags: string[];
  category: string;
  icon?: string;
  /** Documentation URL from knodex.io/docs-url annotation */
  docsUrl?: string;
  labels: Record<string, string>;
  instances: number;
  apiVersion?: string;
  kind?: string;
  /** Parent RGD Kinds this RGD extends (from knodex.io/extends-kind annotation) */
  extendsKinds?: string[];
  /** KRO processing state (e.g., "Active") */
  status?: string;
  /** Most recently issued GraphRevision number (0 or absent = no revisions) */
  lastIssuedRevision?: number;
  /** Kinds of external references this RGD depends on (populated from externalRef resources) */
  dependsOnKinds?: string[];
  /** GVKs of non-external resources this RGD produces (populated from spec.resources[]) */
  producesKinds?: GVKRef[];
  /** Allowed deployment modes for this RGD. If empty/undefined, all modes are allowed. */
  allowedDeploymentModes?: DeploymentMode[];
  /** Whether this RGD produces cluster-scoped (non-namespaced) instances */
  isClusterScoped?: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * RGDListResponse represents the paginated response from the RGD list API
 */
export interface RGDListResponse {
  items: CatalogRGD[];
  totalCount: number;
  page: number;
  pageSize: number;
}

/**
 * RGDListParams represents the query parameters for listing RGDs
 */
export interface RGDListParams {
  namespace?: string;
  category?: string;
  tags?: string[];
  search?: string;
  /** Filter RGDs that extend the specified Kind */
  extendsKind?: string;
  dependsOnKind?: string;
  /** Filter RGDs that produce the specified Kind as a non-external resource */
  producesKind?: string;
  /** Narrows producesKind to a specific API group (optional) */
  producesGroup?: string;
  /** Filter by RGD status. Always "Active" in the frontend. */
  status?: string;
  page?: number;
  pageSize?: number;
  sortBy?: "name" | "namespace" | "createdAt" | "updatedAt" | "category";
  sortOrder?: "asc" | "desc";
}

/**
 * Filter options returned by the /api/v1/rgds/filters endpoint.
 * Categories are already filtered by the user's Casbin authorization.
 */
export interface RGDFilterOptions {
  projects: string[];
  tags: string[];
  categories: string[];
}

/**
 * Category icons mapping
 */
export const CATEGORY_ICONS: Record<string, string> = {
  database: "database",
  storage: "hard-drive",
  networking: "network",
  compute: "server",
  messaging: "message-square",
  monitoring: "activity",
  security: "shield",
  default: "box",
};

/**
 * Get the icon for a category
 */
export function getCategoryIcon(category: string): string {
  return CATEGORY_ICONS[category.toLowerCase()] || CATEGORY_ICONS.default;
}

/**
 * Resource Graph types - internal resources within an RGD
 */

/**
 * External reference information for a resource
 */
export interface ExternalRefInfo {
  apiVersion: string;
  kind: string;
  nameExpr: string;
  namespaceExpr?: string;
  usesSchemaSpec: boolean;
  schemaField?: string;
}

/**
 * Iterator describes a single forEach dimension on a collection resource
 */
export interface Iterator {
  name: string;
  expression: string;
  source: "schema" | "resource" | "literal";
  sourcePath: string;
  dimensionIndex: number;
}

/**
 * A parse error encountered during graph construction
 */
export interface ParseError {
  resourceID: string;
  field: string;
  message: string;
}

/**
 * A resource node in the internal resource graph
 */
export interface ResourceNode {
  id: string;
  apiVersion: string;
  kind: string;
  isTemplate: boolean;
  isConditional: boolean;
  conditionExpr?: string;
  dependsOn: string[];
  externalRef?: ExternalRefInfo;
  isCollection?: boolean;
  forEach?: Iterator[];
  readyWhen?: string[];
}

/**
 * An edge between resources in the resource graph
 */
export interface ResourceEdge {
  from: string;
  to: string;
  type: string;
}

/**
 * Resource graph showing internal resources of an RGD
 */
export interface ResourceGraph {
  rgdName: string;
  rgdNamespace: string;
  resources: ResourceNode[];
  edges: ResourceEdge[];
  topologicalOrder?: string[];
  parseErrors?: ParseError[];
}

/**
 * ExternalRef selector metadata for form fields
 * Used to populate dropdowns with existing K8s resources
 */
export interface ExternalRefSelectorMetadata {
  apiVersion: string;
  kind: string;
  /** When true, the dropdown should filter by the deployment namespace */
  useInstanceNamespace?: boolean;
  /** Maps resource attributes to sub-field names for auto-populating from a resource picker.
   * Example: { name: "name", namespace: "namespace" } */
  autoFillFields?: Record<string, string>;
}

/**
 * Form schema property
 */
export interface FormProperty {
  type: string;
  title?: string;
  description?: string;
  default?: unknown;
  enum?: unknown[];
  format?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  properties?: Record<string, FormProperty>;
  /** Display order for nested object fields */
  propertyOrder?: string[];
  required?: string[];
  items?: FormProperty;
  "x-kubernetes-preserve-unknown-fields"?: boolean;
  nullable?: boolean;
  path?: string;
  externalRefSelector?: ExternalRefSelectorMetadata;
  /** Indicates this field is under the advanced section and hidden by default */
  isAdvanced?: boolean;
}

/**
 * A structured condition rule extracted from a CEL expression via AST analysis.
 * Used for client-side evaluation of field visibility without parsing CEL strings.
 */
export interface ConditionRule {
  /** Schema field path (e.g., "spec.enableDatabase") */
  field: string;
  /** Comparison operator ("==", "!=", ">", "<", ">=", "<=") */
  op: string;
  /** Comparison target value (true, false, 42, "premium") */
  value: unknown;
}

/**
 * Conditional section - controls visibility of form fields based on other field values
 */
export interface ConditionalSection {
  /** CEL expression from includeWhen */
  condition: string;
  /** schema.spec.* path that controls visibility */
  controllingField: string;
  /** Value that makes the section visible (for non-boolean fields) */
  expectedValue?: unknown;
  /** Property paths that should be hidden/shown */
  affectedProperties: string[];
  /** Structured condition rules extracted via CEL AST analysis */
  rules?: ConditionRule[];
  /** Whether the frontend can evaluate this condition using structured rules.
   * When false, falls back to expectedValue evaluation or shows fields (fail open). */
  clientEvaluable?: boolean;
}

/**
 * Advanced section - configuration hidden by default
 */
export interface AdvancedSection {
  /** Base path for advanced config (e.g., "advanced") */
  path: string;
  /** All property paths under advanced */
  affectedProperties: string[];
}

/**
 * Form schema extracted from CRD
 */
export interface FormSchema {
  name: string;
  namespace: string;
  group: string;
  kind: string;
  version: string;
  title?: string;
  description?: string;
  properties: Record<string, FormProperty>;
  /** Display order for top-level form fields */
  propertyOrder?: string[];
  required?: string[];
  conditionalSections?: ConditionalSection[];
  /** Whether this RGD produces cluster-scoped instances (no namespace needed) */
  isClusterScoped?: boolean;
  /** Per-feature and global advanced configuration toggles */
  advancedSections?: AdvancedSection[];
}

/**
 * Schema API response
 */
export interface SchemaResponse {
  rgd: string;
  schema: FormSchema | null;
  /** ExternalRef resources that reference Kubernetes Secrets */
  secretRefs?: SecretRef[];
  error?: string;
  /** Non-fatal warnings encountered during schema generation */
  warnings?: string[];
  crdFound: boolean;
  /** Schema source: "crd+rgd" (full) or "rgd-only" (degraded, missing validation constraints) */
  source?: "crd+rgd" | "rgd-only";
}

/**
 * Instance health status
 */
export type InstanceHealth =
  | "Healthy"
  | "Degraded"
  | "Unhealthy"
  | "Progressing"
  | "Unknown";

/**
 * Instance condition from Kubernetes status
 */
export interface InstanceCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}

/**
 * Git push status for tracking async Git operations
 */
export type GitPushStatus =
  | "not_applicable"
  | "pending"
  | "in_progress"
  | "completed"
  | "success"
  | "failed";

/**
 * Git information for an instance deployment
 */
export interface GitInfo {
  repositoryId?: string;
  /** Human-readable repository reference, e.g. "owner/repo" */
  repositoryUrl?: string;
  commitSha?: string;
  commitUrl?: string;
  branch?: string;
  path?: string;
  pushStatus: GitPushStatus;
  pushError?: string;
  pushedAt?: string;
}

/**
 * Deployment mode determines how instances are deployed
 */
export type DeploymentMode = "direct" | "gitops" | "hybrid";

/**
 * Shared fields for all instances (both namespaced and cluster-scoped)
 */
interface InstanceBase {
  name: string;
  rgdName: string;
  rgdNamespace: string;
  apiVersion: string;
  kind: string;
  health: InstanceHealth;
  conditions: InstanceCondition[];
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  createdAt: string;
  updatedAt: string;
  deploymentMode?: DeploymentMode;
  gitInfo?: GitInfo;
  /** Kubernetes unique identifier */
  uid?: string;
  /** KRO status of the parent RGD (e.g., "Active") */
  rgdStatus?: string;
  /** Icon slug from the parent RGD's knodex.io/icon annotation */
  rgdIcon?: string;
  /** Category from the parent RGD's knodex.io/category annotation */
  rgdCategory?: string;
  /** Kubernetes resource version for optimistic concurrency control */
  resourceVersion?: string;
  /** True when kro.run/reconcile: suspended is set — KRO holds off creating child resources */
  reconciliationSuspended?: boolean;
  /** True when live spec doesn't match the desired spec pushed to Git */
  gitopsDrift?: boolean;
  /** The desired spec that was pushed to Git (for drift comparison) */
  desiredSpec?: Record<string, unknown>;
  /** ISO8601 timestamp of when drift was first detected (from Git push time) */
  driftedAt?: string;
}

/**
 * Represents a deployed instance of an RGD.
 *
 * Discriminated union enforcing that cluster-scoped instances (isClusterScoped: true)
 * must have an empty namespace, preventing invalid state at compile time.
 *
 * Note: TypeScript's structural type system cannot statically prevent
 * `{ isClusterScoped: false, namespace: "" }` without a branded NonEmptyString
 * type, which would require invasive changes across all Instance-creating code.
 * The union enforces the primary invariant: `isClusterScoped: true` ↔ `namespace: ""`.
 * Use `hasValidNamespace()` for runtime validation at API boundaries.
 */
export type Instance = InstanceBase &
  (
    | { isClusterScoped?: false | undefined; namespace: string }
    | { isClusterScoped: true; namespace: "" }
  );

/**
 * Runtime guard validating the namespace/isClusterScoped invariant.
 * Cluster-scoped instances must have an empty namespace; namespaced instances
 * must have a non-empty namespace.
 *
 * Use at API response boundaries to catch server-side inconsistencies.
 */
export function hasValidNamespace(
  instance: Pick<Instance, "namespace" | "isClusterScoped">
): boolean {
  if (instance.isClusterScoped) return instance.namespace === "";
  return instance.namespace.length > 0;
}

/**
 * GitOps location annotation keys
 */
export const GITOPS_ANNOTATIONS = {
  enabled: "gitops.knodex.io/enabled",
  vcs: "gitops.knodex.io/vcs",
  repository: "gitops.knodex.io/repository",
  owner: "gitops.knodex.io/owner",
  repo: "gitops.knodex.io/repo",
  branch: "gitops.knodex.io/branch",
  path: "gitops.knodex.io/path",
} as const;

/**
 * GitOps location information extracted from instance annotations
 */
export interface GitOpsLocation {
  enabled: boolean;
  vcs?: string;
  repository?: string;
  owner?: string;
  repo?: string;
  branch?: string;
  path?: string;
}

/**
 * Extract GitOps location from instance annotations
 */
export function extractGitOpsLocation(
  annotations?: Record<string, string>
): GitOpsLocation | null {
  if (!annotations) return null;
  if (annotations[GITOPS_ANNOTATIONS.enabled] !== "true") return null;

  return {
    enabled: true,
    vcs: annotations[GITOPS_ANNOTATIONS.vcs],
    repository: annotations[GITOPS_ANNOTATIONS.repository],
    owner: annotations[GITOPS_ANNOTATIONS.owner],
    repo: annotations[GITOPS_ANNOTATIONS.repo],
    branch: annotations[GITOPS_ANNOTATIONS.branch],
    path: annotations[GITOPS_ANNOTATIONS.path],
  };
}

/**
 * Build a URL to view the manifest file in the VCS web UI
 * Supports GitHub, GitLab, and Bitbucket
 */
export function buildGitOpsFileURL(location: GitOpsLocation): string | null {
  if (
    !location.enabled ||
    !location.owner ||
    !location.repo ||
    !location.branch ||
    !location.path
  ) {
    return null;
  }

  const vcs = (location.vcs || "github").toLowerCase();

  switch (vcs) {
    case "github":
      return `https://github.com/${location.owner}/${location.repo}/blob/${location.branch}/${location.path}`;
    case "gitlab":
      return `https://gitlab.com/${location.owner}/${location.repo}/-/blob/${location.branch}/${location.path}`;
    case "bitbucket":
      return `https://bitbucket.org/${location.owner}/${location.repo}/src/${location.branch}/${location.path}`;
    default:
      return `https://github.com/${location.owner}/${location.repo}/blob/${location.branch}/${location.path}`;
  }
}

/**
 * Parameters for listing instances
 */
export interface InstanceListParams {
  namespace?: string;
  rgdName?: string;
  rgdNamespace?: string;
  health?: InstanceHealth;
  search?: string;
  page?: number;
  pageSize?: number;
  sortBy?: "name" | "namespace" | "createdAt" | "health";
  sortOrder?: "asc" | "desc";
}

/**
 * Response for listing instances
 */
export interface InstanceListResponse {
  items: Instance[];
  totalCount: number;
  page: number;
  pageSize: number;
}

/**
 * Request body for creating an instance
 */
export interface CreateInstanceRequest {
  name: string;
  namespace?: string; // Optional for cluster-scoped instances
  projectId?: string; // Project ID for RBAC and policy validation
  rgdName: string;
  rgdNamespace?: string;
  spec: Record<string, unknown>;
  deploymentMode?: DeploymentMode;
  repositoryId?: string;
  gitBranch?: string; // Optional: Override default branch for GitOps deployment
  gitPath?: string; // Optional: Override semantic path for GitOps deployment
  clusterRef?: string; // Optional: Target CAPI cluster for multi-cluster deployments
}

/**
 * Response after creating an instance
 */
export interface CreateInstanceResponse {
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
 * Request body for updating an instance spec
 */
export interface UpdateInstanceRequest {
  spec: Record<string, unknown>;
  resourceVersion?: string;
  repositoryId?: string;
  gitBranch?: string;
  gitPath?: string;
}

/**
 * Response after updating an instance spec
 */
export interface UpdateInstanceResponse {
  name: string;
  namespace: string;
  kind: string;
  status: string;
  deploymentMode?: DeploymentMode;
  gitInfo?: GitInfo;
}

/**
 * GitOps instance status for tracking deployment progress
 */
export type GitOpsInstanceStatus =
  | "PushedToGit"
  | "WaitingForSync"
  | "Syncing"
  | "Creating"
  | "Ready"
  | "GitOpsFailed";

/**
 * Pending GitOps instance
 */
export interface PendingInstance {
  instanceId: string;
  name: string;
  namespace: string;
  rgdName: string;
  rgdNamespace?: string;
  deploymentMode: string;
  status: string;
  pushedAt: string;
  commitSha?: string;
  isStuck: boolean;
}

/**
 * Response for pending instances API
 */
export interface PendingInstancesResponse {
  items: PendingInstance[];
  total: number;
}

/**
 * Response for stuck instances API
 */
export interface StuckInstancesResponse {
  items: PendingInstance[];
  total: number;
  stuckThreshold: string;
}

/**
 * Response for count endpoints (RGDs and Instances)
 */
export interface CountResponse {
  count: number;
}

/**
 * GraphRevision condition (e.g., GraphVerified, Ready)
 */
export interface GraphRevisionCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
}

/**
 * GraphRevision represents a KRO GraphRevision CRD
 */
export interface GraphRevision {
  revisionNumber: number;
  rgdName: string;
  namespace: string;
  conditions: GraphRevisionCondition[];
  contentHash?: string;
  createdAt: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  resourceVersion?: string;
  snapshot?: Record<string, unknown>;
}

/**
 * Paginated list of GraphRevisions
 */
export interface GraphRevisionList {
  items: GraphRevision[];
  totalCount: number;
}

/**
 * A single changed field in a revision diff
 */
export interface DiffField {
  /** Dot-delimited field path (e.g., "spec.resources[0].apiVersion") */
  path: string;
  /** Value in the older revision (absent for added fields) */
  oldValue?: unknown;
  /** Value in the newer revision (absent for removed fields) */
  newValue?: unknown;
}

/**
 * Structured diff between two RGD revision snapshots
 */
export interface RevisionDiff {
  rgdName: string;
  /** Older (smaller) revision number */
  rev1: number;
  /** Newer (larger) revision number */
  rev2: number;
  /** Fields present in rev2 but not in rev1 */
  added: DiffField[];
  /** Fields present in rev1 but not in rev2 */
  removed: DiffField[];
  /** Fields that exist in both revisions but with different values */
  modified: DiffField[];
  /** True when there are no differences between the two revisions */
  identical: boolean;
}

/**
 * ChildResource represents a single Kubernetes resource created by KRO
 * as part of an instance's resource graph.
 */
export interface ChildResource {
  name: string;
  namespace: string;
  kind: string;
  apiVersion: string;
  nodeId: string;
  health: string;
  phase?: string;
  /** Human-readable status message from status.message (e.g., condition details) */
  status?: string;
  createdAt: string;
  labels?: Record<string, string>;
  /** Cluster name where this child resource lives (empty = management cluster) */
  cluster?: string;
  /** Cluster connectivity status ("unreachable" when cluster is down) */
  clusterStatus?: string;
}

/**
 * ChildResourceGroup groups child resources by their node-id within
 * the RGD resource graph.
 */
export interface ChildResourceGroup {
  nodeId: string;
  kind: string;
  apiVersion: string;
  count: number;
  readyCount: number;
  health: string;
  resources: ChildResource[];
}

/**
 * ChildResourceResponse is the API response for listing child resources
 * of an instance, grouped by node-id.
 */
export interface ChildResourceResponse {
  instanceName: string;
  instanceNamespace: string;
  instanceKind: string;
  totalCount: number;
  groups: ChildResourceGroup[];
  /** True when one or more target clusters are unreachable */
  clusterUnreachable?: boolean;
  /** Names of clusters that are unreachable */
  unreachableClusters?: string[];
}
