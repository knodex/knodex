import apiClient from "./client";
import type {
  ComplianceSummary,
  ComplianceListResponse,
  ConstraintTemplate,
  Constraint,
  Violation,
  ConstraintListParams,
  ViolationListParams,
  TemplateListParams,
  EnforcementAction,
  CreateConstraintRequest,
  ViolationHistoryCountParams,
  ViolationHistoryCountResponse,
  ViolationHistoryExportParams,
} from "@/types/compliance";

/**
 * Get compliance summary statistics
 * @returns Aggregate compliance statistics (templates, constraints, violations)
 */
export async function getComplianceSummary(): Promise<ComplianceSummary> {
  const response = await apiClient.get<ComplianceSummary>("/v1/compliance/summary");
  return response.data;
}

/**
 * List constraint templates with pagination
 * @param params - Pagination parameters
 * @returns Paginated list of constraint templates
 */
export async function listConstraintTemplates(
  params?: TemplateListParams
): Promise<ComplianceListResponse<ConstraintTemplate>> {
  const queryParams = new URLSearchParams();

  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString ? `/v1/compliance/templates?${queryString}` : "/v1/compliance/templates";

  const response = await apiClient.get<ComplianceListResponse<ConstraintTemplate>>(url);
  return response.data;
}

/**
 * Get a specific constraint template by name
 * @param name - Template name
 * @returns Constraint template details
 */
export async function getConstraintTemplate(name: string): Promise<ConstraintTemplate> {
  const response = await apiClient.get<ConstraintTemplate>(
    `/v1/compliance/templates/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * List constraints with optional filtering and pagination
 * @param params - Filter and pagination parameters
 * @returns Paginated list of constraints
 */
export async function listConstraints(
  params?: ConstraintListParams
): Promise<ComplianceListResponse<Constraint>> {
  const queryParams = new URLSearchParams();

  if (params?.kind) queryParams.append("kind", params.kind);
  if (params?.enforcement) queryParams.append("enforcement", params.enforcement);
  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString ? `/v1/compliance/constraints?${queryString}` : "/v1/compliance/constraints";

  const response = await apiClient.get<ComplianceListResponse<Constraint>>(url);
  return response.data;
}

/**
 * Get a specific constraint by kind and name
 * @param kind - Constraint kind
 * @param name - Constraint name
 * @returns Constraint details including violations
 */
export async function getConstraint(kind: string, name: string): Promise<Constraint> {
  const response = await apiClient.get<Constraint>(
    `/v1/compliance/constraints/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * List violations with optional filtering and pagination
 * @param params - Filter and pagination parameters
 * @returns Paginated list of violations
 */
export async function listViolations(
  params?: ViolationListParams
): Promise<ComplianceListResponse<Violation>> {
  const queryParams = new URLSearchParams();

  if (params?.constraint) queryParams.append("constraint", params.constraint);
  if (params?.resource) queryParams.append("resource", params.resource);
  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString ? `/v1/compliance/violations?${queryString}` : "/v1/compliance/violations";

  const response = await apiClient.get<ComplianceListResponse<Violation>>(url);
  return response.data;
}

/**
 * Check if the current API response indicates enterprise feature is required
 * @param error - Error object from API call
 * @returns True if error is 402 Payment Required
 */
export function isEnterpriseRequired(error: unknown): boolean {
  if (error && typeof error === "object" && "status" in error) {
    return (error as { status: number }).status === 402;
  }
  if (error && typeof error === "object" && "response" in error) {
    const axiosError = error as { response?: { status?: number } };
    return axiosError.response?.status === 402;
  }
  return false;
}

/**
 * Check if the current API response indicates Gatekeeper is unavailable
 * @param error - Error object from API call
 * @returns True if error is 503 Service Unavailable (Gatekeeper not installed)
 */
export function isGatekeeperUnavailable(error: unknown): boolean {
  if (error && typeof error === "object" && "status" in error) {
    return (error as { status: number }).status === 503;
  }
  if (error && typeof error === "object" && "response" in error) {
    const axiosError = error as { response?: { status?: number } };
    return axiosError.response?.status === 503;
  }
  return false;
}

/**
 * Update a constraint's enforcement action
 * @param kind - Constraint kind
 * @param name - Constraint name
 * @param enforcementAction - New enforcement action (deny, warn, dryrun)
 * @returns Updated constraint
 */
export async function updateConstraintEnforcement(
  kind: string,
  name: string,
  enforcementAction: EnforcementAction
): Promise<Constraint> {
  const response = await apiClient.patch<Constraint>(
    `/v1/compliance/constraints/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/enforcement`,
    { enforcementAction }
  );
  return response.data;
}

/**
 * Create a new constraint from a ConstraintTemplate
 * @param request - Constraint creation request
 * @returns Created constraint
 */
export async function createConstraint(
  request: CreateConstraintRequest
): Promise<Constraint> {
  const response = await apiClient.post<Constraint>(
    "/v1/compliance/constraints",
    request
  );
  return response.data;
}

/**
 * Check if an error indicates a constraint already exists
 * @param error - Error object from API call
 * @returns True if error is 409 Conflict (already exists)
 */
export function isAlreadyExists(error: unknown): boolean {
  if (error && typeof error === "object" && "status" in error) {
    return (error as { status: number }).status === 409;
  }
  if (error && typeof error === "object" && "response" in error) {
    const axiosError = error as { response?: { status?: number } };
    return axiosError.response?.status === 409;
  }
  return false;
}

/**
 * Get violation history count with optional filters
 */
export async function getViolationHistoryCount(
  params?: ViolationHistoryCountParams
): Promise<ViolationHistoryCountResponse> {
  const queryParams = new URLSearchParams();

  if (params?.since) queryParams.append("since", params.since);
  if (params?.until) queryParams.append("until", params.until);
  if (params?.enforcement) queryParams.append("enforcement", params.enforcement);
  if (params?.constraint) queryParams.append("constraint", params.constraint);
  if (params?.resource) queryParams.append("resource", params.resource);

  const queryString = queryParams.toString();
  const url = queryString
    ? `/v1/compliance/violations/history/count?${queryString}`
    : "/v1/compliance/violations/history/count";

  const response = await apiClient.get<ViolationHistoryCountResponse>(url);
  return response.data;
}

/**
 * Export violation history as CSV (returns blob for download)
 */
export async function exportViolationHistory(
  params: ViolationHistoryExportParams
): Promise<{ blob: Blob; filename: string }> {
  const queryParams = new URLSearchParams();
  queryParams.append("since", params.since);

  if (params.enforcement) queryParams.append("enforcement", params.enforcement);
  if (params.constraint) queryParams.append("constraint", params.constraint);
  if (params.resource) queryParams.append("resource", params.resource);

  const response = await apiClient.get(
    `/v1/compliance/violations/history/export?${queryParams.toString()}`,
    { responseType: "blob" }
  );

  // Extract filename from Content-Disposition header if available
  const disposition = response.headers?.["content-disposition"] as string | undefined;
  let filename = `violations_${params.since.split("T")[0]}_${new Date().toISOString().split("T")[0]}.csv`;
  if (disposition) {
    const match = disposition.match(/filename="?([^";\n]+)"?/);
    if (match?.[1]) {
      filename = match[1];
    }
  }

  return { blob: response.data, filename };
}

/**
 * Download a blob as a file
 */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}
