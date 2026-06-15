// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * ProjectCard — grid card for the Projects list.
 *
 * Visuals (AC 2/3): a 44px rounded icon whose hue is deterministic per project
 * via `avatarToneIndex(project.name)` over the shared 7-tone avatar token
 * palette (presentation only — there is NO persisted `color` field), the
 * project name, a 2-line description clamp, a hover `CardChevron`, and a footer
 * of DERIVED stats: `{instances} instances · {roles} roles` plus an amber
 * `{N} issues` pill (the `--status-warning` token) when the project has any
 * Degraded/Unhealthy instances. No `lead` avatar is rendered (no field).
 */
import { Package, Trash2 } from "@/lib/icons";
import { avatarInitials, avatarToneIndex } from "@/components/ui/avatar";
import { CardChevron } from "@/components/ui/card-chevron";
import { Button } from "@/components/ui/button";
import type { Project } from "@/types/project";
import type { ProjectInstanceStats } from "./project-instances";

interface ProjectCardProps {
  project: Project;
  stats?: ProjectInstanceStats;
  onDelete?: (projectName: string) => void;
  onClick?: (project: Project) => void;
  canManage?: boolean;
}

export function ProjectCard({
  project,
  stats,
  onDelete,
  onClick,
  canManage = false,
}: ProjectCardProps) {
  const roleCount = project.roles?.length ?? 0;
  const instanceCount = stats?.total ?? 0;
  const issueCount = stats?.issues ?? 0;
  const tone = avatarToneIndex(project.name);

  return (
    <div
      role="button"
      tabIndex={0}
      data-testid="project-card"
      aria-label={`View details for ${project.name}`}
      className="group relative flex flex-col rounded-lg border border-border bg-card p-4 text-left transition-colors hover:bg-secondary/50 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand-primary)]"
      onClick={() => onClick?.(project)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick?.(project);
        }
      }}
    >
      <CardChevron />

      {/* Header: tone icon + name/description */}
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          data-tone={tone}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-sm font-semibold select-none"
          style={{
            backgroundColor: `hsl(var(--avatar-tone-${tone}-bg-hsl) / 0.18)`,
            color: `var(--avatar-tone-${tone}-fg)`,
          }}
        >
          {avatarInitials(project.name)}
        </span>
        <div className="min-w-0 flex-1 pr-5">
          <p className="font-medium text-foreground truncate">{project.name}</p>
          {project.description ? (
            <p className="text-xs text-muted-foreground line-clamp-2 mt-0.5">
              {project.description}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground/70 italic mt-0.5">
              No description
            </p>
          )}
        </div>
      </div>

      {/* Footer: derived stats */}
      <div className="mt-4 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs text-muted-foreground min-w-0">
          <span className="inline-flex items-center gap-1 whitespace-nowrap">
            <Package className="h-3.5 w-3.5" />
            <span className="text-foreground font-medium">{instanceCount}</span>{" "}
            {instanceCount === 1 ? "instance" : "instances"}
          </span>
          <span aria-hidden="true" className="opacity-60">
            ·
          </span>
          <span className="whitespace-nowrap">
            <span className="text-foreground font-medium">{roleCount}</span>{" "}
            {roleCount === 1 ? "role" : "roles"}
          </span>
          {issueCount > 0 && (
            <span
              className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-medium whitespace-nowrap"
              style={{
                color: "var(--status-warning)",
                backgroundColor: "hsl(var(--status-warning-hsl) / 0.12)",
              }}
            >
              {issueCount} {issueCount === 1 ? "issue" : "issues"}
            </span>
          )}
        </div>

        {canManage && onDelete && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
            aria-label={`Delete ${project.name}`}
            onClick={(e) => {
              e.stopPropagation();
              onDelete(project.name);
            }}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  );
}
