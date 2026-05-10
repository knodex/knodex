// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Shield, X } from "@/lib/icons";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHead } from "@/components/ui/sortable-table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useConstraints,
  useConstraintTemplates,
  isEnterprise,
} from "@/hooks/useCompliance";
import { isEnterpriseRequired } from "@/api/compliance";
import { PageHeader } from "@/components/layout/PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { EnterpriseRequired } from "./EnterpriseRequired";
import { EnforcementBadge } from "./EnforcementBadge";
import { MatchRulesDisplay } from "./MatchRulesDisplay";
import { formatDistanceToNow } from "@/lib/date";
import { filterSelectClasses } from "@/components/ui/filter-bar";
import { cn } from "@/lib/utils";
import type { EnforcementAction } from "@/types/compliance";

type SortField = "name" | "kind" | "enforcement" | "violations" | "createdAt";
type SortDir = "asc" | "desc";

export function ConstraintsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const kindFilter = searchParams.get("kind") || "";
  const enforcementFilter = (searchParams.get("enforcement") as EnforcementAction | "") || "";

  const updateFilters = useCallback((updates: { kind?: string; enforcement?: string }) => {
    setSearchParams((prev) => {
      const newParams = new URLSearchParams(prev);
      if (updates.kind !== undefined) {
        if (updates.kind) { newParams.set("kind", updates.kind); } else { newParams.delete("kind"); }
      }
      if (updates.enforcement !== undefined) {
        if (updates.enforcement) { newParams.set("enforcement", updates.enforcement); } else { newParams.delete("enforcement"); }
      }
      return newParams;
    });
    setPage(1);
  }, [setSearchParams]);

  const clearFilters = useCallback(() => {
    setSearchParams(new URLSearchParams());
    setPage(1);
  }, [setSearchParams]);

  const hasFilters = kindFilter || enforcementFilter;

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  const { data, isLoading, isError, error, refetch, isRefetching } =
    useConstraints({
      kind: kindFilter || undefined,
      enforcement: enforcementFilter || undefined,
      page,
      pageSize,
    });

  const { data: templatesData } = useConstraintTemplates({ pageSize: 100 });
  const constraintKinds = templatesData?.items.map((t) => t.kind) || [];

  if (!isEnterprise() || (isError && isEnterpriseRequired(error))) {
    return (
      <EnterpriseRequired
        feature="Constraints"
        description="View and manage OPA Gatekeeper Constraints with an Enterprise license."
      />
    );
  }

  const columns = [
    { header: "Name", width: "25%" },
    { header: "Kind", width: "15%" },
    { header: "Enforcement", width: "12%" },
    { header: "Violations", width: "10%" },
    { header: "Match Scope", width: "25%", hideOnMobile: true },
    { header: "Created", width: "13%", hideOnMobile: true },
  ];

  return (
    <section className="space-y-6">
      <PageHeader title="Constraints" breadcrumbs={[{ label: "Compliance", href: "/compliance" }, { label: "Constraints" }]} />

      {/* Filters */}
      <div className="flex items-center gap-2">
        <Select
          value={kindFilter || "__all__"}
          onValueChange={(v) => updateFilters({ kind: v === "__all__" ? "" : v })}
        >
          <SelectTrigger data-testid="kind-filter" className={cn(filterSelectClasses(!!kindFilter), "h-8 w-[150px] text-xs")}>
            <SelectValue placeholder="All kinds" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All kinds</SelectItem>
            {constraintKinds.map((kind) => (
              <SelectItem key={kind} value={kind}>{kind}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={enforcementFilter || "__all__"}
          onValueChange={(v) => updateFilters({ enforcement: v === "__all__" ? "" : v })}
        >
          <SelectTrigger data-testid="enforcement-filter" className={cn(filterSelectClasses(!!enforcementFilter), "h-8 w-[130px] text-xs")}>
            <SelectValue placeholder="All actions" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All actions</SelectItem>
            <SelectItem value="deny">Deny</SelectItem>
            <SelectItem value="warn">Warn</SelectItem>
            <SelectItem value="dryrun">Dryrun</SelectItem>
          </SelectContent>
        </Select>

        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters} className="h-8 px-2 text-xs">
            <X className="h-3.5 w-3.5 mr-1" />
            Clear
          </Button>
        )}
      </div>

      {isLoading && <TableLoadingSkeleton columns={columns} showDoubleLines />}

      {isError && (
        <ErrorState
          message="Failed to load constraints"
          details={error instanceof Error ? error.message : "Unknown error"}
          onRetry={() => refetch()}
          isRetrying={isRefetching}
        />
      )}

      {!isLoading && !isError && data?.items.length === 0 && (
        <EmptyState
          icon={<Shield className="h-8 w-8 text-muted-foreground" />}
          title={hasFilters ? "No Matching Constraints" : "No Constraints"}
          description={
            hasFilters
              ? "No constraints match the selected filters."
              : "No OPA Gatekeeper constraints have been created in the cluster."
          }
          action={hasFilters ? <Button variant="outline" onClick={clearFilters}>Clear Filters</Button> : undefined}
        />
      )}

      {!isLoading && !isError && data && data.items.length > 0 && (
        <>
          <div className="rounded-lg border border-border overflow-hidden">
            <Table className="table-fixed">
              <TableHeader>
                <TableRow>
                  <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[25%]">Name</SortableHead>
                  <SortableHead field="kind" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[15%]">Kind</SortableHead>
                  <SortableHead field="enforcement" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Enforcement</SortableHead>
                  <SortableHead field="violations" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[10%]">Violations</SortableHead>
                  <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[25%] hidden md:table-cell">Match Scope</SortableHead>
                  <SortableHead field="createdAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[13%] hidden md:table-cell">Created</SortableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((constraint) => (
                  <TableRow
                    key={`${constraint.kind}-${constraint.name}`}
                    className="cursor-pointer hover:bg-muted/50"
                  >
                    <TableCell>
                      <Link
                        to={`/compliance/constraints/${constraint.kind}/${constraint.name}`}
                        className="font-medium hover:underline"
                      >
                        {constraint.name}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Link
                        to={`/compliance/constraints?kind=${constraint.kind}`}
                        className="text-primary hover:underline"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {constraint.kind}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <EnforcementBadge action={constraint.enforcementAction} />
                    </TableCell>
                    <TableCell>
                      {constraint.violationCount > 0 ? (
                        <Badge variant="destructive" className="font-mono">
                          {constraint.violationCount}
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-green-600 border-green-200 bg-green-50 dark:text-green-400 dark:border-green-900 dark:bg-green-950/30">
                          0
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="hidden md:table-cell">
                      <MatchRulesDisplay match={constraint.match} variant="compact" />
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-xs text-muted-foreground">
                      {formatDistanceToNow(constraint.createdAt)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <CompliancePagination
            page={page}
            pageSize={pageSize}
            totalCount={data.total}
            onPageChange={setPage}
            onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
          />
        </>
      )}
    </section>
  );
}

export default ConstraintsPage;
