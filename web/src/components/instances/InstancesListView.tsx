// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import type { Instance } from "@/types/rgd";
import { formatDistanceToNow } from "@/lib/date";
import { HealthBadge } from "./HealthBadge";
import { ScopeIndicator } from "@/components/shared/ScopeIndicator";
import { SortableHead } from "@/components/ui/sortable-table";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type SortField = "name" | "kind" | "namespace" | "health" | "conditions" | "updatedAt";
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
        case "conditions":
          aVal = a.conditions?.filter((c) => c.status === "True").length ?? 0;
          bVal = b.conditions?.filter((c) => c.status === "True").length ?? 0;
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

  return (
    <div className="rounded-lg border border-border overflow-hidden animate-fade-in-up">
      <Table className="table-fixed">
        <TableHeader>
          <TableRow>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[28%]">Name</SortableHead>
            <SortableHead field="kind" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[14%]">Kind</SortableHead>
            <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[16%]">Namespace</SortableHead>
            <SortableHead field="health" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Health</SortableHead>
            <SortableHead field="conditions" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Conditions</SortableHead>
            <SortableHead field="updatedAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[18%]">Last Updated</SortableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((instance) => {
            const trueConditions = instance.conditions?.filter((c) => c.status === "True").length ?? 0;
            const totalConditions = instance.conditions?.length ?? 0;
            return (
              <TableRow
                key={`${instance.namespace || "_cluster"}/${instance.kind}/${instance.name}`}
                className="cursor-pointer"
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
                <TableCell>
                  <HealthBadge health={instance.health} size="sm" />
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {totalConditions > 0 ? `${trueConditions}/${totalConditions}` : "—"}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {formatDistanceToNow(instance.updatedAt)}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
