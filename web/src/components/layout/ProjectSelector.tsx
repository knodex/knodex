// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo } from "react";
import { FolderOpen } from "@/lib/icons";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useUserStore } from "@/stores/userStore";
import { useProjects } from "@/hooks/useProjects";
import { cn } from "@/lib/utils";

const ALL_PROJECTS_VALUE = "__all_projects__";

export function ProjectSelector() {
  const currentProject = useUserStore((s) => s.currentProject);
  const setCurrentProject = useUserStore((s) => s.setCurrentProject);
  const isAuthenticated = useUserStore((s) => s.isAuthenticated);

  // Use API-fetched projects (RBAC-aware — admins see all projects)
  const { data: projectsData, isLoading } = useProjects();
  const projects = useMemo(() => projectsData?.items?.map((p) => p.name) ?? [], [projectsData]);

  const handleChange = useCallback(
    (value: string) => {
      setCurrentProject(value === ALL_PROJECTS_VALUE ? null : value);
    },
    [setCurrentProject]
  );

  if (!isAuthenticated) return null;

  return (
    <Select
      value={currentProject ?? ALL_PROJECTS_VALUE}
      onValueChange={handleChange}
      disabled={isLoading}
    >
      <SelectTrigger
        className={cn(
          "h-8 w-auto min-w-[140px] max-w-[200px] gap-1.5 border-[var(--border-subtle)] bg-transparent px-2.5",
          "text-[var(--text-size-sm)] hover:border-[var(--border-hover)] transition-colors duration-150",
          "focus:ring-1 focus:ring-[var(--brand-primary)]/30",
          currentProject
            ? "text-foreground font-medium"
            : "text-muted-foreground"
        )}
        aria-label="Select project"
        data-testid="project-selector"
      >
        <FolderOpen className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <SelectValue placeholder="All Projects" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_PROJECTS_VALUE}>All Projects</SelectItem>
        {projects.map((project) => (
          <SelectItem key={project} value={project}>
            {project}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
