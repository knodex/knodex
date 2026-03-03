import apiClient from "./client";
import type {
  AuditEventList,
  AuditEvent,
  AuditEventFilter,
  AuditStats,
  AuditConfig,
} from "@/types/audit";

/**
 * List audit events with optional filtering and pagination
 */
export async function getAuditEvents(
  params?: AuditEventFilter
): Promise<AuditEventList> {
  const queryParams = new URLSearchParams();

  if (params?.userId) queryParams.append("userId", params.userId);
  if (params?.action) queryParams.append("action", params.action);
  if (params?.resource) queryParams.append("resource", params.resource);
  if (params?.project) queryParams.append("project", params.project);
  if (params?.result) queryParams.append("result", params.result);
  if (params?.from) queryParams.append("from", params.from);
  if (params?.to) queryParams.append("to", params.to);
  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.pageSize) queryParams.append("pageSize", params.pageSize.toString());

  const queryString = queryParams.toString();
  const url = queryString
    ? `/v1/settings/audit/events?${queryString}`
    : "/v1/settings/audit/events";

  const response = await apiClient.get<AuditEventList>(url);
  return response.data;
}

/**
 * Get a single audit event by ID
 */
export async function getAuditEvent(id: string): Promise<AuditEvent> {
  const response = await apiClient.get<AuditEvent>(
    `/v1/settings/audit/events/${encodeURIComponent(id)}`
  );
  return response.data;
}

/**
 * Get aggregate audit statistics
 */
export async function getAuditStats(): Promise<AuditStats> {
  const response = await apiClient.get<AuditStats>("/v1/settings/audit/stats");
  return response.data;
}

/**
 * Get the current audit configuration
 */
export async function getAuditConfig(): Promise<AuditConfig> {
  const response = await apiClient.get<AuditConfig>("/v1/settings/audit/config");
  return response.data;
}

/**
 * Update audit configuration
 */
export async function updateAuditConfig(
  config: AuditConfig
): Promise<AuditConfig> {
  const response = await apiClient.put<AuditConfig>(
    "/v1/settings/audit/config",
    config
  );
  return response.data;
}

/**
 * Fetch all audit events matching the given filter, looping through pages.
 * Used for CSV export where we need the complete dataset, not just one page.
 * Caps at 10,000 events to avoid excessive memory usage in the browser.
 */
export async function fetchAllAuditEvents(
  params?: Omit<AuditEventFilter, "page" | "pageSize">
): Promise<AuditEvent[]> {
  const maxExportEvents = 10000;
  const pageSize = 200; // handler max
  const allEvents: AuditEvent[] = [];
  let page = 1;

  while (allEvents.length < maxExportEvents) {
    const result = await getAuditEvents({ ...params, page, pageSize });
    allEvents.push(...result.events);

    if (allEvents.length >= result.total || result.events.length < pageSize) {
      break;
    }
    page++;
  }

  return allEvents;
}
