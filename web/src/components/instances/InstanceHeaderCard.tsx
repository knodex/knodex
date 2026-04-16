// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { RGDIcon } from "@/components/ui/rgd-icon";
import { ScopeIndicator } from "@/components/shared/ScopeIndicator";
import { GitBranch } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { Link } from "react-router-dom";
import type { Instance, InstanceHealth } from "@/types/rgd";

const HEALTH_COLOR: Record<InstanceHealth, string> = {
  Healthy: "var(--status-healthy)",
  Degraded: "var(--status-warning)",
  Unhealthy: "var(--status-error)",
  Progressing: "var(--status-info)",
  Unknown: "var(--status-inactive)",
};

/** Label/value row */
function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-3" style={{ borderBottom: "1px solid var(--border-subtle)" }}>
      <span className="text-sm text-[var(--text-muted)]">{label}</span>
      <span className="text-sm text-[var(--text-primary)]">{children}</span>
    </div>
  );
}

interface InstanceHeaderCardProps {
  instance: Instance;
  parentRGD?: { description?: string; lastIssuedRevision?: number; labels?: Record<string, string> };
  canReadRGD: boolean;
  kroState: string;
  onRevisionClick: () => void;
}

export function InstanceHeaderCard({
  instance,
  parentRGD,
  canReadRGD,
  kroState,
  onRevisionClick,
}: InstanceHeaderCardProps) {

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr] gap-0">
      {/* Left: Icon + name + description */}
      <div className="px-6 py-5 flex items-start gap-4" style={{ borderBottom: "1px solid var(--border-subtle)" }}>
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-secondary text-muted-foreground">
          <RGDIcon icon={instance.rgdIcon} category={instance.rgdCategory || "uncategorized"} className="h-6 w-6" />
        </div>
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-[var(--text-primary)] truncate">
            {instance.name}
          </h1>
          {parentRGD?.description && (
            <p className="text-sm text-[var(--text-muted)] mt-0.5 line-clamp-1">{parentRGD.description}</p>
          )}
        </div>
      </div>

      {/* Right: Key answers — Status, Kind, Namespace/Scope */}
      <div className="px-6 py-2 lg:border-l" style={{ borderBottom: "1px solid var(--border-subtle)", borderLeftColor: "var(--border-subtle)" }}>
        <DetailRow label="Status">
          <span className="inline-flex items-center gap-1.5">
            <span
              className={cn("inline-block h-2 w-2 rounded-full", instance.health === "Progressing" && "animate-status-pulse")}
              style={{ background: HEALTH_COLOR[instance.health] ?? "var(--status-inactive)" }}
            />
            <span className="font-medium">{instance.health}</span>
            {kroState && kroState !== "ACTIVE" && (
              <span className="text-xs text-[var(--text-muted)] font-mono ml-1">{kroState}</span>
            )}
          </span>
        </DetailRow>
        <DetailRow label="Kind">
          <span className="inline-flex items-center gap-2">
            <Link to={`/catalog/${encodeURIComponent(instance.rgdName)}`} className="font-mono text-primary hover:underline">
              {instance.kind}
            </Link>
            {canReadRGD && parentRGD?.lastIssuedRevision ? (
              <button
                type="button"
                onClick={onRevisionClick}
                aria-label={`View changes for revision ${parentRGD.lastIssuedRevision}`}
                className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium text-muted-foreground hover:text-primary hover:border-primary/50 transition-colors cursor-pointer"
              >
                <GitBranch className="h-3 w-3" />
                Rev {parentRGD.lastIssuedRevision}
              </button>
            ) : null}
          </span>
        </DetailRow>
        {!instance.isClusterScoped && instance.namespace && (
          <DetailRow label="Namespace">
            <span className="font-mono">{instance.namespace}</span>
          </DetailRow>
        )}
        {instance.isClusterScoped && (
          <DetailRow label="Scope">
            <ScopeIndicator isClusterScoped namespace="" variant="inline" />
          </DetailRow>
        )}
      </div>
    </div>
  );
}
