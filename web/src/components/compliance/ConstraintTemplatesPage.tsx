// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Link } from "react-router-dom";
import { FileText, ChevronRight, Layers } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useConstraintTemplates } from "@/hooks/useCompliance";
import { PageHeader } from "./PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { formatDistanceToNow } from "@/lib/date";

/**
 * ConstraintTemplates list page
 * AC-TPL-01: Table of templates showing name, kind, description
 * AC-TPL-02: Link to constraints using this template
 * AC-TPL-03: Row click navigates to template detail
 */
export function ConstraintTemplatesPage() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const { data, isLoading, isError, error, refetch, isRefetching } =
    useConstraintTemplates({ page, pageSize });

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
    setPage(1);
  };

  const columns = [
    { header: "Name", width: "30%" },
    { header: "Kind", width: "20%" },
    { header: "Description", width: "35%", hideOnMobile: true },
    { header: "Created", width: "15%", hideOnMobile: true },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Constraint Templates"
        subtitle="OPA Gatekeeper policy templates that define constraint kinds"
        breadcrumbs={[
          { label: "Compliance", href: "/compliance" },
          { label: "Templates" },
        ]}
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-lg">
            <FileText className="h-5 w-5 text-muted-foreground" />
            Templates
            {data && (
              <span className="text-sm font-normal text-muted-foreground">
                ({data.total})
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading && (
            <TableLoadingSkeleton columns={columns} showDoubleLines />
          )}

          {isError && (
            <ErrorState
              message="Failed to load templates"
              details={error instanceof Error ? error.message : "Unknown error"}
              onRetry={() => refetch()}
              isRetrying={isRefetching}
            />
          )}

          {!isLoading && !isError && data?.items.length === 0 && (
            <EmptyState
              icon={<Layers className="h-8 w-8 text-muted-foreground" />}
              title="No Constraint Templates"
              description="No OPA Gatekeeper ConstraintTemplates have been installed in the cluster."
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
                        className={col.hideOnMobile ? "hidden md:table-cell" : ""}
                        style={{ width: col.width }}
                      >
                        {col.header}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((template) => (
                    <TableRow
                      key={template.name}
                      className="cursor-pointer hover:bg-muted/50"
                    >
                      <TableCell>
                        <Link
                          to={`/compliance/templates/${template.name}`}
                          className="flex items-center gap-2 font-medium hover:underline"
                        >
                          {template.name}
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        </Link>
                      </TableCell>
                      <TableCell>
                        <Link
                          to={`/compliance/constraints?kind=${template.kind}`}
                          className="text-primary hover:underline"
                        >
                          {template.kind}
                        </Link>
                      </TableCell>
                      <TableCell className="hidden md:table-cell text-muted-foreground">
                        <span className="line-clamp-2">
                          {template.description || "No description"}
                        </span>
                      </TableCell>
                      <TableCell className="hidden md:table-cell text-muted-foreground text-sm">
                        {formatDistanceToNow(template.createdAt)}
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

export default ConstraintTemplatesPage;
