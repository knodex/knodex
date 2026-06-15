// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Navigate, Outlet, useParams } from "react-router-dom";
import { RefreshCw } from "@/lib/icons";
import { useAgents } from "@/hooks/useAgents";
import { Skeleton } from "@/components/ui/skeleton";
import { SessionsSidebar } from "@/components/agents/SessionsSidebar";

/**
 * Kagent-style layout for agent chat. Valid agents come from the live
 * Casbin-scoped list, not a static registry, so we must NOT redirect while the
 * list is loading — that would bounce a valid agent. Once loaded, a
 * namespace+name pair absent from the list redirects to /agents/list.
 */
export function AgentChatLayout() {
  const { namespace, name, sessionId } = useParams<{ namespace: string; name: string; sessionId?: string }>();
  const { data, isLoading, isError, refetch } = useAgents();

  // Don't decide until the list resolves — avoids a flash-redirect on a valid
  // agent during the in-flight fetch.
  if (isLoading) {
    return (
      <div className="flex -mx-6 -mt-4 -mb-8 lg:-mx-10 min-h-[calc(100vh-4rem)]">
        <aside className="hidden md:flex w-56 flex-shrink-0 flex-col border-r border-border p-3">
          <Skeleton className="h-8 w-full" data-testid="agent-chat-loading" />
        </aside>
        <div className="flex-1 min-w-0 px-6 pt-4 pb-8 lg:px-10">
          <Skeleton className="h-40 w-full" />
        </div>
      </div>
    );
  }

  // A fetch failure with no cached data is NOT the same as "unknown agent" —
  // show a retryable error instead of ejecting the user from a valid chat.
  // (When a prior fetch succeeded, React Query keeps `data`, so we fall through
  // to the normal membership check below.)
  if (isError && !data) {
    return (
      <div
        className="flex flex-col items-center justify-center min-h-[40vh] gap-3 px-6 text-center"
        data-testid="agent-chat-error"
      >
        <p className="text-sm text-muted-foreground max-w-md">
          Couldn't load agents. This is usually transient.
        </p>
        <button
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          <RefreshCw className="h-4 w-4" />
          Retry
        </button>
      </div>
    );
  }

  if (!data?.agents.some((a) => a.namespace === namespace && a.name === name)) {
    return <Navigate to="/agents/list" replace />;
  }

  return (
    <div className="flex -mx-6 -mt-4 -mb-8 lg:-mx-10 min-h-[calc(100vh-4rem)]">
      <aside
        className="hidden md:flex w-56 flex-shrink-0 flex-col border-r border-border"
        data-testid="agent-chat-sidebar"
      >
        <SessionsSidebar namespace={namespace!} name={name!} activeSessionId={sessionId} />
      </aside>
      <div className="flex-1 min-w-0 px-6 pt-4 pb-8 lg:px-10">
        <Outlet />
      </div>
    </div>
  );
}
