// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Access Tab — team-centric view of project authorization.
 *
 * Inverts the role-centric model (`roles[].teams[]`) into one row per Team,
 * each showing the team's OIDC-group provenance, its role (editable dropdown),
 * and the role's namespace scope. Edits translate back into a new `roles[]`
 * via the pure helpers in `lib/team-access` and persist through `updateProject`.
 *
 * Membership note: the API exposes each team's OIDC groups, not member
 * identities, so rows show "via N OIDC groups" rather than member avatars.
 */
import { useMemo, useState } from "react";
import { Users, Circle, ChevronDown, MoreVertical, Plus, AlertTriangle } from "@/lib/icons";
import { toast } from "sonner";
import { AxiosError } from "axios";
import { toUserFriendlyError } from "@/lib/errors";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
} from "@/components/ui/dropdown-menu";
import { avatarToneIndex } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";
import { useTeams } from "@/hooks/useTeams";
import type { Project, UpdateProjectRequest } from "@/types/project";
import type { Team } from "@/types/team";
import {
  deriveTeamBindings,
  assignTeamToRole,
  removeTeamFromProject,
  boundTeamNames,
  scopeLabel,
} from "@/lib/team-access";
import { AddTeamAccessDialog } from "../AddTeamAccessDialog";

interface ProjectAccessTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

/** Tone classes for the role pill, keyed by common role names. */
function roleToneClasses(roleName: string): string {
  const n = roleName.toLowerCase();
  if (n.includes("admin")) return "text-emerald-400 border-emerald-500/30 bg-emerald-500/10";
  if (n.includes("dev")) return "text-sky-400 border-sky-500/30 bg-sky-500/10";
  if (n.includes("read") || n.includes("view")) return "text-zinc-300 border-zinc-500/30 bg-zinc-500/10";
  return "text-primary border-primary/30 bg-primary/10";
}

export function ProjectAccessTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectAccessTabProps) {
  const roles = useMemo(() => project.roles || [], [project.roles]);
  const { data: teamsData } = useTeams();

  const teamsByName = useMemo(() => {
    const m = new Map<string, Team>();
    for (const t of teamsData?.items || []) m.set(t.name, t);
    return m;
  }, [teamsData]);

  const bindings = useMemo(() => deriveTeamBindings(roles), [roles]);

  const availableTeams = useMemo(() => {
    const bound = new Set(boundTeamNames(roles));
    return (teamsData?.items || []).filter((t) => !bound.has(t.name));
  }, [teamsData, roles]);

  const [showAdd, setShowAdd] = useState(false);

  const changeRole = async (team: string, role: string) => {
    try {
      await onUpdate({ roles: assignTeamToRole(roles, team, role) });
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      toast.error(toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to update role"
      ));
    }
  };

  const removeTeam = async (team: string) => {
    try {
      await onUpdate({ roles: removeTeamFromProject(roles, team) });
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      toast.error(toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to remove team"
      ));
    }
  };

  const addTeam = (team: string, role: string) =>
    onUpdate({ roles: assignTeamToRole(roles, team, role) });

  const headerAction = canManage && (
    <Button
      size="sm"
      data-testid="add-team-access-button"
      onClick={() => setShowAdd(true)}
      disabled={isUpdating}
    >
      <Plus className="h-3.5 w-3.5 mr-1.5" />
      Add team
    </Button>
  );

  const addDialog = (
    <AddTeamAccessDialog
      open={showAdd}
      onOpenChange={setShowAdd}
      availableTeams={availableTeams}
      hasAnyTeams={(teamsData?.items || []).length > 0}
      roles={roles}
      onAdd={addTeam}
      isSubmitting={isUpdating}
    />
  );

  if (bindings.length === 0) {
    return (
      <div className="space-y-4" data-testid="project-access-empty">
        <div className="py-12 text-center">
          <Users className="h-8 w-8 mx-auto mb-3 text-muted-foreground opacity-50" />
          <p className="text-sm font-medium text-foreground">No teams have access</p>
          <p className="text-xs text-muted-foreground mt-1 max-w-sm mx-auto">
            Grant a team access to bind its OIDC groups to a role in this project.
          </p>
          {canManage && (
            <Button
              size="sm"
              className="mt-4"
              data-testid="add-team-access-button"
              onClick={() => setShowAdd(true)}
              disabled={isUpdating}
            >
              <Plus className="h-3.5 w-3.5 mr-1.5" />
              Add team
            </Button>
          )}
        </div>
        {addDialog}
      </div>
    );
  }

  return (
    <div className="space-y-4" data-testid="project-access-tab">
      {/* Section header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h3 className="text-sm font-medium text-foreground">Team access</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            {bindings.length} team{bindings.length !== 1 ? "s" : ""} {bindings.length !== 1 ? "have" : "has"} access to this project. Membership resolves from each team&apos;s OIDC groups.
          </p>
        </div>
        {headerAction}
      </div>

      {/* Team rows */}
      <div className="rounded-lg border border-border divide-y divide-border overflow-hidden">
        {bindings.map((binding) => {
          const team = teamsByName.get(binding.team);
          const groupCount = team?.oidcGroups?.length ?? 0;
          const tone = avatarToneIndex(binding.team);
          const multiRole = binding.roleNames.length > 1;

          return (
            <div
              key={binding.team}
              data-testid={`team-access-row-${binding.team}`}
              className="flex items-center gap-4 px-4 py-4"
            >
              {/* Team tile */}
              <span
                aria-hidden="true"
                className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl"
                style={{
                  backgroundColor: `hsl(var(--avatar-tone-${tone}-bg-hsl) / 0.18)`,
                  color: `var(--avatar-tone-${tone}-fg)`,
                }}
              >
                <Users className="h-5 w-5" />
              </span>

              {/* Name + provenance */}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-foreground truncate">
                    {binding.team}
                  </span>
                  {!team && (
                    <span className="text-xs text-amber-400" title="No matching Team resource found">
                      team not found
                    </span>
                  )}
                  {multiRole && (
                    <span
                      className="inline-flex items-center gap-1 text-xs text-amber-400"
                      title={`Bound to multiple roles: ${binding.roleNames.join(", ")}. Pick one to consolidate.`}
                    >
                      <AlertTriangle className="h-3 w-3" />
                      multiple roles
                    </span>
                  )}
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">
                  via {groupCount} OIDC group{groupCount !== 1 ? "s" : ""}
                </p>
              </div>

              {/* Namespace scope */}
              <div className="hidden sm:flex items-center gap-1.5 text-sm text-muted-foreground min-w-[140px]">
                <Circle className="h-3 w-3 opacity-60" />
                <span className="truncate">{scopeLabel(binding.destinations)}</span>
              </div>

              {/* Role pill / dropdown */}
              {canManage ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      data-testid={`team-role-${binding.team}`}
                      disabled={isUpdating}
                      className={cn(
                        "inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-sm font-medium transition-colors disabled:opacity-50",
                        roleToneClasses(binding.primaryRole),
                      )}
                    >
                      {binding.primaryRole}
                      <ChevronDown className="h-3.5 w-3.5 opacity-70" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuLabel>Role</DropdownMenuLabel>
                    <DropdownMenuRadioGroup
                      value={binding.primaryRole}
                      onValueChange={(role) => {
                        if (role !== binding.primaryRole || multiRole) changeRole(binding.team, role);
                      }}
                    >
                      {roles.map((r) => (
                        <DropdownMenuRadioItem
                          key={r.name}
                          value={r.name}
                          data-testid={`role-option-${binding.team}-${r.name}`}
                        >
                          {r.name}
                        </DropdownMenuRadioItem>
                      ))}
                    </DropdownMenuRadioGroup>
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : (
                <span
                  className={cn(
                    "inline-flex items-center rounded-md border px-2.5 py-1 text-sm font-medium",
                    roleToneClasses(binding.primaryRole),
                  )}
                >
                  {binding.primaryRole}
                </span>
              )}

              {/* Overflow menu */}
              {canManage && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      data-testid={`team-access-menu-${binding.team}`}
                      disabled={isUpdating}
                      aria-label={`Manage access for ${binding.team}`}
                      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground transition-colors disabled:opacity-50"
                    >
                      <MoreVertical className="h-4 w-4" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      data-testid={`remove-team-${binding.team}`}
                      onClick={() => removeTeam(binding.team)}
                    >
                      Remove from project
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem asChild>
                      <a href="/settings/teams">Manage teams…</a>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
          );
        })}
      </div>

      {addDialog}
    </div>
  );
}
