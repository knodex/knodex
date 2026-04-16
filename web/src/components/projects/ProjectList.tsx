// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { FolderOpen, Plus, Search, Trash2, X } from "@/lib/icons";
import { useState, useMemo, useCallback } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableHeader,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { SortableHead } from "@/components/ui/sortable-table";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import { formatDistanceToNow } from "@/lib/date";
import { Skeleton } from "@/components/ui/skeleton";
import type { Project } from "@/types/project";

type SortField = "name" | "roles" | "destinations" | "createdAt";
type SortDir = "asc" | "desc";

interface ProjectListProps {
  projects: Project[];
  onEdit?: (project: Project) => void;
  onDelete?: (projectName: string) => void;
  onClick?: (project: Project) => void;
  onCreate?: () => void;
  canManage?: boolean;
  isLoading?: boolean;
}

export function ProjectList({
  projects,
  onDelete,
  onClick,
  onCreate,
  canManage = false,
  isLoading = false,
}: ProjectListProps) {
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
    let items = projects;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      items = items.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.description?.toLowerCase().includes(q)
      );
    }

    return [...items].sort((a, b) => {
      let aVal: string | number;
      let bVal: string | number;

      switch (sortField) {
        case "name":
          aVal = a.name.toLowerCase();
          bVal = b.name.toLowerCase();
          break;
        case "roles":
          aVal = a.roles?.length ?? 0;
          bVal = b.roles?.length ?? 0;
          break;
        case "destinations":
          aVal = a.destinations?.length ?? 0;
          bVal = b.destinations?.length ?? 0;
          break;
        case "createdAt":
          aVal = a.createdAt;
          bVal = b.createdAt;
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [projects, searchQuery, sortField, sortDir]);

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
                <th className="pl-4 w-[35%] p-3"><Skeleton className="h-4 w-12" /></th>
                <th className="w-[10%] p-3"><Skeleton className="h-4 w-10" /></th>
                <th className="w-[15%] p-3"><Skeleton className="h-4 w-16" /></th>
                <th className="w-[20%] p-3 text-right"><Skeleton className="h-4 w-14 ml-auto" /></th>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell className="pl-4">
                    <div className="flex items-center gap-3">
                      <Skeleton className="h-8 w-8 rounded-md" />
                      <div className="space-y-1">
                        <Skeleton className="h-4 w-32" />
                        <Skeleton className="h-3 w-48" />
                      </div>
                    </div>
                  </TableCell>
                  <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                  <TableCell className="text-right"><Skeleton className="h-4 w-20 ml-auto" /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
    );
  }

  // Empty state — no projects at all
  if (projects.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted mb-5">
          <FolderOpen className="h-8 w-8 text-muted-foreground" />
        </div>
        <h3 className="text-base font-semibold mb-1">No projects yet</h3>
        <p className="text-sm text-muted-foreground mb-6 max-w-sm">
          Start adding projects to organize your teams, namespaces, and access policies.
        </p>
        {canManage && onCreate && (
          <button
            onClick={onCreate}
            className="inline-flex items-center h-9 gap-2 rounded-[var(--radius-token-md)] px-4 text-sm font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97]"
          >
            <Plus className="h-4 w-4" />
            Create Project
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Search + Create on same row */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 min-w-[280px]">
          <Search className={filterSearchIconClasses} />
          <Input
            type="text"
            placeholder="Search projects..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className={filterSearchClasses}
            aria-label="Search projects"
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
          <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
          <p className="text-sm text-muted-foreground">
            No projects match &ldquo;{searchQuery}&rdquo;
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden animate-fade-in-up">
          <Table className="table-fixed">
            <TableHeader>
              <TableRow>
                <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[40%]">Name</SortableHead>
                <SortableHead field="roles" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[12%]">Roles</SortableHead>
                <SortableHead field="destinations" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[15%]">Destinations</SortableHead>
                <SortableHead field="createdAt" sortField={sortField} sortDir={sortDir} onSort={handleSort} className="w-[20%]">Created</SortableHead>
                {canManage && <th className="w-[5%]" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {sorted.map((project) => (
                <TableRow
                  key={project.name}
                  className="cursor-pointer"
                  onClick={() => onClick?.(project)}
                  role="button"
                  tabIndex={0}
                  aria-label={`View details for ${project.name}`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onClick?.(project);
                    }
                  }}
                >
                  <TableCell>
                    <div className="min-w-0">
                      <p className="font-medium text-foreground truncate">{project.name}</p>
                      {project.description && (
                        <p className="text-xs text-muted-foreground truncate">{project.description}</p>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {project.roles?.length ?? 0}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {project.destinations?.length ?? 0}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatDistanceToNow(project.createdAt)}
                  </TableCell>
                  {canManage && (
                    <TableCell>
                      {onDelete && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          aria-label={`Delete ${project.name}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            onDelete(project.name);
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
