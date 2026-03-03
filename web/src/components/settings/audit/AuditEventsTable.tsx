import { useMemo } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight, FileSearch } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ResultBadge } from "./ResultBadge";
import type { AuditEvent, AuditSortField } from "@/types/audit";

interface AuditEventsTableProps {
  events: AuditEvent[];
  total: number;
  page: number;
  pageSize: number;
  sortBy?: AuditSortField;
  sortOrder?: "asc" | "desc";
  isLoading: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  onSortChange: (sortBy: AuditSortField | undefined, sortOrder: "asc" | "desc" | undefined) => void;
  onRowClick: (event: AuditEvent) => void;
}

const PAGE_SIZE_OPTIONS = [25, 50, 100];

/**
 * Format a timestamp for display.
 * NOTE: Relative times ("5m ago") are computed at render time and won't auto-update.
 * They refresh when React Query refetches (staleTime: 30s) or on window refocus.
 */
function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  if (isNaN(date.getTime())) return iso || "\u2014";
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffMins < 1440) return `${Math.floor(diffMins / 60)}h ago`;

  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

const COLUMNS: { label: string; field: AuditSortField; width: string }[] = [
  { label: "Time", field: "timestamp", width: "14%" },
  { label: "User", field: "userEmail", width: "18%" },
  { label: "Action", field: "action", width: "10%" },
  { label: "Resource", field: "resource", width: "12%" },
  { label: "Name", field: "name", width: "16%" },
  { label: "Project", field: "project", width: "14%" },
  { label: "Result", field: "result", width: "10%" },
];

/** Sort indicator icon for column headers */
function SortIcon({ field, sortBy, sortOrder }: { field: AuditSortField; sortBy?: AuditSortField; sortOrder?: "asc" | "desc" }) {
  if (sortBy !== field) return <ArrowUpDown className="h-3 w-3 ml-1 opacity-30" />;
  return sortOrder === "asc"
    ? <ArrowUp className="h-3 w-3 ml-1" />
    : <ArrowDown className="h-3 w-3 ml-1" />;
}

export function AuditEventsTable({
  events,
  total,
  page,
  pageSize,
  sortBy,
  sortOrder,
  isLoading,
  onPageChange,
  onPageSizeChange,
  onSortChange,
  onRowClick,
}: AuditEventsTableProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  // Client-side sort of the current page (backend returns newest-first;
  // full server-side sort to be added in a future story).
  const sortedEvents = useMemo(() => {
    if (!sortBy) return events;
    return [...events].sort((a, b) => {
      const aVal = String(a[sortBy] ?? "");
      const bVal = String(b[sortBy] ?? "");
      const cmp = sortBy === "timestamp"
        ? new Date(aVal).getTime() - new Date(bVal).getTime()
        : aVal.localeCompare(bVal);
      return sortOrder === "desc" ? -cmp : cmp;
    });
  }, [events, sortBy, sortOrder]);

  // Loading skeleton
  if (isLoading && events.length === 0) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex gap-4 p-3">
            <Skeleton className="h-4 w-[120px]" />
            <Skeleton className="h-4 w-[140px]" />
            <Skeleton className="h-4 w-[80px]" />
            <Skeleton className="h-4 w-[80px]" />
            <Skeleton className="h-4 w-[100px]" />
            <Skeleton className="h-4 w-[80px]" />
            <Skeleton className="h-4 w-[60px]" />
          </div>
        ))}
      </div>
    );
  }

  // Empty state
  if (!isLoading && events.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        <FileSearch className="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p className="text-sm font-medium">No audit events found</p>
        <p className="text-xs mt-2">
          Try adjusting your filters or check back later.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              {COLUMNS.map((col) => (
                <TableHead
                  key={col.field}
                  style={{ width: col.width }}
                  className="cursor-pointer select-none hover:bg-muted/50"
                  onClick={() => {
                    if (sortBy === col.field) {
                      if (sortOrder === "asc") {
                        onSortChange(col.field, "desc");
                      } else {
                        // desc → unsorted (3-state cycle)
                        onSortChange(undefined, undefined);
                      }
                    } else {
                      onSortChange(col.field, "asc");
                    }
                  }}
                >
                  <span className="inline-flex items-center">
                    {col.label}
                    <SortIcon field={col.field} sortBy={sortBy} sortOrder={sortOrder} />
                  </span>
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedEvents.map((event) => (
              <TableRow
                key={event.id}
                className="cursor-pointer hover:bg-muted/50"
                tabIndex={0}
                role="button"
                onClick={() => onRowClick(event)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onRowClick(event);
                  }
                }}
              >
                <TableCell className="text-xs text-muted-foreground">
                  {formatTimestamp(event.timestamp)}
                </TableCell>
                <TableCell className="text-sm truncate max-w-[200px]">
                  {event.userEmail || event.userId}
                </TableCell>
                <TableCell className="text-sm">{event.action}</TableCell>
                <TableCell className="text-sm">{event.resource}</TableCell>
                <TableCell className="text-sm truncate max-w-[180px]">
                  {event.name}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {event.project || "\u2014"}
                </TableCell>
                <TableCell>
                  <ResultBadge result={event.result} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between mt-4">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>
            {total} event{total !== 1 ? "s" : ""}
          </span>
          <span>&middot;</span>
          <span>Page {page} of {totalPages}</span>
          <span>&middot;</span>
          <Select
            value={pageSize.toString()}
            onValueChange={(v) => onPageSizeChange(Number(v))}
          >
            <SelectTrigger className="h-8 w-[80px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZE_OPTIONS.map((size) => (
                <SelectItem key={size} value={size.toString()}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span>per page</span>
        </div>

        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            aria-label="Previous page"
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            aria-label="Next page"
          >
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
