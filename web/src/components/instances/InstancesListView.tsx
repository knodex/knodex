// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import type { Instance } from "@/types/rgd";
import { formatDistanceToNow } from "@/lib/date";
import { ChevronRight } from "@/lib/icons";
import { HealthBadge } from "./HealthBadge";
import { ScopeIndicator } from "@/components/shared/ScopeIndicator";
import { SortableHead } from "@/components/ui/sortable-table";
import { deriveActorLabel } from "./instance-utils";
import {
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { ListTableHeader, ListTableShell } from "@/components/ui/list-table";

type SortField = "name" | "kind" | "namespace" | "health" | "updatedAt";
type SortDir = "asc" | "desc";

const HEALTH_ORDER: Record<string, number> = {
  Healthy: 0,
  Progressing: 1,
  Degraded: 2,
  Unknown: 3,
  Unhealthy: 4,
};

interface InstancesListViewProps {
  items: Instance[];
  onInstanceClick?: (instance: Instance) => void;
}

export function InstancesListView({ items, onInstanceClick }: InstancesListViewProps) {
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
          aVal = a.name.toLowerCase();
          bVal = b.name.toLowerCase();
          break;
        case "kind":
          aVal = a.kind.toLowerCase();
          bVal = b.kind.toLowerCase();
          break;
        case "namespace":
          aVal = (a.namespace || "").toLowerCase();
          bVal = (b.namespace || "").toLowerCase();
          break;
        case "health":
          aVal = HEALTH_ORDER[a.health] ?? 99;
          bVal = HEALTH_ORDER[b.health] ?? 99;
          break;
        case "updatedAt":
          aVal = a.updatedAt;
          bVal = b.updatedAt;
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [items, sortField, sortDir]);

  // Raw <table> (no shadcn <Table> wrapper) so the sticky thead escapes the
  // wrapper's overflow-auto containing block and sticks at page-scroll level.
  return (
    <ListTableShell className="overflow-visible">
      <table className="w-full caption-bottom text-sm table-fixed">
        <ListTableHeader sticky>
          <TableRow>
            <SortableHead field="health" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Status</SortableHead>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[30%]">Name</SortableHead>
            <SortableHead field="kind" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[16%]">Kind</SortableHead>
            <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[18%]">Namespace</SortableHead>
            <SortableHead field="updatedAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[24%]">Updated</SortableHead>
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {sorted.map((instance) => (
            <TableRow
              key={`${instance.apiVersion}/${instance.namespace || "_cluster"}/${instance.kind}/${instance.name}`}
              className="group cursor-pointer focus-visible:outline-none focus-visible:shadow-[inset_2px_0_0_var(--brand-primary)]"
              onClick={() => onInstanceClick?.(instance)}
              role="button"
              tabIndex={0}
              aria-label={`View details for ${instance.name}`}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onInstanceClick?.(instance);
                }
              }}
            >
              <TableCell>
                <HealthBadge health={instance.health} size="sm" />
              </TableCell>
              <TableCell>
                <div className="min-w-0">
                  <p className="font-medium text-foreground truncate">{instance.name}</p>
                  <p className="text-xs text-muted-foreground font-mono truncate">
                    {instance.rgdName}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <span className="px-2 py-0.5 rounded-md text-xs font-semibold bg-primary/10 text-primary">
                  {instance.kind}
                </span>
              </TableCell>
              <TableCell>
                <ScopeIndicator
                  isClusterScoped={instance.isClusterScoped}
                  namespace={instance.namespace}
                  variant="badge"
                />
              </TableCell>
              <TableCell className="text-xs text-muted-foreground" style={{ fontVariantNumeric: "tabular-nums" }}>
                <div className="flex items-center justify-between gap-2">
                  <div className="flex flex-col leading-tight min-w-0">
                    <span className="truncate">{formatDistanceToNow(instance.updatedAt)}</span>
                    <span className="text-[10px] text-muted-foreground/70 truncate">
                      {deriveActorLabel(instance)}
                    </span>
                  </div>
                  {/* Decorative-only affordance — lives inside the data cell so it doesn't
                      introduce a spacer column that confuses screen-reader column counts. */}
                  <ChevronRight
                    aria-hidden="true"
                    className="h-4 w-4 text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100 shrink-0"
                  />
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </table>
    </ListTableShell>
  );
}
