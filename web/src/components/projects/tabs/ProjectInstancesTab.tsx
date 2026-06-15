// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * ProjectInstancesTab — full instances table scoped to the project's
 * destination namespaces (AC 11/12). Reuses `InstancesListView` for the 3px
 * health-stripe rows; clicking a row cross-links to the GLOBAL instance detail
 * route (AC 13) — no per-project instance route is introduced.
 *
 * Scoping is client-side namespace membership over the fetched instance page
 * (see project-instances.ts). Large clusters may paginate beyond this page.
 */
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { Package } from "@/lib/icons";
import { InstancesListView } from "@/components/instances/InstancesListView";
import { buildInstanceRoute } from "@/lib/instancePath";
import type { Instance } from "@/types/rgd";
import type { Project } from "@/types/project";
import { computeProjectInstanceStats } from "../project-instances";

interface ProjectInstancesTabProps {
  project: Project;
  instances: Instance[];
}

export function ProjectInstancesTab({
  project,
  instances,
}: ProjectInstancesTabProps) {
  const navigate = useNavigate();

  const matched = useMemo(
    () => computeProjectInstanceStats(project, instances).matched,
    [project, instances]
  );

  if (matched.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <Package className="h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-sm font-medium text-foreground">No instances yet</p>
        <p className="text-sm text-muted-foreground mt-1 max-w-sm">
          Instances deployed into this project&rsquo;s namespaces will appear here.
        </p>
      </div>
    );
  }

  return (
    <InstancesListView
      items={matched}
      onInstanceClick={(instance) => navigate(buildInstanceRoute(instance))}
    />
  );
}
