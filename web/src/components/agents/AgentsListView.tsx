// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback, memo } from "react";
import { Link } from "react-router-dom";
import { Bot, ChevronRight, Pencil } from "@/lib/icons";
import { SortableHead } from "@/components/ui/sortable-table";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { AgentModelBadge } from "@/components/agents/AgentModelBadge";
import { TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table";
import { ListTableHeader, ListTableShell } from "@/components/ui/list-table";
import type { InstalledAgent } from "@/api/agents";

type SortField = "name" | "namespace";
type SortDir = "asc" | "desc";

interface AgentsListViewProps {
  items: InstalledAgent[];
  /** True when the agent has a run in "running" state (live in-flight). */
  isRunning: (name: string, namespace: string) => boolean;
  /** Chat route for an agent — drives the row's name Link (keyboard/AT target). */
  hrefForAgent: (agent: InstalledAgent) => string;
  /** Mouse convenience: clicking anywhere on the row navigates to the chat. */
  onAgentClick: (agent: InstalledAgent) => void;
  onEdit: (agent: InstalledAgent) => void;
}

/**
 * Table (list) view for the Agents workspace — the list-mode counterpart to the
 * AgentCard grid, mirroring InstancesListView so the Agents page matches the
 * Instances page toolbar/list pattern. Rows navigate to the agent's chat route;
 * the edit pencil intercepts the click to open the model editor.
 */
export const AgentsListView = memo(function AgentsListView({
  items,
  isRunning,
  hrefForAgent,
  onAgentClick,
  onEdit,
}: AgentsListViewProps) {
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
      const aVal = (sortField === "name" ? a.name : a.namespace).toLowerCase();
      const bVal = (sortField === "name" ? b.name : b.namespace).toLowerCase();
      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [items, sortField, sortDir]);

  return (
    <ListTableShell className="overflow-visible">
      <table className="w-full caption-bottom text-sm table-fixed">
        <ListTableHeader sticky>
          <TableRow>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[34%]">Name</SortableHead>
            <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[22%]">Namespace</SortableHead>
            <TableHead className="w-[26%]">Model</TableHead>
            <TableHead className="w-[12%]">Status</TableHead>
            <TableHead className="w-[6%]">
              <span className="sr-only">Actions</span>
            </TableHead>
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {sorted.map((agent) => {
            const running = isRunning(agent.name, agent.namespace);
            return (
              <TableRow
                key={`${agent.namespace}/${agent.name}`}
                data-testid="agent-row"
                className="group cursor-pointer"
                // Mouse convenience only — the name Link below is the keyboard/AT
                // navigation target, so the row carries no button role (which
                // would invalidly nest the edit button inside an ARIA button).
                onClick={() => onAgentClick(agent)}
              >
                <TableCell>
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                      <Bot className="h-4 w-4" aria-hidden="true" />
                    </div>
                    <div className="min-w-0">
                      <Link
                        to={hrefForAgent(agent)}
                        aria-label={`Open ${agent.name}`}
                        // Row onClick already navigates on mouse; the Link adds a
                        // real keyboard/AT focus target. Stop propagation so the
                        // click isn't double-handled by the row.
                        onClick={(e) => e.stopPropagation()}
                        className="block font-medium text-foreground truncate group-hover:text-primary transition-colors focus-visible:outline-none focus-visible:text-primary focus-visible:underline"
                      >
                        {agent.name}
                      </Link>
                      {agent.description && (
                        <p className="text-xs text-muted-foreground truncate">
                          {agent.description}
                        </p>
                      )}
                    </div>
                  </div>
                </TableCell>
                <TableCell className="text-xs text-muted-foreground font-mono truncate">
                  {agent.namespace}
                </TableCell>
                <TableCell>
                  <AgentModelBadge model={agent.model} />
                </TableCell>
                <TableCell>
                  {running ? (
                    <span
                      data-testid="agent-running-indicator"
                      className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"
                    >
                      <StatusIndicator status="progressing" />
                      Running
                    </span>
                  ) : (
                    <span className="text-xs text-muted-foreground/50">—</span>
                  )}
                </TableCell>
                <TableCell>
                  <div className="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        onEdit(agent);
                      }}
                      data-testid="agent-edit-button"
                      aria-label={`Change model for ${agent.name}`}
                      className="shrink-0 rounded-md p-1.5 text-muted-foreground/70 hover:text-foreground hover:bg-accent/50 transition-colors"
                    >
                      <Pencil className="h-4 w-4" aria-hidden="true" />
                    </button>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100 shrink-0"
                    />
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </table>
    </ListTableShell>
  );
});
