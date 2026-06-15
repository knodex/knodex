// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback, memo } from "react";
import { FileCode } from "@/lib/icons";
import { SortableHead } from "@/components/ui/sortable-table";
import { TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table";
import { ListTableHeader, ListTableShell } from "@/components/ui/list-table";
import type { CatalogRGD } from "@/types/rgd";

type SortField = "name" | "instances";
type SortDir = "asc" | "desc";

interface AgentsTemplatesListViewProps {
  items: CatalogRGD[];
  onDeploy: (name: string) => void;
}

/**
 * Table (list) view for agent templates — the list-mode counterpart to the
 * template card grid, mirroring AgentsListView so the Templates page matches the
 * Agents page toolbar/list pattern. Each row deploys via the standard
 * /deploy/{name} flow.
 */
export const AgentsTemplatesListView = memo(function AgentsTemplatesListView({
  items,
  onDeploy,
}: AgentsTemplatesListViewProps) {
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const handleSort = useCallback(
    (field: SortField) => {
      if (sortField === field) {
        setSortDir((d) => (d === "asc" ? "desc" : "asc"));
      } else {
        setSortField(field);
        setSortDir("asc");
      }
    },
    [sortField]
  );

  const sorted = useMemo(() => {
    return [...items].sort((a, b) => {
      if (sortField === "instances") {
        return sortDir === "asc" ? a.instances - b.instances : b.instances - a.instances;
      }
      const aVal = (a.title || a.name).toLowerCase();
      const bVal = (b.title || b.name).toLowerCase();
      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [items, sortField, sortDir]);

  return (
    <ListTableShell className="overflow-visible">
      <table className="w-full caption-bottom text-sm table-fixed" data-testid="agents-templates-table">
        <ListTableHeader sticky>
          <TableRow>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[24%]">Name</SortableHead>
            <TableHead className="w-[34%]">Description</TableHead>
            <TableHead className="w-[18%]">Tags</TableHead>
            <SortableHead field="instances" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Instances</SortableHead>
            <TableHead className="w-[12%]">
              <span className="sr-only">Actions</span>
            </TableHead>
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {sorted.map((t) => (
            <TableRow key={t.name} data-testid="agents-templates-row" className="group">
              <TableCell>
                <div className="flex items-center gap-3 min-w-0">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                    <FileCode className="h-4 w-4" aria-hidden="true" />
                  </div>
                  <span className="font-medium text-foreground truncate">{t.title || t.name}</span>
                </div>
              </TableCell>
              <TableCell className="text-muted-foreground truncate">
                {t.description || "—"}
              </TableCell>
              <TableCell>
                {t.tags && t.tags.length > 0 ? (
                  <div className="flex flex-wrap gap-1">
                    {t.tags.slice(0, 2).map((tag) => (
                      <span
                        key={tag}
                        className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60"
                      >
                        {tag}
                      </span>
                    ))}
                    {t.tags.length > 2 && (
                      <span className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60">
                        +{t.tags.length - 2}
                      </span>
                    )}
                  </div>
                ) : (
                  <span className="text-xs text-muted-foreground/50">—</span>
                )}
              </TableCell>
              <TableCell className="text-muted-foreground">{t.instances}</TableCell>
              <TableCell>
                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => onDeploy(t.name)}
                    data-testid="deploy-template-button"
                    className="inline-flex items-center rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
                  >
                    Deploy
                  </button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </table>
    </ListTableShell>
  );
});
