// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useNavigate } from "react-router-dom";
import { Plus } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgentSessions } from "@/hooks/useAgentSessions";
import { formatDistanceToNow } from "@/lib/date";
import { cn } from "@/lib/utils";

interface SessionsSidebarProps {
  namespace: string;
  name: string;
  activeSessionId?: string;
}

export function SessionsSidebar({ namespace, name, activeSessionId }: SessionsSidebarProps) {
  const navigate = useNavigate();
  // Sessions are still keyed by agent name (agentType) server-side.
  const { data, isLoading } = useAgentSessions({ agentType: name, pageSize: 50 });

  const chatPath = (sessionId: string) =>
    `/agents/list/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/chat/${encodeURIComponent(sessionId)}`;

  const handleNewConversation = () => {
    navigate(chatPath(crypto.randomUUID()));
  };

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="p-3 border-b border-border">
        <Button
          variant="outline"
          size="sm"
          className="w-full justify-start"
          onClick={handleNewConversation}
          data-testid="sessions-sidebar-new"
        >
          <Plus className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
          New conversation
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto py-2">
        {isLoading && (
          <div className="space-y-1 px-2">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-12 w-full rounded-md" />
            ))}
          </div>
        )}

        {!isLoading && data?.items.length === 0 && (
          <p
            className="px-4 py-6 text-xs text-muted-foreground text-center"
            data-testid="sessions-sidebar-empty"
          >
            No past conversations
          </p>
        )}

        {!isLoading && data && data.items.length > 0 && (
          <ul data-testid="sessions-sidebar-list">
            {data.items.map((session) => (
              <li key={session.id}>
                <button
                  type="button"
                  className={cn(
                    "w-full text-left px-3 py-2 mx-1 rounded-md text-sm transition-colors",
                    "flex flex-col gap-0.5",
                    session.id === activeSessionId
                      ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
                      : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]"
                  )}
                  onClick={() => navigate(chatPath(session.id))}
                  data-testid="sessions-sidebar-item"
                >
                  <span className="truncate font-medium leading-tight">
                    {session.firstPrompt || (
                      <span className="italic text-muted-foreground">(no prompt)</span>
                    )}
                  </span>
                  {session.lastActivityAt && (
                    <span className="text-xs text-muted-foreground truncate">
                      {formatDistanceToNow(session.lastActivityAt)}
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
