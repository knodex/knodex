// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table";
import { ListTableShell, ListTableHeader } from "@/components/ui/list-table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { filterSelectClasses } from "@/components/ui/filter-bar";
import { CompliancePagination } from "@/components/compliance/CompliancePagination";
import { RunStatusBadge } from "@/components/agents/RunStatusBadge";
import { useAgentRuns } from "@/hooks/useAgentRuns";
import { useAgents } from "@/hooks/useAgents";
import { parseContextRef } from "@/components/agents/context-ref";
import type { AgentRunStatus } from "@/api/agent-runs";
import { cn } from "@/lib/utils";

/** Context cell: linked when the contextRef is parseable, plain text otherwise. */
function ContextRefCell({ contextRef }: { contextRef: string }) {
  if (!contextRef) {
    return <span className="text-xs text-muted-foreground italic">—</span>;
  }
  const parsed = parseContextRef(contextRef);
  if (parsed.kind === "text") {
    return (
      <span className="text-sm text-muted-foreground truncate block max-w-[200px]" title={contextRef}>
        {parsed.label}
      </span>
    );
  }
  return (
    <Link to={parsed.to} className="text-primary hover:underline text-sm truncate block max-w-[200px]">
      {parsed.label}
    </Link>
  );
}

const STATUS_OPTIONS: { value: AgentRunStatus; label: string }[] = [
  { value: "running", label: "Running" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
];

/**
 * Run history table for the Agents workspace (Story 49.4, UX-DR4): paginated,
 * filterable by agent type and status. Live updates: WebSocket invalidation
 * (actor/admin) + conditional polling while runs are in-flight (everyone) —
 * see useAgentRuns.
 */
export function RunHistorySection() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState("");
  const [agentTypeFilter, setAgentTypeFilter] = useState("");

  const { data, isLoading, isError } = useAgentRuns({
    agentType: agentTypeFilter || undefined,
    status: (statusFilter || undefined) as AgentRunStatus | undefined,
    page,
    pageSize,
  });

  // Agent type filter options: visible agent names ∪ agent types present in the
  // current page (covers since-uninstalled agents).
  const { data: agentsData } = useAgents();
  const agentTypeOptions = useMemo(() => {
    const names = new Set<string>();
    agentsData?.agents.forEach((a) => names.add(a.name));
    data?.items.forEach((r) => names.add(r.agentType));
    if (agentTypeFilter) names.add(agentTypeFilter);
    return [...names].sort();
  }, [agentsData, data, agentTypeFilter]);

  return (
    <section aria-labelledby="run-history-heading" className="space-y-3" data-testid="run-history-section">
      <h2 id="run-history-heading" className="text-lg font-semibold text-foreground">
        Run History
      </h2>

      {/* Filter bar: status + agent type (filter changes reset to page 1) */}
      <div className="flex items-center gap-2">
        <Select
          value={statusFilter || "__all__"}
          onValueChange={(v) => {
            setStatusFilter(v === "__all__" ? "" : v);
            setPage(1);
          }}
        >
          <SelectTrigger
            className={cn(filterSelectClasses(!!statusFilter), "h-8 w-[150px] text-xs")}
            aria-label="Filter by status"
            data-testid="run-status-filter"
          >
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All statuses</SelectItem>
            {STATUS_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={agentTypeFilter || "__all__"}
          onValueChange={(v) => {
            setAgentTypeFilter(v === "__all__" ? "" : v);
            setPage(1);
          }}
        >
          <SelectTrigger
            className={cn(filterSelectClasses(!!agentTypeFilter), "h-8 w-[180px] text-xs")}
            aria-label="Filter by agent"
            data-testid="run-agent-filter"
          >
            <SelectValue placeholder="All agents" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All agents</SelectItem>
            {agentTypeOptions.map((name) => (
              <SelectItem key={name} value={name}>
                {name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isLoading && <Skeleton className="h-40 w-full" data-testid="run-history-loading" />}

      {isError && (
        <div
          className="rounded-lg border border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="run-history-error"
        >
          Run history could not be loaded. This is usually transient.
        </div>
      )}

      {!isLoading && !isError && data && data.items.length === 0 && (
        <div
          className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="run-history-empty"
        >
          No agent runs yet
        </div>
      )}

      {!isLoading && !isError && data && data.items.length > 0 && (
        <>
          <ListTableShell noAnimation>
            <Table className="table-fixed">
              <ListTableHeader>
                <TableRow>
                  <TableHead className="w-[22%]">Agent</TableHead>
                  <TableHead className="w-[22%]">Triggered by</TableHead>
                  <TableHead className="w-[22%]">Context</TableHead>
                  <TableHead className="w-[14%]">Status</TableHead>
                  <TableHead className="w-[20%]">Time</TableHead>
                </TableRow>
              </ListTableHeader>
              <TableBody>
                {data.items.map((run) => (
                  <TableRow key={run.id} data-testid="run-history-row">
                    <TableCell>
                      <div className="min-w-0">
                        <p className="font-medium truncate">{run.agentType}</p>
                        {run.agentNamespace && (
                          <p className="text-xs text-muted-foreground truncate">{run.agentNamespace}</p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground truncate">{run.actor}</TableCell>
                    <TableCell>
                      <ContextRefCell contextRef={run.contextRef} />
                    </TableCell>
                    <TableCell>
                      <RunStatusBadge status={run.status} />
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground truncate">
                      {run.timestamp ? new Date(run.timestamp).toLocaleString() : ""}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </ListTableShell>

          <CompliancePagination
            page={page}
            pageSize={pageSize}
            totalCount={data.total}
            onPageChange={setPage}
            onPageSizeChange={(s) => {
              setPageSize(s);
              setPage(1);
            }}
          />
        </>
      )}
    </section>
  );
}
