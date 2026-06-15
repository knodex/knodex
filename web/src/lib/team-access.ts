// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Team-centric access helpers.
 *
 * The project authorization model is role-centric: a Project has `roles[]`,
 * and each role binds N teams via `roles[].teams[]` (each team resolves to its
 * OIDC groups server-side — Story 10.2/10.6). The project "Access" tab presents
 * the *inverse* view: one row per Team, showing the role it is bound to and that
 * role's namespace scope. These pure helpers derive that team-centric view from
 * `roles[]` and translate team-centric edits back into a new `roles[]` array
 * (suitable for `updateProject`).
 *
 * Invariant enforced by the Access view: one role per team. A team that
 * (legacy) appears under multiple roles is surfaced via `roleNames.length > 1`
 * so the operator can consolidate it; selecting a role removes the team from
 * every other role.
 */
import type { ProjectRole } from "@/types/project";

export interface TeamBinding {
  /** Team name (cluster-scoped Team CRD name). */
  team: string;
  /** All role names that currently bind this team (length > 1 ⇒ needs consolidation). */
  roleNames: string[];
  /** The role surfaced in the Access row (first binding, in role order). */
  primaryRole: string;
  /** The primary role's namespace scope (empty ⇒ all namespaces). */
  destinations: string[];
}

/**
 * Build the team-centric binding list from a project's roles, sorted by team
 * name. Teams are collected in role order so `primaryRole` is deterministic.
 */
export function deriveTeamBindings(roles: ProjectRole[]): TeamBinding[] {
  const teamToRoles = new Map<string, string[]>();
  for (const role of roles) {
    for (const team of role.teams || []) {
      const arr = teamToRoles.get(team) || [];
      if (!arr.includes(role.name)) arr.push(role.name);
      teamToRoles.set(team, arr);
    }
  }

  const roleByName = new Map(roles.map((r) => [r.name, r]));

  return Array.from(teamToRoles.entries())
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([team, roleNames]) => {
      const primaryRole = roleNames[0];
      const role = roleByName.get(primaryRole);
      return {
        team,
        roleNames,
        primaryRole,
        destinations: role?.destinations ? [...role.destinations] : [],
      };
    });
}

/**
 * Return a new roles[] with `team` bound to exactly `targetRole`: added to the
 * target role (if absent) and removed from every other role. Idempotent.
 */
export function assignTeamToRole(
  roles: ProjectRole[],
  team: string,
  targetRole: string,
): ProjectRole[] {
  return roles.map((role) => {
    const teams = role.teams || [];
    const has = teams.includes(team);
    if (role.name === targetRole) {
      return has ? role : { ...role, teams: [...teams, team] };
    }
    return has ? { ...role, teams: teams.filter((t) => t !== team) } : role;
  });
}

/** Return a new roles[] with `team` removed from every role. */
export function removeTeamFromProject(
  roles: ProjectRole[],
  team: string,
): ProjectRole[] {
  return roles.map((role) => {
    const teams = role.teams || [];
    return teams.includes(team)
      ? { ...role, teams: teams.filter((t) => t !== team) }
      : role;
  });
}

/** All distinct team names currently bound anywhere in the project. */
export function boundTeamNames(roles: ProjectRole[]): string[] {
  const set = new Set<string>();
  for (const role of roles) for (const t of role.teams || []) set.add(t);
  return Array.from(set);
}

/**
 * Human label for a role's namespace scope. Empty destinations ⇒ project-wide
 * ("All namespaces"); otherwise the joined destination names.
 */
export function scopeLabel(destinations: string[]): string {
  if (!destinations || destinations.length === 0) return "All namespaces";
  if (destinations.length === 1) return destinations[0];
  return `${destinations.length} namespaces`;
}
