// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { AlertTriangle, Filter, X, CheckCircle, Search, Radio, Wifi, WifiOff, FileDown } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useViolations, useConstraints } from "@/hooks/useCompliance";
import { useViolationWebSocket } from "@/hooks/useViolationWebSocket";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { PageHeader } from "./PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { EnforcementBadge } from "./EnforcementBadge";
import { ExportViolationHistoryDialog } from "./ExportViolationHistoryDialog";

/**
 * Violations list page with filtering
 * AC-VIO-01: Table of violations with resource, constraint, message
 * AC-VIO-02: Filter by constraint (dropdown or search)
 * AC-VIO-03: Filter by resource kind
 * AC-VIO-04: Link to violating resource details (if available)
 * AC-VIO-05: Enforcement action badge (deny=red, warn=yellow, dryrun=blue)
 * AC-VIO-06: Zero state shows success message when no violations
 * AC-VIO-07: Filter state in URL query params
 */
export function ViolationsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);

  // Get filters from URL
  const constraintFilter = searchParams.get("constraint") || "";
  const resourceFilter = searchParams.get("resource") || "";

  // Update URL when filters change
  const updateFilters = useCallback(
    (updates: { constraint?: string; resource?: string }) => {
      setSearchParams((prev) => {
        const newParams = new URLSearchParams(prev);

        if (updates.constraint !== undefined) {
          if (updates.constraint) {
            newParams.set("constraint", updates.constraint);
          } else {
            newParams.delete("constraint");
          }
        }

        if (updates.resource !== undefined) {
          if (updates.resource) {
            newParams.set("resource", updates.resource);
          } else {
            newParams.delete("resource");
          }
        }

        return newParams;
      });
      setPage(1);
    },
    [setSearchParams]
  );

  const clearFilters = useCallback(() => {
    setSearchParams(new URLSearchParams());
    setPage(1);
  }, [setSearchParams]);

  const hasFilters = constraintFilter || resourceFilter;

  // WebSocket connection for real-time updates
  const {
    status: wsStatus,
    hasRecentUpdate,
    lastUpdateTime,
  } = useViolationWebSocket({
    debug: import.meta.env.DEV,
  });

  // Fetch violations with filters
  const { data, isLoading, isError, error, refetch, isRefetching } =
    useViolations({
      constraint: constraintFilter || undefined,
      resource: resourceFilter || undefined,
      page,
      pageSize,
    });

  // Fetch constraints for filter dropdown
  const { data: constraintsData } = useConstraints({ pageSize: 100 });
  const constraintOptions = useMemo(
    () =>
      constraintsData?.items
        .filter((c) => c.violationCount > 0)
        .map((c) => ({
          value: `${c.kind}/${c.name}`,
          label: c.name,
          kind: c.kind,
          count: c.violationCount,
        })) || [],
    [constraintsData]
  );

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize);
    setPage(1);
  }, []);

  const handleResourceSearchChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      updateFilters({ resource: e.target.value });
    },
    [updateFilters]
  );

  const columns = [
    { header: "Resource", width: "25%" },
    { header: "Namespace", width: "12%" },
    { header: "Constraint", width: "18%" },
    { header: "Enforcement", width: "10%" },
    { header: "Message", width: "35%", hideOnMobile: true },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Violations"
        subtitle="Policy violations detected by OPA Gatekeeper audit controller"
        breadcrumbs={[
          { label: "Compliance", href: "/compliance" },
          { label: "Violations" },
        ]}
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => setExportDialogOpen(true)}
          >
            <FileDown className="mr-2 h-4 w-4" />
            Export History
          </Button>
        }
      />

      <ExportViolationHistoryDialog
        open={exportDialogOpen}
        onOpenChange={setExportDialogOpen}
        constraintFilter={constraintFilter}
        resourceFilter={resourceFilter}
      />

      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="flex items-center gap-2 text-lg">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              Violations
              {data && (
                <span className="text-sm font-normal text-muted-foreground">
                  ({data.total})
                </span>
              )}
              {/* Real-time connection indicator */}
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="flex items-center gap-1.5 ml-2">
                      {wsStatus === "connected" ? (
                        <>
                          <Wifi className="h-4 w-4 text-green-500" />
                          {hasRecentUpdate && (
                            <span className="relative flex h-2 w-2">
                              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
                              <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
                            </span>
                          )}
                        </>
                      ) : wsStatus === "connecting" ? (
                        <Radio className="h-4 w-4 text-yellow-500 animate-pulse" />
                      ) : (
                        <WifiOff className="h-4 w-4 text-muted-foreground" />
                      )}
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>
                      {wsStatus === "connected"
                        ? hasRecentUpdate
                          ? `Live updates active - Updated ${lastUpdateTime?.toLocaleTimeString() || "just now"}`
                          : "Live updates active"
                        : wsStatus === "connecting"
                        ? "Connecting to real-time updates..."
                        : "Real-time updates disconnected"}
                    </p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </CardTitle>

            {/* Filters */}
            <div className="flex flex-wrap items-center gap-2">
              <Filter className="h-4 w-4 text-muted-foreground" />

              {/* Constraint filter */}
              <Select
                value={constraintFilter}
                onValueChange={(value) =>
                  updateFilters({ constraint: value === "all" ? "" : value })
                }
              >
                <SelectTrigger className="w-[180px] h-8">
                  <SelectValue placeholder="All constraints" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All constraints</SelectItem>
                  {constraintOptions.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      <span className="flex items-center gap-2">
                        <span className="truncate">{opt.label}</span>
                        <span className="text-xs text-muted-foreground">
                          ({opt.count})
                        </span>
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Resource search */}
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                <Input
                  placeholder="Filter by resource..."
                  value={resourceFilter}
                  onChange={handleResourceSearchChange}
                  className="w-[180px] h-8 pl-8"
                />
              </div>

              {hasFilters && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearFilters}
                  className="h-8 px-2"
                >
                  <X className="h-4 w-4 mr-1" />
                  Clear
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading && <TableLoadingSkeleton columns={columns} />}

          {isError && (
            <ErrorState
              message="Failed to load violations"
              details={error instanceof Error ? error.message : "Unknown error"}
              onRetry={() => refetch()}
              isRetrying={isRefetching}
            />
          )}

          {/* Success state when no violations */}
          {!isLoading &&
            !isError &&
            data?.items.length === 0 &&
            !hasFilters && (
              <EmptyState
                icon={
                  <CheckCircle className="h-8 w-8 text-green-500 dark:text-green-400" />
                }
                title="No Violations"
                description="All resources are compliant with your OPA Gatekeeper policies."
                variant="success"
              />
            )}

          {/* Filter empty state */}
          {!isLoading &&
            !isError &&
            data?.items.length === 0 &&
            hasFilters && (
              <EmptyState
                icon={
                  <AlertTriangle className="h-8 w-8 text-muted-foreground" />
                }
                title="No Matching Violations"
                description="No violations match the selected filters. Try adjusting your filters."
                action={
                  <Button variant="outline" onClick={clearFilters}>
                    Clear Filters
                  </Button>
                }
              />
            )}

          {!isLoading && !isError && data && data.items.length > 0 && (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    {columns.map((col, index) => (
                      <TableHead
                        key={index}
                        className={
                          col.hideOnMobile ? "hidden md:table-cell" : ""
                        }
                        style={{ width: col.width }}
                      >
                        {col.header}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((violation, index) => (
                    <TableRow key={index}>
                      <TableCell>
                        <div className="flex flex-col">
                          <span className="font-medium">
                            {violation.resource.kind}/{violation.resource.name}
                          </span>
                          {violation.resource.apiGroup && (
                            <span className="text-xs text-muted-foreground">
                              {violation.resource.apiGroup}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {violation.resource.namespace || (
                          <span className="italic text-xs">cluster-scoped</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Link
                          to={`/compliance/constraints/${violation.constraintKind}/${violation.constraintName}`}
                          className="text-primary hover:underline"
                        >
                          <div className="flex flex-col">
                            <span className="font-medium">
                              {violation.constraintName}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              {violation.constraintKind}
                            </span>
                          </div>
                        </Link>
                      </TableCell>
                      <TableCell>
                        <EnforcementBadge action={violation.enforcementAction} />
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <span className="text-sm line-clamp-2">
                          {violation.message}
                        </span>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <CompliancePagination
                page={page}
                pageSize={pageSize}
                totalCount={data.total}
                onPageChange={handlePageChange}
                onPageSizeChange={handlePageSizeChange}
              />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default ViolationsPage;
