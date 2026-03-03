import { useState, useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { Download, ScrollText, ShieldAlert } from "lucide-react";
import { AxiosError } from "axios";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { isEnterprise } from "@/hooks/useCompliance";
import { EnterpriseRequired } from "@/components/compliance";
import { useAuditEvents, useAuditStats } from "@/hooks/useAudit";
import { fetchAllAuditEvents } from "@/api/audit";
import { AuditStats } from "@/components/settings/audit/AuditStats";
import { AuditFilters } from "@/components/settings/audit/AuditFilters";
import { AuditEventsTable } from "@/components/settings/audit/AuditEventsTable";
import { AuditEventDetail } from "@/components/settings/audit/AuditEventDetail";
import type { AuditEvent, AuditEventFilter, AuditSortField } from "@/types/audit";

/** Escape a CSV field value, quoting if it contains commas, quotes, or newlines. */
function csvEscape(value: string): string {
  if (value.includes(",") || value.includes('"') || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

/** Convert audit events to a CSV string. */
function auditEventsToCSV(events: AuditEvent[]): string {
  const headers = [
    "Timestamp", "User Email", "User ID", "Source IP",
    "Action", "Resource", "Name", "Project", "Namespace",
    "Result", "Request ID", "Details",
  ];

  const rows = events.map((e) => [
    e.timestamp,
    e.userEmail || "",
    e.userId || "",
    e.sourceIP || "",
    e.action || "",
    e.resource || "",
    e.name || "",
    e.project || "",
    e.namespace || "",
    e.result || "",
    e.requestId || "",
    e.details ? JSON.stringify(e.details) : "",
  ]);

  return [
    headers.map(csvEscape).join(","),
    ...rows.map((row) => row.map(csvEscape).join(",")),
  ].join("\n");
}

/** Trigger a browser file download from a string. */
function downloadCSV(csv: string, filename: string) {
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

/** Validate that a string parses to a valid date */
function isValidDate(s: string): boolean {
  return !isNaN(new Date(s).getTime());
}

/**
 * Top-level Audit page at /audit — Browse and filter audit events.
 *
 * Enterprise-only feature gated by __ENTERPRISE__ build-time constant.
 * Authorization handled at API level; 403 displayed as Access Denied.
 *
 * URL state is centralized here: this component owns all search param reads/writes.
 * AuditFilters receives values as props and communicates changes via callbacks.
 */
export default function AuditPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [exporting, setExporting] = useState(false);

  // Read filter values from URL params (single source of truth)
  const userId = searchParams.get("userId") || "";
  const action = searchParams.get("action") || "";
  const resource = searchParams.get("resource") || "";
  const project = searchParams.get("project") || "";
  const result = searchParams.get("result") || "";
  const fromParam = searchParams.get("from") || "";
  const toParam = searchParams.get("to") || "";
  const from = fromParam && isValidDate(fromParam) ? fromParam : "";
  const to = toParam && isValidDate(toParam) ? toParam : "";

  // Build API filter from validated URL params
  const filters: AuditEventFilter = useMemo(() => ({
    userId: userId || undefined,
    action: action || undefined,
    resource: resource || undefined,
    project: project || undefined,
    result: result || undefined,
    from: from || undefined,
    to: to || undefined,
    page: Number(searchParams.get("page")) || 1,
    pageSize: Number(searchParams.get("pageSize")) || 50,
  }), [userId, action, resource, project, result, from, to, searchParams]);

  // Sort state (persisted in URL but applied client-side)
  const sortBy = (searchParams.get("sortBy") as AuditSortField) || undefined;
  const sortOrder = (searchParams.get("sortOrder") as "asc" | "desc") || undefined;

  // Hooks have `enabled: isEnterprise()` so they won't fire API calls in OSS builds
  const { data, isLoading, error } = useAuditEvents(filters);
  const { data: stats, isLoading: statsLoading, error: statsError } = useAuditStats();

  const handleFilterChange = useCallback(
    (key: string, value: string) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        if (value) {
          next.set(key, value);
        } else {
          next.delete(key);
        }
        next.delete("page"); // reset pagination on filter change
        return next;
      });
    },
    [setSearchParams]
  );

  const handleClearFilters = useCallback(() => {
    setSearchParams((prev) => {
      const next = new URLSearchParams();
      // Preserve sort and page size preferences
      for (const key of ["sortBy", "sortOrder", "pageSize"]) {
        const val = prev.get(key);
        if (val) next.set(key, val);
      }
      return next;
    });
  }, [setSearchParams]);

  const handlePageChange = useCallback(
    (page: number) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set("page", page.toString());
        return next;
      });
    },
    [setSearchParams]
  );

  const handlePageSizeChange = useCallback(
    (pageSize: number) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set("pageSize", pageSize.toString());
        next.delete("page"); // reset to page 1
        return next;
      });
    },
    [setSearchParams]
  );

  const handleRowClick = useCallback((event: AuditEvent) => {
    setSelectedEvent(event);
    setDetailOpen(true);
  }, []);

  const handleSortChange = useCallback(
    (newSortBy: AuditSortField | undefined, newSortOrder: "asc" | "desc" | undefined) => {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        if (newSortBy && newSortOrder) {
          next.set("sortBy", newSortBy);
          next.set("sortOrder", newSortOrder);
        } else {
          next.delete("sortBy");
          next.delete("sortOrder");
        }
        next.delete("page"); // reset to page 1 on sort change
        return next;
      });
    },
    [setSearchParams]
  );

  const handleExportCSV = useCallback(async () => {
    setExporting(true);
    try {
      const events = await fetchAllAuditEvents({
        userId: filters.userId,
        action: filters.action,
        resource: filters.resource,
        project: filters.project,
        result: filters.result,
        from: filters.from,
        to: filters.to,
      });
      const csv = auditEventsToCSV(events);
      const date = new Date().toISOString().slice(0, 10);
      downloadCSV(csv, `audit-events-${date}.csv`);
    } finally {
      setExporting(false);
    }
  }, [filters]);

  // Enterprise gate (all hooks called unconditionally above)
  if (!isEnterprise()) {
    return (
      <EnterpriseRequired
        feature="Audit Trail"
        description="Monitor user actions and security events with a comprehensive audit trail. Track logins, permission changes, and resource modifications."
      />
    );
  }

  // Handle 403 Access Denied
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  if (is403Error) {
    return (
      <div className="container mx-auto py-6 px-4 sm:px-6 lg:px-8">
        <div className="mb-8">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ScrollText className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">Audit Trail</h2>
              <p className="text-muted-foreground">Browse audit events</p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to view audit events.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 px-4 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="mb-8 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <ScrollText className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">Audit Trail</h2>
            <p className="text-muted-foreground">
              Browse and filter audit events for user actions and security events
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={exporting || isLoading}
          onClick={handleExportCSV}
        >
          <Download className="h-4 w-4 mr-2" />
          {exporting ? "Exporting..." : "Export CSV"}
        </Button>
      </div>

      {/* Stats cards */}
      <div className="mb-6">
        <AuditStats stats={stats} isLoading={statsLoading} error={statsError} />
      </div>

      {/* Filters */}
      <Card className="mb-6">
        <CardContent className="pt-6">
          <AuditFilters
            userId={userId}
            action={action}
            resource={resource}
            project={project}
            result={result}
            from={from}
            to={to}
            onFilterChange={handleFilterChange}
            onClearFilters={handleClearFilters}
          />
        </CardContent>
      </Card>

      {/* Error state (non-403) */}
      {error && !is403Error && (
        <Card className="mb-6">
          <CardContent className="pt-6">
            <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load audit events:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Events table */}
      <Card>
        <CardContent className="pt-6">
          <AuditEventsTable
            events={data?.events ?? []}
            total={data?.total ?? 0}
            page={filters.page ?? 1}
            pageSize={filters.pageSize ?? 50}
            sortBy={sortBy}
            sortOrder={sortOrder}
            isLoading={isLoading}
            onPageChange={handlePageChange}
            onPageSizeChange={handlePageSizeChange}
            onSortChange={handleSortChange}
            onRowClick={handleRowClick}
          />
        </CardContent>
      </Card>

      {/* Event detail slide-over */}
      <AuditEventDetail
        event={selectedEvent}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />
    </div>
  );
}
