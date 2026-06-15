// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { FolderOpen, Plus, Search, X } from "@/lib/icons";
import { useState, useMemo } from "react";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ListFooter } from "@/components/ui/list-footer";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import type { Project } from "@/types/project";
import type { Instance } from "@/types/rgd";
import { ProjectCard } from "./ProjectCard";
import {
  computeProjectInstanceStats,
  type ProjectInstanceStats,
} from "./project-instances";

interface ProjectListProps {
  projects: Project[];
  /** Fetched instance list — derived per-project stats are computed from this. */
  instances?: Instance[];
  onDelete?: (projectName: string) => void;
  onClick?: (project: Project) => void;
  onCreate?: () => void;
  canManage?: boolean;
  isLoading?: boolean;
}

export function ProjectList({
  projects,
  instances = [],
  onDelete,
  onClick,
  onCreate,
  canManage = false,
  isLoading = false,
}: ProjectListProps) {
  const [searchQuery, setSearchQuery] = useState("");

  // Derived per-project instance stats (presentation-only; no wire change).
  const statsByProject = useMemo(() => {
    const map = new Map<string, ProjectInstanceStats>();
    for (const p of projects) {
      map.set(p.name, computeProjectInstanceStats(p, instances));
    }
    return map;
  }, [projects, instances]);

  // Filtered + alphabetically sorted (preserves prior default name-asc order).
  const visible = useMemo(() => {
    let items = projects;
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      items = items.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.description?.toLowerCase().includes(q)
      );
    }
    return [...items].sort((a, b) =>
      a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    );
  }, [projects, searchQuery]);

  // Footer counts over the FILTERED/visible set (48.6 visible-rows pitfall).
  const footer = useMemo(() => {
    let totalInstances = 0;
    let totalRoles = 0;
    for (const p of visible) {
      totalInstances += statsByProject.get(p.name)?.total ?? 0;
      totalRoles += p.roles?.length ?? 0;
    }
    return { totalInstances, totalRoles };
  }, [visible, statsByProject]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Skeleton className="h-9 flex-1" />
          <Skeleton className="h-8 w-20" />
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="rounded-lg border border-border p-4">
              <div className="flex items-start gap-3">
                <Skeleton className="h-11 w-11 rounded-xl" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3 w-full" />
                </div>
              </div>
              <Skeleton className="h-3 w-40 mt-4" />
            </div>
          ))}
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
            New project
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Search + Create on same row. Search is bounded (was `flex-1` =
          full-row); the action button sits immediately to its right rather
          than at the far edge — closer to a typical filter-bar feel. */}
      <div className="flex items-center gap-2">
        <div className="relative w-72">
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
            New project
          </button>
        )}
      </div>

      {/* Card grid */}
      {visible.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
          <p className="text-sm text-muted-foreground">
            No projects match &ldquo;{searchQuery}&rdquo;
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {visible.map((project) => (
              <ProjectCard
                key={project.name}
                project={project}
                stats={statsByProject.get(project.name)}
                onClick={onClick}
                onDelete={onDelete}
                canManage={canManage}
              />
            ))}
          </div>
          <ListFooter
            data-testid="projects-list-footer"
            total={visible.length}
            totalLabel={visible.length === 1 ? "project" : "projects"}
            breakdown={[
              ["instances", footer.totalInstances],
              ["roles", footer.totalRoles],
            ]}
          />
        </>
      )}
    </div>
  );
}
