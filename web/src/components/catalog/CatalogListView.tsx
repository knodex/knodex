// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import { Globe } from "@/lib/icons";
import type { CatalogRGD } from "@/types/rgd";

import { SortableHead } from "@/components/ui/sortable-table";
import { RGDIcon } from "@/components/ui/rgd-icon";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type SortField = "name" | "category" | "instances";
type SortDir = "asc" | "desc";

interface CatalogListViewProps {
  items: CatalogRGD[];
  onRGDClick?: (rgd: CatalogRGD) => void;
  /** Hide category column (used when already scoped to a single category) */
  compact?: boolean;
}

export function CatalogListView({ items, onRGDClick, compact = false }: CatalogListViewProps) {
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const handleSort = useCallback((field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  }, [sortField]);

  const sorted = useMemo(() => {
    return [...items].sort((a, b) => {
      let aVal: string | number;
      let bVal: string | number;

      switch (sortField) {
        case "name":
          aVal = (a.title || a.name).toLowerCase();
          bVal = (b.title || b.name).toLowerCase();
          break;
        case "category":
          aVal = (a.category || "").toLowerCase();
          bVal = (b.category || "").toLowerCase();
          break;
        case "instances":
          aVal = a.instances;
          bVal = b.instances;
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [items, sortField, sortDir]);

  return (
    <div className="rounded-lg border border-border overflow-hidden animate-fade-in-up">
      <Table className="table-fixed">
        <TableHeader>
          <TableRow>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className={compact ? "w-[60%]" : "w-[50%]"}>Name</SortableHead>
            {!compact && <SortableHead field="category" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[20%]">Category</SortableHead>}
            <SortableHead field="instances" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[20%]">Instances</SortableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((rgd) => {
            return (
              <TableRow
                key={`${rgd.namespace}/${rgd.name}`}
                className="cursor-pointer"
                onClick={() => onRGDClick?.(rgd)}
                role="button"
                tabIndex={0}
                aria-label={`View details for ${rgd.title || rgd.name}`}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onRGDClick?.(rgd);
                  }
                }}
              >
                <TableCell>
                  <div className="flex items-center gap-2 min-w-0">
                    <RGDIcon icon={rgd.icon} category={rgd.category || "uncategorized"} className="h-4 w-4 shrink-0" />
                    <p className="font-medium text-foreground truncate">
                      {rgd.title || rgd.name}
                      {rgd.isClusterScoped && (
                        <Globe className="inline ml-1.5 h-3 w-3 text-violet-500" aria-label="Cluster-scoped" />
                      )}
                    </p>
                  </div>
                </TableCell>
                {!compact && (
                  <TableCell>
                    <span className="px-2 py-0.5 rounded-md text-xs font-semibold bg-primary/10 text-primary">
                      {(rgd.category || "uncategorized").toLowerCase()}
                    </span>
                  </TableCell>
                )}
                <TableCell className="text-sm text-muted-foreground">
                  {rgd.instances}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
