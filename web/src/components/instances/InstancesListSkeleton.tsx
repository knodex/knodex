// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";
import {
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@/components/ui/table";
import { ListTableHeader, ListTableShell } from "@/components/ui/list-table";

interface InstancesListSkeletonProps {
  /** Number of placeholder rows to render. Defaults to 8. */
  rows?: number;
}

// Column widths must mirror InstancesListView.tsx (sum: 100%).
export function InstancesListSkeleton({ rows = 8 }: InstancesListSkeletonProps) {
  return (
    <ListTableShell
      className="overflow-visible"
      noAnimation
      aria-busy="true"
      aria-label="Loading instances"
    >
      <table className="w-full caption-bottom text-sm table-fixed">
        <ListTableHeader>
          <TableRow>
            <TableHead className="w-[12%]">Status</TableHead>
            <TableHead className="w-[30%]">Name</TableHead>
            <TableHead className="w-[16%]">Kind</TableHead>
            <TableHead className="w-[18%]">Namespace</TableHead>
            <TableHead className="w-[24%]">Updated</TableHead>
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {Array.from({ length: rows }).map((_, i) => (
            <TableRow key={`skeleton-row-${i}`}>
              <TableCell><Skeleton className="h-5 w-16 rounded-full" /></TableCell>
              <TableCell>
                <Skeleton className="h-4 w-40 mb-1" />
                <Skeleton className="h-3 w-24" />
              </TableCell>
              <TableCell><Skeleton className="h-5 w-20" /></TableCell>
              <TableCell><Skeleton className="h-5 w-28" /></TableCell>
              <TableCell>
                <Skeleton className="h-3 w-16 mb-1" />
                <Skeleton className="h-3 w-12" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </table>
    </ListTableShell>
  );
}
