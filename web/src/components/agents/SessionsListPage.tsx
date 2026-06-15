// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ArrowLeft } from "@/lib/icons";
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table";
import { ListTableShell, ListTableHeader } from "@/components/ui/list-table";
import { Skeleton } from "@/components/ui/skeleton";
import { CompliancePagination } from "@/components/compliance/CompliancePagination";
import { RunStatusBadge } from "@/components/agents/RunStatusBadge";
import { useAgentSessions } from "@/hooks/useAgentSessions";
import { formatDistanceToNow } from "@/lib/date";

/**
 * Past chat conversations list (Story 50.6). Sessions are a server-side view
 * over Knodex-owned run records grouped by conversationId — most-recent-first,
 * namespace-visibility filtered. Row click opens the live chat. Optionally
 * scoped to one agent via ?agentType.
 */
export function SessionsListPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const agentType = searchParams.get("agentType") || undefined;

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const { data, isLoading, isError } = useAgentSessions({ agentType, page, pageSize });

  return (
    <div className="space-y-6 max-w-4xl" data-testid="sessions-list-page">
      <div className="space-y-2">
        <Link
          to="/agents"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          data-testid="sessions-back"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Agents
        </Link>
        <h1 className="text-2xl font-semibold text-foreground">Past conversations</h1>
        <p className="text-sm text-muted-foreground">
          Revisit and continue a previous agent conversation.
        </p>
      </div>

      {isLoading && <Skeleton className="h-40 w-full" data-testid="sessions-loading" />}

      {isError && (
        <div
          className="rounded-lg border border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="sessions-error"
        >
          Conversations could not be loaded. This is usually transient.
        </div>
      )}

      {!isLoading && !isError && data && data.items.length === 0 && (
        <div
          className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="sessions-empty"
        >
          No conversations yet
        </div>
      )}

      {!isLoading && !isError && data && data.items.length > 0 && (
        <>
          <ListTableShell noAnimation>
            <Table className="table-fixed">
              <ListTableHeader>
                <TableRow>
                  <TableHead className="w-[44%]">Conversation</TableHead>
                  <TableHead className="w-[18%]">Agent</TableHead>
                  <TableHead className="w-[10%]">Turns</TableHead>
                  <TableHead className="w-[14%]">Status</TableHead>
                  <TableHead className="w-[14%]">Last activity</TableHead>
                </TableRow>
              </ListTableHeader>
              <TableBody>
                {data.items.map((session) => (
                  <TableRow
                    key={session.id}
                    data-testid="session-row"
                    className="cursor-pointer"
                    onClick={() =>
                      navigate(
                        `/agents/list/${encodeURIComponent(session.agentNamespace)}/${encodeURIComponent(session.agentType)}/chat/${encodeURIComponent(session.id)}`
                      )
                    }
                  >
                    <TableCell>
                      <p className="truncate" title={session.firstPrompt}>
                        {session.firstPrompt || (
                          <span className="text-muted-foreground italic">(no prompt)</span>
                        )}
                      </p>
                    </TableCell>
                    <TableCell>
                      <div className="min-w-0">
                        <p className="font-medium truncate">{session.agentType}</p>
                        {session.agentNamespace && (
                          <p className="text-xs text-muted-foreground truncate">
                            {session.agentNamespace}
                          </p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {session.runCount}
                    </TableCell>
                    <TableCell>
                      <RunStatusBadge status={session.status} />
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground truncate">
                      {session.lastActivityAt
                        ? formatDistanceToNow(session.lastActivityAt)
                        : ""}
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
    </div>
  );
}
