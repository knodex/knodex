// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo } from "react";
import { GitBranch, Plus, Search, Trash2, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ListFooter } from "@/components/ui/list-footer";
import { Skeleton } from "@/components/ui/skeleton";
import { formatDate } from "@/lib/date";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import type { RepositoryConfig } from "@/types/repository";
import { getRepositoryDisplayURL } from "@/types/repository";

interface RepositoryListProps {
  repositories: RepositoryConfig[];
  onEdit?: (repo: RepositoryConfig) => void;
  onDelete?: (repoID: string) => void;
  onCreate?: () => void;
  canManage?: boolean;
  isLoadingPermission?: boolean;
  isLoading?: boolean;
}

/**
 * Connection-registry mental model (Story 48.7): a repository is presented as a
 * binary connection — `Connected` (green) when its `validationStatus` is `"valid"`,
 * `Disconnected` (neutral grey) for everything else (`"invalid"`, `"unknown"`,
 * `undefined`). Deliberately a binary state, not a sync-style health tristate —
 * a repository is a registration, not a continuously reconciled target.
 */
function getConnectionState(status?: string): {
  label: "Connected" | "Disconnected";
  className: string;
} {
  return status === "valid"
    ? { label: "Connected", className: "bg-status-success/10 text-status-success" }
    : { label: "Disconnected", className: "bg-secondary text-muted-foreground" };
}

/**
 * Format `createdAt` for the per-row "Connected since" metadatum. Returns null when
 * the value is absent or unparseable so the UI renders nothing (no "Invalid Date").
 */
function formatConnectedSince(createdAt?: string): string | null {
  if (!createdAt) return null;
  const parsed = new Date(createdAt);
  if (isNaN(parsed.getTime())) return null;
  return formatDate(createdAt);
}

export function RepositoryList({
  repositories,
  onEdit,
  onDelete,
  onCreate,
  canManage = false,
  isLoading = false,
}: RepositoryListProps) {
  const [searchQuery, setSearchQuery] = useState("");

  // Prototype landing (48.7 follow-up): card layout, no sortable column headers.
  // Cards are ordered: Connected first (alphabetical), then Disconnected (alphabetical).
  // This matches the prototype's visual grouping (healthy state on top).
  const sorted = useMemo(() => {
    let items = repositories;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      items = items.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          getRepositoryDisplayURL(r).toLowerCase().includes(q) ||
          r.projectId?.toLowerCase().includes(q)
      );
    }

    return [...items].sort((a, b) => {
      const aConnected = getConnectionState(a.validationStatus).label === "Connected";
      const bConnected = getConnectionState(b.validationStatus).label === "Connected";
      if (aConnected !== bConnected) return aConnected ? -1 : 1;
      return a.name.toLowerCase().localeCompare(b.name.toLowerCase());
    });
  }, [repositories, searchQuery]);

  // Footer counts are computed over the VISIBLE (filtered) rows, not the raw
  // `repositories` array — mirrors the 48.2/48.3/48.6 ListFooter precedent so
  // the breakdown stays consistent with the rows on screen when a search is active.
  const connectedCount = useMemo(
    () => sorted.filter((r) => getConnectionState(r.validationStatus).label === "Connected").length,
    [sorted]
  );
  const disconnectedCount = sorted.length - connectedCount;

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Skeleton className="h-9 flex-1" />
          <Skeleton className="h-8 w-20" />
        </div>
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-4 rounded-[var(--radius-token-lg)] border border-[var(--border-subtle)] bg-[var(--surface-primary)] p-4"
            >
              <Skeleton className="h-12 w-12 rounded-[var(--radius-token-md)] shrink-0" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-48" />
                <Skeleton className="h-3 w-64" />
                <Skeleton className="h-3 w-40" />
              </div>
              <Skeleton className="h-6 w-24 rounded-full shrink-0" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  // Empty state — no repositories at all
  if (repositories.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted mb-5">
          <GitBranch className="h-8 w-8 text-muted-foreground" />
        </div>
        <h3 className="text-base font-semibold mb-1">No repositories yet</h3>
        <p className="text-sm text-muted-foreground mb-6 max-w-sm">
          Connect a Git repository to use it as a deployment source.
        </p>
        {canManage && onCreate && (
          <button
            onClick={onCreate}
            className="inline-flex items-center h-9 gap-2 rounded-[var(--radius-token-md)] px-4 text-sm font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97]"
          >
            <Plus className="h-4 w-4" />
            Add Repository
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Search + Create */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 min-w-[280px]">
          <Search className={filterSearchIconClasses} />
          <Input
            type="text"
            placeholder="Search repositories..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className={filterSearchClasses}
            aria-label="Search repositories"
          />
          {searchQuery && (
            <button
              type="button"
              onClick={() => setSearchQuery("")}
              className={filterClearButtonClasses}
              aria-label="Clear search"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        {canManage && onCreate && (
          <button
            onClick={onCreate}
            className="inline-flex items-center h-8 gap-1.5 rounded-[var(--radius-token-md)] px-2.5 text-xs font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97] shrink-0"
          >
            <Plus className="h-3 w-3" />
            Create
          </button>
        )}
      </div>

      {/* Card list (prototype-aligned; replaces the prior table) */}
      {sorted.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <GitBranch className="h-12 w-12 text-muted-foreground mb-4" />
          <p className="text-sm text-muted-foreground">
            No repositories match &ldquo;{searchQuery}&rdquo;
          </p>
        </div>
      ) : (
        <>
        <ul
          className="space-y-3 list-none"
          data-testid="repositories-list"
          aria-label="Repositories"
        >
          {sorted.map((repo) => {
            const connectedSince = formatConnectedSince(repo.createdAt);
            const { label: statusLabel, className: statusClassName } = getConnectionState(repo.validationStatus);
            const interactive = !!onEdit;
            return (
              // The card is promoted to role="button" with tabIndex=0 + Enter/
              // Space handlers when onEdit is set — the whole card is the click
              // target. jsx-a11y doesn't follow the role promotion on <li>, so
              // suppress the noninteractive-element-interactions warning.
              // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
              <li
                key={repo.id}
                data-testid={`repository-card-${repo.id}`}
                className={
                  "group relative flex items-center gap-4 rounded-[var(--radius-token-lg)] " +
                  "border border-[var(--border-subtle)] bg-[var(--surface-primary)] p-4 " +
                  "transition-colors duration-150 " +
                  (interactive
                    ? "cursor-pointer hover:border-[var(--border-hover)] hover:bg-[var(--surface-elevated)] focus-within:border-[var(--border-hover)]"
                    : "")
                }
                onClick={() => onEdit?.(repo)}
                role={interactive ? "button" : undefined}
                tabIndex={interactive ? 0 : undefined}
                aria-label={interactive ? `View details for ${repo.name}` : undefined}
                onKeyDown={(e) => {
                  if (interactive && (e.key === "Enter" || e.key === " ")) {
                    e.preventDefault();
                    onEdit?.(repo);
                  }
                }}
              >
                {/* Git-icon tile (teal on a muted background) */}
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[var(--radius-token-md)] bg-[rgba(45,212,191,0.08)] text-[var(--brand-primary)]">
                  <GitBranch className="h-5 w-5" aria-hidden="true" />
                </div>

                {/* Name + project tag, URL, connected-since metadata */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-foreground truncate">
                      {repo.name}
                    </span>
                    {repo.projectId && (
                      <Badge
                        variant="secondary"
                        className="text-[11px] font-normal text-muted-foreground bg-[rgba(255,255,255,0.05)] border-0"
                      >
                        {repo.projectId}
                      </Badge>
                    )}
                  </div>
                  <p className="mt-0.5 text-xs font-mono text-muted-foreground truncate">
                    {getRepositoryDisplayURL(repo)}
                  </p>
                  {connectedSince && (
                    <p className="mt-1 text-xs text-muted-foreground">
                      Connected since {connectedSince}
                    </p>
                  )}
                </div>

                {/* Status pill (right) */}
                <div className="flex items-center gap-2 shrink-0">
                  <Badge className={`text-xs ${statusClassName}`}>{statusLabel}</Badge>
                  {canManage && onDelete && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity"
                      aria-label={`Delete ${repo.name}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(repo.id);
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
        <div data-testid="repositories-list-footer">
          <ListFooter
            total={sorted.length}
            totalLabel="repositories"
            breakdown={[
              ["connected", connectedCount],
              ["disconnected", disconnectedCount],
            ]}
          />
        </div>
        </>
      )}
    </div>
  );
}
