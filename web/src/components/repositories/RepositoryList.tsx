// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import { GitBranch, Plus, Search, Trash2, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableHeader,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { SortableHead } from "@/components/ui/sortable-table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import type { RepositoryConfig } from "@/types/repository";
import { getRepositoryDisplayURL, getAuthTypeDisplayName } from "@/types/repository";

type SortField = "name" | "url" | "authType" | "project" | "status";
type SortDir = "asc" | "desc";

interface RepositoryListProps {
  repositories: RepositoryConfig[];
  onEdit?: (repo: RepositoryConfig) => void;
  onDelete?: (repoID: string) => void;
  onCreate?: () => void;
  canManage?: boolean;
  isLoadingPermission?: boolean;
  isLoading?: boolean;
}

function getStatusColor(status?: string) {
  switch (status) {
    case "valid": return "bg-status-success/10 text-status-success";
    case "invalid": return "bg-destructive/10 text-destructive";
    default: return "bg-secondary text-muted-foreground";
  }
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
      let aVal: string;
      let bVal: string;

      switch (sortField) {
        case "name":
          aVal = a.name.toLowerCase();
          bVal = b.name.toLowerCase();
          break;
        case "url":
          aVal = getRepositoryDisplayURL(a).toLowerCase();
          bVal = getRepositoryDisplayURL(b).toLowerCase();
          break;
        case "authType":
          aVal = a.authType || "";
          bVal = b.authType || "";
          break;
        case "project":
          aVal = a.projectId || "";
          bVal = b.projectId || "";
          break;
        case "status":
          aVal = a.validationStatus || "";
          bVal = b.validationStatus || "";
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [repositories, searchQuery, sortField, sortDir]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Skeleton className="h-9 flex-1" />
          <Skeleton className="h-8 w-20" />
        </div>
        <div className="rounded-lg border border-border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <th className="pl-4 w-[25%] p-3"><Skeleton className="h-4 w-12" /></th>
                <th className="w-[30%] p-3"><Skeleton className="h-4 w-10" /></th>
                <th className="w-[12%] p-3"><Skeleton className="h-4 w-10" /></th>
                <th className="w-[15%] p-3"><Skeleton className="h-4 w-12" /></th>
                <th className="w-[10%] p-3"><Skeleton className="h-4 w-10" /></th>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell className="pl-4">
                    <div className="flex items-center gap-3">
                      <Skeleton className="h-8 w-8 rounded-md" />
                      <Skeleton className="h-4 w-32" />
                    </div>
                  </TableCell>
                  <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
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
          Start adding repositories to enable deployment tracking and GitOps workflows.
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

      {/* Table */}
      {sorted.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <GitBranch className="h-12 w-12 text-muted-foreground mb-4" />
          <p className="text-sm text-muted-foreground">
            No repositories match &ldquo;{searchQuery}&rdquo;
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden animate-fade-in-up">
          <Table className="table-fixed">
            <TableHeader>
              <TableRow>
                <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[22%]">Name</SortableHead>
                <SortableHead field="url" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[32%]">URL</SortableHead>
                <SortableHead field="authType" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Auth</SortableHead>
                <SortableHead field="project" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[15%]">Project</SortableHead>
                <SortableHead field="status" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[10%]">Status</SortableHead>
                {canManage && <th className="w-[5%]" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {sorted.map((repo) => (
                <TableRow
                  key={repo.id}
                  className={onEdit ? "cursor-pointer" : ""}
                  onClick={() => onEdit?.(repo)}
                  role={onEdit ? "button" : undefined}
                  tabIndex={onEdit ? 0 : undefined}
                  aria-label={`View details for ${repo.name}`}
                  onKeyDown={(e) => {
                    if (onEdit && (e.key === "Enter" || e.key === " ")) {
                      e.preventDefault();
                      onEdit(repo);
                    }
                  }}
                >
                  <TableCell className="font-medium text-foreground truncate">{repo.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground truncate">
                    {getRepositoryDisplayURL(repo)} ({repo.defaultBranch})
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {repo.authType ? getAuthTypeDisplayName(repo.authType) : "—"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground truncate">{repo.projectId || "—"}</TableCell>
                  <TableCell>
                    {repo.validationStatus && (
                      <Badge className={`text-xs ${getStatusColor(repo.validationStatus)}`}>
                        {repo.validationStatus}
                      </Badge>
                    )}
                  </TableCell>
                  {canManage && (
                    <TableCell>
                      {onDelete && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          aria-label={`Delete ${repo.name}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            onDelete(repo.id);
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
