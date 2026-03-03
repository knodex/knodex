/**
 * OPA Gatekeeper Compliance types
 * Enterprise-only feature for policy compliance monitoring
 */

/**
 * ConstraintTemplate represents an OPA Gatekeeper ConstraintTemplate.
 * ConstraintTemplates define the Rego policy logic and create a new constraint CRD kind.
 */
export interface ConstraintTemplate {
  /** ConstraintTemplate resource name */
  name: string;

  /** Constraint kind this template creates (e.g., K8sRequiredLabels) */
  kind: string;

  /** Human-readable explanation of what the template enforces */
  description: string;

  /** Rego policy logic (optional in responses) */
  rego?: string;

  /** Schema for constraint parameters */
  parameters?: Record<string, unknown>;

  /** Kubernetes labels on the ConstraintTemplate */
  labels?: Record<string, string>;

  /** When the ConstraintTemplate was created */
  createdAt: string;
}

/**
 * Constraint represents an active OPA Gatekeeper constraint instance.
 * Constraints are created from ConstraintTemplates and define the scope of enforcement.
 */
export interface Constraint {
  /** Constraint resource name */
  name: string;

  /** Constraint kind (matches the ConstraintTemplate's generated kind) */
  kind: string;

  /** Name of the ConstraintTemplate this constraint uses */
  templateName: string;

  /** What happens on violation: deny, dryrun, or warn */
  enforcementAction: EnforcementAction;

  /** Which resources this constraint applies to */
  match: ConstraintMatch;

  /** Values passed to the Rego policy */
  parameters?: Record<string, unknown>;

  /** Current number of violations from the audit controller */
  violationCount: number;

  /** Detailed violation data from the audit controller */
  violations?: Violation[];

  /** Kubernetes labels on the constraint */
  labels?: Record<string, string>;

  /** When the constraint was created */
  createdAt: string;
}

/**
 * ConstraintMatch defines which resources a constraint applies to.
 */
export interface ConstraintMatch {
  /** Resource kinds to match */
  kinds?: MatchKind[];

  /** Namespaces to limit enforcement (empty = all) */
  namespaces?: string[];

  /** Scope of resources: Cluster, Namespaced, or * */
  scope?: string;
}

/**
 * MatchKind specifies a group of Kubernetes resource kinds to match.
 */
export interface MatchKind {
  /** API groups to match (empty string = core API group) */
  apiGroups: string[];

  /** Resource kinds to match within the API groups */
  kinds: string[];
}

/**
 * Violation represents a policy violation detected by Gatekeeper's audit controller.
 */
export interface Violation {
  /** Name of the constraint that was violated */
  constraintName: string;

  /** Kind of the constraint that was violated */
  constraintKind: string;

  /** Kubernetes resource that violates the constraint */
  resource: ViolationResource;

  /** Human-readable violation message from the Rego policy */
  message: string;

  /** Action taken: deny, dryrun, or warn */
  enforcementAction: string;
}

/**
 * ViolationResource identifies a Kubernetes resource that violated a constraint.
 */
export interface ViolationResource {
  /** Resource kind (e.g., Pod, Deployment) */
  kind: string;

  /** Resource namespace (empty for cluster-scoped) */
  namespace: string;

  /** Resource name */
  name: string;

  /** API group of the resource (empty for core API group) */
  apiGroup?: string;
}

/**
 * ComplianceSummary provides aggregate Gatekeeper compliance statistics.
 */
export interface ComplianceSummary {
  /** Count of ConstraintTemplates */
  totalTemplates: number;

  /** Count of active constraints */
  totalConstraints: number;

  /** Total count of violations across all constraints */
  totalViolations: number;

  /** Violations breakdown by enforcement action (deny, warn, dryrun) */
  byEnforcement: Record<string, number>;
}

/**
 * Enforcement action types
 */
export type EnforcementAction = "deny" | "warn" | "dryrun";

/**
 * Paginated response wrapper for compliance list endpoints
 */
export interface ComplianceListResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

/**
 * Parameters for listing constraints
 */
export interface ConstraintListParams {
  kind?: string;
  enforcement?: EnforcementAction;
  page?: number;
  pageSize?: number;
}

/**
 * Parameters for listing violations
 */
export interface ViolationListParams {
  /** Filter by constraint: {kind}/{name} */
  constraint?: string;
  /** Filter by resource: {kind}/{namespace}/{name} */
  resource?: string;
  page?: number;
  pageSize?: number;
}

/**
 * Parameters for listing templates
 */
export interface TemplateListParams {
  page?: number;
  pageSize?: number;
}

/**
 * ViolationHistoryRecord represents a persisted violation history entry.
 */
export interface ViolationHistoryRecord {
  key: string;
  constraintKind: string;
  constraintName: string;
  resourceKind: string;
  resourceNamespace: string;
  resourceName: string;
  enforcementAction: string;
  message: string;
  firstSeen: string;
  resolvedAt: string | null;
  status: "active" | "resolved";
}

/**
 * Parameters for listing violation history
 */
export interface ViolationHistoryListParams {
  since?: string;
  until?: string;
  enforcement?: string;
  constraint?: string;
  resource?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}

/**
 * Parameters for counting violation history records
 */
export interface ViolationHistoryCountParams {
  since?: string;
  until?: string;
  enforcement?: string;
  constraint?: string;
  resource?: string;
}

/**
 * Response from the count endpoint
 */
export interface ViolationHistoryCountResponse {
  count: number;
  retentionDays: number;
}

/**
 * Parameters for exporting violation history as CSV
 */
export interface ViolationHistoryExportParams {
  since: string;
  enforcement?: string;
  constraint?: string;
  resource?: string;
}

/**
 * Request body for creating a new constraint
 */
export interface CreateConstraintRequest {
  /** Constraint resource name (must be DNS-compatible) */
  name: string;

  /** Name of the ConstraintTemplate to use */
  templateName: string;

  /** What happens on violation: deny, dryrun, or warn (defaults to deny) */
  enforcementAction?: EnforcementAction;

  /** Which resources this constraint applies to */
  match?: ConstraintMatch;

  /** Values passed to the Rego policy */
  parameters?: Record<string, unknown>;

  /** Kubernetes labels to apply to the constraint */
  labels?: Record<string, string>;
}

/**
 * Enforcement action color mapping for UI
 */
export const ENFORCEMENT_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  deny: {
    bg: "bg-red-50 dark:bg-red-950/30",
    text: "text-red-700 dark:text-red-400",
    border: "border-red-200 dark:border-red-900",
  },
  warn: {
    bg: "bg-yellow-50 dark:bg-yellow-950/30",
    text: "text-yellow-700 dark:text-yellow-400",
    border: "border-yellow-200 dark:border-yellow-900",
  },
  dryrun: {
    bg: "bg-blue-50 dark:bg-blue-950/30",
    text: "text-blue-700 dark:text-blue-400",
    border: "border-blue-200 dark:border-blue-900",
  },
};

/**
 * Get color classes for an enforcement action
 */
export function getEnforcementColors(action: string): { bg: string; text: string; border: string } {
  return ENFORCEMENT_COLORS[action.toLowerCase()] || ENFORCEMENT_COLORS.dryrun;
}
