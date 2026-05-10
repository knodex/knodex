// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Link } from "react-router-dom";
import { Layers } from "@/lib/icons";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHead } from "@/components/ui/sortable-table";
import { useConstraintTemplates, isEnterprise } from "@/hooks/useCompliance";
import { isEnterpriseRequired } from "@/api/compliance";
import { PageHeader } from "@/components/layout/PageHeader";
import { CompliancePagination } from "./CompliancePagination";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";
import { EmptyState } from "./EmptyState";
import { ErrorState } from "./ErrorState";
import { EnterpriseRequired } from "./EnterpriseRequired";
import { formatDistanceToNow } from "@/lib/date";

type SortField = "name" | "kind" | "createdAt";
type SortDir = "asc" | "desc";

export function ConstraintTemplatesPage() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const { data, isLoading, isError, error, refetch, isRefetching } =
    useConstraintTemplates({ page, pageSize });

  if (!isEnterprise() || (isError && isEnterpriseRequired(error))) {
    return (
      <EnterpriseRequired
        feature="Constraint Templates"
        description="View and manage OPA Gatekeeper ConstraintTemplates with an Enterprise license."
      />
    );
  }

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  const columns = [
    { header: "Name", width: "30%" },
    { header: "Kind", width: "20%" },
    { header: "Description", width: "35%", hideOnMobile: true },
    { header: "Created", width: "15%", hideOnMobile: true },
  ];

  return (
    <section className="space-y-6">
      <PageHeader title="Constraint Templates" breadcrumbs={[{ label: "Compliance", href: "/compliance" }, { label: "Constraint Templates" }]} />

      {isLoading && <TableLoadingSkeleton columns={columns} showDoubleLines />}

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
          <div className="rounded-lg border border-border overflow-hidden">
            <Table className="table-fixed">
              <TableHeader>
                <TableRow>
                  <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[30%]">Name</SortableHead>
                  <SortableHead field="kind" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[20%]">Kind</SortableHead>
                  <SortableHead field="createdAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[35%] hidden md:table-cell">Description</SortableHead>
                  <SortableHead field="createdAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[15%] hidden md:table-cell">Created</SortableHead>
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
                        className="font-medium hover:underline"
                      >
                        {template.name}
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
                    <TableCell className="hidden md:table-cell text-muted-foreground truncate">
                      {template.description || "No description"}
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-xs text-muted-foreground">
                      {formatDistanceToNow(template.createdAt)}
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

export default ConstraintTemplatesPage;
