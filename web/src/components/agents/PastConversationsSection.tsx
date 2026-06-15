// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table";
import { ListTableShell, ListTableHeader } from "@/components/ui/list-table";
import { Skeleton } from "@/components/ui/skeleton";
import { CompliancePagination } from "@/components/compliance/CompliancePagination";
import { RunStatusBadge } from "@/components/agents/RunStatusBadge";
import { useAgentSessions } from "@/hooks/useAgentSessions";
import { formatDistanceToNow } from "@/lib/date";

/**
 * Past conversations section for the Agents workspace (Story 53.2): the caller's
 * recent sessions (unscoped), most-recent-first, opening the live chat on row
 * click. Every session now carries a namespace + agent name.
 */
export function PastConversationsSection() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const { data, isLoading, isError } = useAgentSessions({ page, pageSize });

  return (
    <section
      aria-labelledby="past-conversations-heading"
      className="space-y-3"
      data-testid="past-conversations-section"
    >
      <h2 id="past-conversations-heading" className="text-lg font-semibold text-foreground">
        Past Conversations
      </h2>

      {isLoading && (
        <Skeleton className="h-40 w-full" data-testid="past-conversations-loading" />
      )}

      {isError && (
        <div
          className="rounded-lg border border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="past-conversations-error"
        >
          Conversations could not be loaded. This is usually transient.
        </div>
      )}

      {!isLoading && !isError && data && data.items.length === 0 && (
        <div
          className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground"
          data-testid="past-conversations-empty"
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
                  <TableHead className="w-[50%]">Conversation</TableHead>
                  <TableHead className="w-[12%]">Turns</TableHead>
                  <TableHead className="w-[18%]">Status</TableHead>
                  <TableHead className="w-[20%]">Last activity</TableHead>
                </TableRow>
              </ListTableHeader>
              <TableBody>
                {data.items.map((session) => (
                  <TableRow
                    key={session.id}
                    data-testid="past-conversation-row"
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
    </section>
  );
}
