// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { AlertTriangle, X, CheckCircle, Search, FileDown } from "@/lib/icons";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHead } from "@/components/ui/sortable-table";
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
import { useCurrentProject } from "@/hooks/useAuth";
import { useProjects } from "@/hooks/useProjects";
import { filterByProjectNamespaces } from "@/lib/project-utils";
import { PageHeader } from "@/components/layout/PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { EnforcementBadge } from "./EnforcementBadge";
import { ExportViolationHistoryDialog } from "./ExportViolationHistoryDialog";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
  filterSelectClasses,
} from "@/components/ui/filter-bar";
import { cn } from "@/lib/utils";

type SortField = "resource" | "namespace" | "constraint" | "enforcement";
type SortDir = "asc" | "desc";

export function ViolationsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [sortField, setSortField] = useState<SortField>("resource");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const constraintFilter = searchParams.get("constraint") || "";
  const resourceFilter = searchParams.get("resource") || "";

  const updateFilters = useCallback(
    (updates: { constraint?: string; resource?: string }) => {
      setSearchParams((prev) => {
        const newParams = new URLSearchParams(prev);
        if (updates.constraint !== undefined) {
          if (updates.constraint) { newParams.set("constraint", updates.constraint); } else { newParams.delete("constraint"); }
        }
        if (updates.resource !== undefined) {
          if (updates.resource) { newParams.set("resource", updates.resource); } else { newParams.delete("resource"); }
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

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  // Global project selector integration
  const currentProject = useCurrentProject();
  const { data: projectsData } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData?.items]);

  // When a project is selected, fetch all violations (no server pagination) so client-side
  // filtering doesn't hide items that would appear on other pages.
  const effectivePageSize = currentProject ? 1000 : pageSize;
  const effectivePage = currentProject ? 1 : page;

  const { data, isLoading, isError, error, refetch, isRefetching } =
    useViolations({
      constraint: constraintFilter || undefined,
      resource: resourceFilter || undefined,
      page: effectivePage,
      pageSize: effectivePageSize,
    });

  // Filter violations by selected project's namespaces (client-side, same pattern as InstancesPage)
  const projectFilteredData = useMemo(() => {
    if (!data || !currentProject || !projectsData) return data;
    const selectedProject = projects.find((p) => p.name === currentProject);
    if (!selectedProject) return data;
    const mapped = data.items.map((v) => ({ ...v, namespace: v.resource.namespace ?? "" }));
    const filtered = filterByProjectNamespaces(mapped, selectedProject);
    return { ...data, items: filtered, total: filtered.length };
  }, [data, currentProject, projects, projectsData]);

  const { data: constraintsData } = useConstraints({ pageSize: 100 });
  const constraintOptions = useMemo(
    () =>
      constraintsData?.items
        .filter((c) => c.violationCount > 0)
        .map((c) => ({
          value: `${c.kind}/${c.name}`,
          label: c.name,
          count: c.violationCount,
        })) || [],
    [constraintsData]
  );

  const columns = [
    { header: "Resource", width: "25%" },
    { header: "Namespace", width: "12%" },
    { header: "Constraint", width: "18%" },
    { header: "Enforcement", width: "10%" },
    { header: "Message", width: "35%", hideOnMobile: true },
  ];

  return (
    <section className="space-y-6">
      <PageHeader title="Violations" breadcrumbs={[{ label: "Compliance", href: "/compliance" }, { label: "Violations" }]} />

      <ExportViolationHistoryDialog
        open={exportDialogOpen}
        onOpenChange={setExportDialogOpen}
        constraintFilter={constraintFilter}
        resourceFilter={resourceFilter}
      />

      {/* Filters + Export */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 min-w-[180px] max-w-[240px]">
          <Search className={filterSearchIconClasses} />
          <Input
            placeholder="Filter by resource..."
            value={resourceFilter}
            onChange={(e) => updateFilters({ resource: e.target.value })}
            className={cn(filterSearchClasses, "h-8 text-xs")}
            aria-label="Filter by resource"
          />
          {resourceFilter && (
            <button
              type="button"
              onClick={() => updateFilters({ resource: "" })}
              className={filterClearButtonClasses}
              aria-label="Clear resource filter"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        <Select
          value={constraintFilter || "__all__"}
          onValueChange={(v) => updateFilters({ constraint: v === "__all__" ? "" : v })}
        >
          <SelectTrigger className={cn(filterSelectClasses(!!constraintFilter), "h-8 w-[170px] text-xs")}>
            <SelectValue placeholder="All constraints" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All constraints</SelectItem>
            {constraintOptions.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label} ({opt.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters} className="h-8 px-2 text-xs">
            <X className="h-3.5 w-3.5 mr-1" />
            Clear
          </Button>
        )}

        <div className="flex-1" />

        <button
          onClick={() => setExportDialogOpen(true)}
          className="inline-flex items-center h-8 gap-1.5 rounded-[var(--radius-token-md)] px-2.5 text-xs font-medium border border-[var(--border-default)] text-muted-foreground hover:border-[var(--border-hover)] hover:text-foreground transition-all duration-150 shrink-0"
        >
          <FileDown className="h-3 w-3" />
          Export
        </button>
      </div>

      {isLoading && <TableLoadingSkeleton columns={columns} />}

      {isError && (
        <ErrorState
          message="Failed to load violations"
          details={error instanceof Error ? error.message : "Unknown error"}
          onRetry={() => refetch()}
          isRetrying={isRefetching}
        />
      )}

      {!isLoading && !isError && projectFilteredData?.items.length === 0 && !hasFilters && !currentProject && (
        <EmptyState
          icon={<CheckCircle className="h-8 w-8 text-green-500 dark:text-green-400" />}
          title="No Violations"
          description="All resources are compliant with your OPA Gatekeeper policies."
          variant="success"
        />
      )}

      {!isLoading && !isError && projectFilteredData?.items.length === 0 && (hasFilters || currentProject) && (
        <EmptyState
          icon={<AlertTriangle className="h-8 w-8 text-muted-foreground" />}
          title="No Matching Violations"
          description={currentProject && !hasFilters
            ? `No violations found in project "${currentProject}". Try selecting a different project.`
            : "No violations match the selected filters."}
          action={hasFilters ? <Button variant="outline" onClick={clearFilters}>Clear Filters</Button> : undefined}
        />
      )}

      {!isLoading && !isError && projectFilteredData && projectFilteredData.items.length > 0 && (
        <>
          <div className="rounded-lg border border-border overflow-hidden">
            <Table className="table-fixed">
              <TableHeader>
                <TableRow>
                  <SortableHead field="resource" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[25%]">Resource</SortableHead>
                  <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Namespace</SortableHead>
                  <SortableHead field="constraint" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[18%]">Constraint</SortableHead>
                  <SortableHead field="enforcement" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[10%]">Enforcement</SortableHead>
                  <SortableHead field="resource" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[35%] hidden md:table-cell">Message</SortableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {projectFilteredData.items.map((violation) => (
                  <TableRow key={`${violation.constraintKind}/${violation.constraintName}/${violation.resource.namespace ?? ""}/${violation.resource.kind}/${violation.resource.name}`}>
                    <TableCell>
                      <div className="min-w-0">
                        <p className="font-medium truncate">{violation.resource.kind}/{violation.resource.name}</p>
                        {violation.resource.apiGroup && (
                          <p className="text-xs text-muted-foreground truncate">{violation.resource.apiGroup}</p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground truncate">
                      {violation.resource.namespace || <span className="italic text-xs">cluster</span>}
                    </TableCell>
                    <TableCell>
                      <Link
                        to={`/compliance/constraints/${violation.constraintKind}/${violation.constraintName}`}
                        className="text-primary hover:underline"
                      >
                        <div className="min-w-0">
                          <p className="font-medium truncate">{violation.constraintName}</p>
                          <p className="text-xs text-muted-foreground truncate">{violation.constraintKind}</p>
                        </div>
                      </Link>
                    </TableCell>
                    <TableCell>
                      <EnforcementBadge action={violation.enforcementAction} />
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-sm text-muted-foreground truncate">
                      {violation.message}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <CompliancePagination
            page={page}
            pageSize={pageSize}
            totalCount={projectFilteredData.total}
            onPageChange={setPage}
            onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
          />
        </>
      )}
    </section>
  );
}

export default ViolationsPage;
