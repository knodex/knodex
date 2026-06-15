// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Overview Tab — derived StatCards + recent-instances/namespaces
 * 2-column block (AC 8/9), above the preserved Vercel/Dokploy-style flat
 * property rows with inline description editing.
 *
 * All instance figures are DERIVED client-side from the fetched instance list
 * by namespace membership (project-instances.ts) — no wire-format change.
 */
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Edit,
  Loader2,
  MapPin,
  Package,
  Save,
  Users,
  X,
} from "@/lib/icons";
import { toast } from "sonner";
import { AxiosError } from "axios";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { StatCard } from "@/components/ui/stat-card";
import { STRIPE_COLOR } from "@/components/instances/instance-utils";
import { buildInstanceRoute } from "@/lib/instancePath";
import { formatDistanceToNow } from "@/lib/date";
import { toUserFriendlyError } from "@/lib/errors";
import type { Instance } from "@/types/rgd";
import type { Project, UpdateProjectRequest } from "@/types/project";
import { computeProjectInstanceStats } from "../project-instances";

interface ProjectOverviewTabProps {
  project: Project;
  instances: Instance[];
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

const RECENT_LIMIT = 5;

function PropertyRow({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`flex items-center justify-between py-3 border-b border-border ${className || ""}`}>
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm text-foreground">{children}</span>
    </div>
  );
}

export function ProjectOverviewTab({
  project,
  instances,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectOverviewTabProps) {
  const navigate = useNavigate();
  const [isEditing, setIsEditing] = useState(false);
  const [description, setDescription] = useState(project.description || "");

  const stats = useMemo(
    () => computeProjectInstanceStats(project, instances),
    [project, instances]
  );

  const recent = useMemo(
    () =>
      [...stats.matched]
        .sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
        .slice(0, RECENT_LIMIT),
    [stats.matched]
  );

  const handleSave = async () => {
    try {
      await onUpdate({ description });
      setIsEditing(false);
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      const errorMessage = toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to save project"
      );
      toast.error(errorMessage);
    }
  };

  const handleCancel = () => {
    setDescription(project.description || "");
    setIsEditing(false);
  };

  const roleCount = project.roles?.length || 0;
  const destCount = project.destinations?.length || 0;

  return (
    <div className="space-y-8">
      {/* Derived stat cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Instances" value={stats.total} icon={Package} />
        <StatCard title="Healthy" value={stats.healthy} icon={CheckCircle2} />
        <StatCard title="Progressing" value={stats.progressing} icon={Activity} />
        <StatCard
          title="Issues"
          value={stats.issues}
          icon={AlertTriangle}
          variant={stats.issues > 0 ? "warning" : "default"}
        />
      </div>

      {/* 2-column: recent instances + namespaces */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Recent instances */}
        <section>
          <h3 className="text-sm font-medium text-foreground mb-3">Recent instances</h3>
          {recent.length === 0 ? (
            <p className="text-sm text-muted-foreground py-6 text-center border border-dashed border-border rounded-lg">
              No instances in this project&rsquo;s namespaces.
            </p>
          ) : (
            <div className="rounded-lg border border-border overflow-hidden">
              <table className="w-full text-sm">
                <tbody>
                  {recent.map((instance) => (
                    <tr
                      key={`${instance.apiVersion}/${instance.namespace || "_cluster"}/${instance.kind}/${instance.name}`}
                      role="button"
                      tabIndex={0}
                      aria-label={`View details for ${instance.name}`}
                      className="group cursor-pointer border-b border-border last:border-b-0 hover:bg-secondary/50 focus-visible:outline-none focus-visible:bg-secondary/50"
                      onClick={() => navigate(buildInstanceRoute(instance))}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          navigate(buildInstanceRoute(instance));
                        }
                      }}
                    >
                      <td
                        className="w-1"
                        data-testid="instance-row-health-stripe"
                        data-health={instance.health}
                        aria-label={`Health: ${instance.health}`}
                        style={{
                          boxShadow: `inset 3px 0 0 ${STRIPE_COLOR[instance.health] ?? "var(--status-inactive)"}`,
                        }}
                      />
                      <td className="px-3 py-2 min-w-0">
                        <p className="font-medium text-foreground truncate">{instance.name}</p>
                        <p className="text-xs text-muted-foreground truncate">
                          {instance.kind}
                          {instance.namespace ? ` · ${instance.namespace}` : ""}
                        </p>
                      </td>
                      <td className="px-3 py-2 text-right text-xs text-muted-foreground whitespace-nowrap">
                        {formatDistanceToNow(instance.createdAt)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        {/* Namespaces with instance counts */}
        <section>
          <h3 className="text-sm font-medium text-foreground mb-3">Namespaces</h3>
          {stats.byNamespace.length === 0 ? (
            <p className="text-sm text-muted-foreground py-6 text-center border border-dashed border-border rounded-lg">
              No destination namespaces configured.
            </p>
          ) : (
            <ul className="rounded-lg border border-border divide-y divide-border">
              {stats.byNamespace.map((ns, i) => (
                <li
                  key={`${ns.namespace}-${i}`}
                  className="flex items-center justify-between px-3 py-2 text-sm"
                >
                  <span className="inline-flex items-center gap-2 min-w-0">
                    <MapPin className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <code className="font-mono text-xs truncate">{ns.namespace}</code>
                  </span>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    <span className="text-foreground font-medium">{ns.count}</span>{" "}
                    {ns.count === 1 ? "instance" : "instances"}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {/* Project Properties (preserved) */}
      <section>
        <div className="flex items-center justify-between mb-1">
          <h3 className="text-sm font-medium text-foreground">Project Details</h3>
          {canManage && !isEditing && (
            <Button variant="ghost" size="sm" onClick={() => setIsEditing(true)} className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground">
              <Edit className="h-3 w-3 mr-1" />
              Edit
            </Button>
          )}
          {isEditing && (
            <div className="flex gap-1.5">
              <Button variant="ghost" size="sm" onClick={handleCancel} disabled={isUpdating} className="h-7 px-2 text-xs">
                <X className="h-3 w-3 mr-1" />
                Cancel
              </Button>
              <Button size="sm" onClick={handleSave} disabled={isUpdating} className="h-7 px-2 text-xs">
                {isUpdating ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <Save className="h-3 w-3 mr-1" />}
                Save
              </Button>
            </div>
          )}
        </div>

        <div className="border-t border-border">
          <PropertyRow label="Name">
            <code className="font-mono text-xs px-1.5 py-0.5 rounded bg-secondary">{project.name}</code>
          </PropertyRow>

          <div className={`flex ${isEditing ? "flex-col gap-2 py-3" : "items-center justify-between py-3"} border-b border-border`}>
            <span className="text-sm text-muted-foreground">Description</span>
            {isEditing ? (
              <Textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Add a description..."
                rows={2}
                className="text-sm"
              />
            ) : (
              <span className="text-sm text-foreground max-w-[400px] text-right">
                {project.description || <span className="text-muted-foreground italic">No description</span>}
              </span>
            )}
          </div>

          <PropertyRow label="Roles">
            <span className="inline-flex items-center gap-1.5">
              <Users className="h-3 w-3 text-muted-foreground" />
              {roleCount}
            </span>
          </PropertyRow>

          <PropertyRow label="Destinations" className="border-b-0">
            <span className="inline-flex items-center gap-1.5">
              <MapPin className="h-3 w-3 text-muted-foreground" />
              {destCount}
            </span>
          </PropertyRow>
        </div>
      </section>
    </div>
  );
}
