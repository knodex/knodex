import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Shield, ChevronRight, Filter, X } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
} from "@/hooks/useCompliance";
import { PageHeader } from "./PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { EnforcementBadge } from "./EnforcementBadge";
import { MatchRulesDisplay } from "./MatchRulesDisplay";
import { formatDistanceToNow } from "@/lib/date";
import type { EnforcementAction } from "@/types/compliance";

/**
 * Constraints list page with filtering
 * AC-CON-01: Table of constraints with name, kind, enforcement, violations
 * AC-CON-02: Filter by constraint kind (dropdown)
 * AC-CON-03: Filter by enforcement action
 * AC-CON-04: Badge showing violation count
 * AC-CON-05: Row click navigates to constraint detail
 * AC-CON-06: Filter state in URL query params
 */
export function ConstraintsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // Get filters from URL
  const kindFilter = searchParams.get("kind") || "";
  const enforcementFilter =
    (searchParams.get("enforcement") as EnforcementAction | "") || "";

  // Update URL when filters change
  const updateFilters = (updates: {
    kind?: string;
    enforcement?: string;
  }) => {
    const newParams = new URLSearchParams(searchParams);

    if (updates.kind !== undefined) {
      if (updates.kind) {
        newParams.set("kind", updates.kind);
      } else {
        newParams.delete("kind");
      }
    }

    if (updates.enforcement !== undefined) {
      if (updates.enforcement) {
        newParams.set("enforcement", updates.enforcement);
      } else {
        newParams.delete("enforcement");
      }
    }

    setSearchParams(newParams);
    setPage(1); // Reset to first page when filters change
  };

  const clearFilters = () => {
    setSearchParams(new URLSearchParams());
    setPage(1);
  };

  const hasFilters = kindFilter || enforcementFilter;

  // Fetch constraints with filters
  const { data, isLoading, isError, error, refetch, isRefetching } =
    useConstraints({
      kind: kindFilter || undefined,
      enforcement: enforcementFilter || undefined,
      page,
      pageSize,
    });

  // Fetch templates for kind filter dropdown
  const { data: templatesData } = useConstraintTemplates({ pageSize: 100 });
  const constraintKinds = templatesData?.items.map((t) => t.kind) || [];

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
    setPage(1);
  };

  const columns = [
    { header: "Name", width: "25%" },
    { header: "Kind", width: "15%" },
    { header: "Enforcement", width: "12%" },
    { header: "Violations", width: "10%" },
    { header: "Match Scope", width: "25%", hideOnMobile: true },
    { header: "Created", width: "13%", hideOnMobile: true },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Constraints"
        subtitle="Active OPA Gatekeeper policy constraints and their enforcement status"
        breadcrumbs={[
          { label: "Compliance", href: "/compliance" },
          { label: "Constraints" },
        ]}
      />

      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="flex items-center gap-2 text-lg">
              <Shield className="h-5 w-5 text-muted-foreground" />
              Constraints
              {data && (
                <span className="text-sm font-normal text-muted-foreground">
                  ({data.total})
                </span>
              )}
            </CardTitle>

            {/* Filters */}
            <div className="flex items-center gap-2">
              <Filter className="h-4 w-4 text-muted-foreground" />

              {/* Kind filter */}
              <Select
                value={kindFilter}
                onValueChange={(value) =>
                  updateFilters({ kind: value === "all" ? "" : value })
                }
              >
                <SelectTrigger className="w-[160px] h-8">
                  <SelectValue placeholder="All kinds" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All kinds</SelectItem>
                  {constraintKinds.map((kind) => (
                    <SelectItem key={kind} value={kind}>
                      {kind}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Enforcement filter */}
              <Select
                value={enforcementFilter}
                onValueChange={(value) =>
                  updateFilters({ enforcement: value === "all" ? "" : value })
                }
              >
                <SelectTrigger className="w-[130px] h-8">
                  <SelectValue placeholder="All actions" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All actions</SelectItem>
                  <SelectItem value="deny">Deny</SelectItem>
                  <SelectItem value="warn">Warn</SelectItem>
                  <SelectItem value="dryrun">Dryrun</SelectItem>
                </SelectContent>
              </Select>

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
          {isLoading && (
            <TableLoadingSkeleton columns={columns} showDoubleLines />
          )}

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
                  ? "No constraints match the selected filters. Try adjusting your filters."
                  : "No OPA Gatekeeper constraints have been created in the cluster."
              }
              action={
                hasFilters ? (
                  <Button variant="outline" onClick={clearFilters}>
                    Clear Filters
                  </Button>
                ) : undefined
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
                  {data.items.map((constraint) => (
                    <TableRow
                      key={`${constraint.kind}-${constraint.name}`}
                      className="cursor-pointer hover:bg-muted/50"
                    >
                      <TableCell>
                        <Link
                          to={`/compliance/constraints/${constraint.kind}/${constraint.name}`}
                          className="flex items-center gap-2 font-medium hover:underline"
                        >
                          {constraint.name}
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
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
                          <Badge
                            variant="outline"
                            className="text-green-600 border-green-200 bg-green-50 dark:text-green-400 dark:border-green-900 dark:bg-green-950/30"
                          >
                            0
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <MatchRulesDisplay
                          match={constraint.match}
                          variant="compact"
                        />
                      </TableCell>
                      <TableCell className="hidden md:table-cell text-muted-foreground text-sm">
                        {formatDistanceToNow(constraint.createdAt)}
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

export default ConstraintsPage;
