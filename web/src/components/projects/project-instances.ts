// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * project-instances — pure, client-side project→instance matching + stat
 * aggregation.
 *
 * The production wire format does NOT carry instance counts, health rollups, or
 * a member roster on a Project (see types/project.ts). Everything here is
 * DERIVED at render time from the real instance list (`useInstanceList`) by
 * matching an instance's namespace against the project's destination namespace
 * patterns. No wire-format change; no persisted field.
 *
 * Match semantics (documented per AC 12):
 *   - A project owns an instance iff `instance.namespace` matches ANY of the
 *     project's `destinations[].namespace` patterns.
 *   - Pattern matching mirrors the destination wildcard convention used across
 *     Knodex (types/project.ts Destination):
 *       "*"        → matches every (namespaced) instance
 *       "dev-*"    → prefix match: namespace starts with "dev-"
 *       "team-ns"  → exact match
 *   - Cluster-scoped instances (empty namespace) are NOT matched by namespace
 *     patterns — they have no namespace to scope into a project boundary.
 *   - The "issues" bucket merges Degraded + Unhealthy into one count, mirroring
 *     the 48.3 health-stripe precedent.
 *
 * Pagination caveat (AC 12): `useInstanceList` is server-paginated and its
 * `namespace` param is single-exact, so matching is done over the fetched page
 * client-side. Very large clusters may have instances beyond the first page;
 * a server-side "by project" filter would be a follow-up.
 */

import type { Instance } from "@/types/rgd";
import type { Project } from "@/types/project";

export interface ProjectNamespaceCount {
  /** The destination namespace pattern as declared on the project. */
  namespace: string;
  /** Count of fetched instances matching this specific pattern. */
  count: number;
}

export interface ProjectInstanceStats {
  /** Instances matched to the project (over the fetched instance page). */
  matched: Instance[];
  /** Total matched (== matched.length). */
  total: number;
  /** Count with health === "Healthy". */
  healthy: number;
  /** Count with health === "Progressing". */
  progressing: number;
  /** Merged Degraded + Unhealthy count (48.3 "issues" bucket). */
  issues: number;
  /** Per-destination-namespace match counts, in destination order. */
  byNamespace: ProjectNamespaceCount[];
}

/**
 * Returns true when an instance namespace matches a destination namespace
 * pattern. Supports "*" (all), trailing-"*" prefix globs, and exact match.
 */
export function namespaceMatchesPattern(
  namespace: string,
  pattern: string | undefined
): boolean {
  if (!pattern) return false;
  if (!namespace) return false; // cluster-scoped — no namespace to scope
  if (pattern === "*") return true;
  if (pattern.endsWith("*")) {
    return namespace.startsWith(pattern.slice(0, -1));
  }
  return namespace === pattern;
}

/**
 * Returns true when an instance belongs to a project by namespace membership.
 */
export function instanceMatchesProject(
  instance: Pick<Instance, "namespace">,
  project: Pick<Project, "destinations">
): boolean {
  const destinations = project.destinations ?? [];
  if (destinations.length === 0) return false;
  const ns = instance.namespace ?? "";
  return destinations.some((d) => namespaceMatchesPattern(ns, d.namespace));
}

/**
 * Aggregate derived instance stats for a single project over a fetched
 * instance list. Pure — safe to call inside useMemo.
 */
export function computeProjectInstanceStats(
  project: Pick<Project, "destinations">,
  instances: Instance[]
): ProjectInstanceStats {
  const matched = instances.filter((i) => instanceMatchesProject(i, project));

  let healthy = 0;
  let progressing = 0;
  let issues = 0;
  for (const i of matched) {
    if (i.health === "Healthy") healthy += 1;
    else if (i.health === "Progressing") progressing += 1;
    else if (i.health === "Degraded" || i.health === "Unhealthy") issues += 1;
  }

  const byNamespace: ProjectNamespaceCount[] = (project.destinations ?? []).map(
    (d) => {
      const pattern = d.namespace ?? "";
      const count = instances.filter((i) =>
        namespaceMatchesPattern(i.namespace ?? "", pattern)
      ).length;
      return { namespace: pattern || "(unnamed)", count };
    }
  );

  return {
    matched,
    total: matched.length,
    healthy,
    progressing,
    issues,
    byNamespace,
  };
}
