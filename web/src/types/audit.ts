/**
 * Audit trail TypeScript types.
 * Matches the backend services.AuditEvent JSON schema exactly.
 */

/** Single audit event */
export interface AuditEvent {
  id: string;
  timestamp: string; // ISO 8601 / RFC 3339
  userId: string;
  userEmail: string;
  sourceIP: string;
  action: string;
  resource: string;
  name: string;
  project?: string;
  namespace?: string;
  requestId: string;
  result: string; // "success" | "denied" | "error"
  details?: Record<string, unknown>;
}

/** Paginated audit event list response */
export interface AuditEventList {
  events: AuditEvent[];
  total: number;
  page: number;
  pageSize: number;
}

/** Sortable column names for audit events table */
export type AuditSortField = "timestamp" | "userEmail" | "action" | "resource" | "name" | "project" | "result";

/** Query parameters for listing audit events (API-level filters only) */
export interface AuditEventFilter {
  userId?: string;
  action?: string;
  resource?: string;
  project?: string;
  result?: string;
  from?: string; // RFC 3339
  to?: string;   // RFC 3339
  page?: number;
  pageSize?: number;
}

/** User activity count for top-users aggregation */
export interface UserActivity {
  userId: string;
  count: number;
}

/** Aggregate audit statistics */
export interface AuditStats {
  totalEvents: number;
  eventsToday: number;
  topUsers: UserActivity[];
  deniedAttempts: number;
  byActionToday: Record<string, number>;
  byResultToday: Record<string, number>;
}

/** Audit configuration */
export interface AuditConfig {
  enabled: boolean;
  retentionDays: number;
  maxStreamLength: number;
  excludeActions: string[];
  excludeResources: string[];
}
