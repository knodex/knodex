// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { Bot, FolderKanban, Pencil } from "@/lib/icons";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { AgentModelBadge } from "@/components/agents/AgentModelBadge";
import type { AgentModel } from "@/api/agents";
import { cn } from "@/lib/utils";

interface AgentCardProps {
  name: string;
  description: string;
  /** Namespace the agent is deployed in. */
  namespace?: string;
  /**
   * Resolved AI model (Story 50.4): renders a model badge in place of the
   * removed kagent badge. Omitted/absent ⇒ no badge.
   */
  model?: AgentModel | null;
  /**
   * Live in-flight indicator (Story 49.4, UX-DR6): true when this agent has
   * a run in "running" state. Renders a pulsing status dot next to the
   * badges; updates via the shared ["agents","runs"] query invalidation.
   */
  running?: boolean;
  /**
   * Navigation target (Story 53.2): when set, the card renders as a Link
   * with the RGDCard hover affordance. Every agent is now chat-enabled via the
   * namespaced invoke path, so all cards pass `to`.
   */
  to?: string;
  /**
   * Model-edit affordance: when set, a pencil button renders in the card's
   * top-right corner. On the navigable (Link) variant it intercepts the click
   * so opening the editor never navigates to the chat page.
   */
  onEdit?: () => void;
  className?: string;
}

/**
 * Agent card for the Agents workspace (Story 49.2). Mirrors the RGDCard
 * structure. Display-only by default; cards with a page pass `to` to become
 * navigable (Story 53.2 — every agent is chat-enabled).
 */
export function AgentCard({
  name,
  description,
  namespace,
  model,
  running = false,
  to,
  onEdit,
  className,
}: AgentCardProps) {
  // Intercept on the Link variant so the editor opens instead of navigating.
  const handleEdit = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onEdit?.();
  };

  const inner = (
    <>
      {/* Header */}
      <div className="flex items-start gap-3 mb-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Bot className="h-5 w-5" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <h3
            className={cn(
              "font-semibold text-foreground line-clamp-2 text-base",
              to && "group-hover:text-primary transition-colors duration-200"
            )}
          >
            {name}
          </h3>
          {namespace && (
            <p className="flex items-center gap-1.5 text-xs text-muted-foreground truncate mt-1">
              <FolderKanban className="h-3 w-3 shrink-0 text-muted-foreground/70" />
              <span className="truncate">{namespace}</span>
            </p>
          )}
        </div>
        {onEdit && (
          <button
            type="button"
            onClick={handleEdit}
            data-testid="agent-edit-button"
            aria-label={`Change model for ${name}`}
            className="shrink-0 -mr-1 -mt-1 rounded-md p-1.5 text-muted-foreground/70 hover:text-foreground hover:bg-accent/50 transition-colors"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
          </button>
        )}
      </div>

      {/* Description */}
      <p className="text-sm text-muted-foreground line-clamp-2 mb-4 leading-relaxed">
        {description || "No description available"}
      </p>

      <div className="flex flex-wrap gap-1.5">
        <AgentModelBadge model={model} />
        {running && (
          <span
            data-testid="agent-running-indicator"
            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60"
          >
            <StatusIndicator status="progressing" />
            Running
          </span>
        )}
      </div>
    </>
  );

  // Navigable variant (Story 53.2): Link wrapper with the RGDCard hover
  // affordance — every agent opens its conversational page.
  if (to) {
    return (
      <Link
        to={to}
        data-testid="agent-card"
        aria-label={`Open ${name}`}
        className={cn(
          "group block rounded-lg border border-border/60 bg-card p-5",
          "transition-all duration-200 ease-out",
          "hover:border-primary/30 hover:bg-accent/5",
          className
        )}
      >
        {inner}
      </Link>
    );
  }

  return (
    <div
      data-testid="agent-card"
      className={cn(
        "rounded-lg border border-border/60 bg-card p-5",
        className
      )}
    >
      {inner}
    </div>
  );
}
