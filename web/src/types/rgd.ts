/**
 * RGD (ResourceGraphDefinition) types for the catalog
 */

/**
 * CatalogRGD represents an RGD as returned by the catalog API
 */
export interface CatalogRGD {
  name: string;
  /** Human-readable display title from knodex.io/title annotation (falls back to name) */
  title?: string;
  namespace: string;
  description: string;
  version: string;
  tags: string[];
  category: string;
  icon?: string;
  labels: Record<string, string>;
  instances: number;
  apiVersion?: string;
  kind?: string;
  /** KRO processing state (e.g., "Active", "Inactive") */
  status?: string;
  /** Allowed deployment modes for this RGD. If empty/undefined, all modes are allowed. */
  allowedDeploymentModes?: DeploymentMode[];
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
  page?: number;
  pageSize?: number;
  sortBy?: "name" | "namespace" | "createdAt" | "updatedAt" | "category";
  sortOrder?: "asc" | "desc";
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
  required?: string[];
  conditionalSections?: ConditionalSection[];
  /** Advanced configuration toggle for fields under spec.advanced */
  advancedSection?: AdvancedSection;
}

/**
 * Schema API response
 */
export interface SchemaResponse {
  rgd: string;
  schema: FormSchema | null;
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
  | "failed";

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
 * Deployment mode determines how instances are deployed
 */
export type DeploymentMode = "direct" | "gitops" | "hybrid";

/**
 * Represents a deployed instance of an RGD
 */
export interface Instance {
  name: string;
  namespace: string;
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
  /** Kubernetes resource version for optimistic concurrency control */
  resourceVersion?: string;
  /** True when live spec doesn't match the desired spec pushed to Git */
  gitopsDrift?: boolean;
  /** The desired spec that was pushed to Git (for drift comparison) */
  desiredSpec?: Record<string, unknown>;
}

/**
 * Annotation key for GitOps instance ID (used for correlation)
 */
export const INSTANCE_ID_ANNOTATION = "knodex.io/instance-id";

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
  namespace: string; // Required: Namespace for deployment
  projectId?: string; // Project ID for RBAC and policy validation
  rgdName: string;
  rgdNamespace?: string;
  spec: Record<string, unknown>;
  deploymentMode?: DeploymentMode;
  repositoryId?: string;
  gitBranch?: string; // Optional: Override default branch for GitOps deployment
  gitPath?: string; // Optional: Override semantic path for GitOps deployment
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
 * Status transition in the instance timeline
 */
export interface StatusTransition {
  fromStatus: string;
  toStatus: string;
  timestamp: string;
  message?: string;
}

/**
 * Response for status timeline API
 */
export interface StatusTimelineResponse {
  instanceId: string;
  name?: string;
  namespace?: string;
  currentStatus?: string;
  deploymentMode?: string;
  pushedAt?: string;
  isStuck: boolean;
  timeline: StatusTransition[];
}

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
