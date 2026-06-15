// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { RGDIcon } from "@/components/ui/rgd-icon";
import { HealthBadge } from "./HealthBadge";
import { InstanceMetadataSection } from "./InstanceMetadataSection";
import { GitBranch } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { formatDistanceToNow } from "@/lib/date";
import { Link } from "react-router-dom";
import type { Instance } from "@/types/rgd";
import type { InstanceTabId } from "./hooks/useInstanceTabs";

function MetaChip({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-2 rounded-md border border-[var(--border-subtle)] bg-[var(--surface-bg)] px-2.5 py-1.5">
      <span className="text-[11px] text-[var(--text-muted)]">{label}</span>
      {children}
    </span>
  );
}

export interface InstanceRollup {
  conditionsPassing: number;
  conditionsTotal: number;
  resourcesReady: number;
  resourcesTotal: number;
  resourcesFailing: number;
  eventsCount: number;
  eventsWarnings: number;
  lastReconciled?: string;
}

interface RollupCellProps {
  label: string;
  value: React.ReactNode;
  sub?: string;
  tone?: "ok" | "warn" | "neutral";
  onClick?: () => void;
}

function RollupCell({ label, value, sub, tone = "neutral", onClick }: RollupCellProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col items-start gap-1 px-6 py-3.5 text-left border-r last:border-r-0 border-[var(--border-subtle)] hover:bg-[var(--bg-hover,rgba(255,255,255,0.015))] transition-colors"
    >
      <span className="text-[11px] uppercase tracking-wider font-medium text-[var(--text-muted)]">
        {label}
      </span>
      <span
        className={cn(
          "flex items-baseline gap-2 text-[15px] font-semibold text-[var(--text-primary)]",
          tone === "ok" && "text-[var(--status-healthy)]",
          tone === "warn" && "text-[var(--status-error)]"
        )}
      >
        {value}
        {sub && <span className="text-xs font-normal text-[var(--text-secondary)]">{sub}</span>}
      </span>
    </button>
  );
}

interface InstanceHeaderCardProps {
  instance: Instance;
  parentRGD?: { description?: string; lastIssuedRevision?: number; labels?: Record<string, string> };
  canReadRGD: boolean;
  kroState: string;
  isGitOps: boolean;
  rollup: InstanceRollup;
  actions: React.ReactNode;
  onRevisionClick: () => void;
  onSelectTab: (tab: InstanceTabId) => void;
}

export function InstanceHeaderCard({
  instance,
  parentRGD,
  canReadRGD,
  kroState,
  isGitOps,
  rollup,
  actions,
  onRevisionClick,
  onSelectTab,
}: InstanceHeaderCardProps) {
  const allConditionsPassing =
    rollup.conditionsTotal > 0 && rollup.conditionsPassing === rollup.conditionsTotal;
  const allResourcesReady =
    rollup.resourcesTotal > 0 && rollup.resourcesReady === rollup.resourcesTotal;

  return (
    <div
      className="rounded-lg border overflow-hidden"
      style={{ borderColor: "var(--border-default)", background: "var(--surface-primary)" }}
      data-testid="instance-header-card"
    >
      {/* Identity + actions */}
      <div className="flex items-start gap-4 px-6 pt-5 pb-4">
        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-[var(--border-subtle)] bg-[var(--surface-elevated)]">
          <RGDIcon icon={instance.rgdIcon} category={instance.rgdCategory || "uncategorized"} className="h-6 w-6" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-xl font-semibold text-[var(--text-primary)] truncate">
              {instance.name}
            </h1>
            <HealthBadge health={instance.health} size="sm" />
            {kroState && kroState !== "ACTIVE" && (
              <span className="text-xs text-[var(--text-muted)] font-mono">{kroState}</span>
            )}
          </div>
          {parentRGD?.description && (
            <p className="text-sm text-[var(--text-secondary)] mt-1 line-clamp-1">{parentRGD.description}</p>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">{actions}</div>
      </div>

      {/* Meta chip strip */}
      <div className="flex items-center gap-2 flex-wrap px-6 pb-4 text-xs">
        <MetaChip label="Kind">
          <span className="inline-flex items-center gap-2">
            <Link
              to={`/catalog/${encodeURIComponent(instance.rgdName)}`}
              className="font-mono text-xs text-primary hover:underline"
            >
              {instance.kind}
            </Link>
            {canReadRGD && parentRGD?.lastIssuedRevision ? (
              <button
                type="button"
                onClick={onRevisionClick}
                aria-label={`View changes for revision ${parentRGD.lastIssuedRevision}`}
                className="inline-flex items-center gap-1 rounded-full border border-[var(--border-default)] px-2 py-px text-[11px] font-mono text-[var(--text-secondary)] hover:text-primary hover:border-primary/50 transition-colors cursor-pointer"
              >
                <GitBranch className="h-3 w-3" />
                Rev {parentRGD.lastIssuedRevision}
              </button>
            ) : null}
          </span>
        </MetaChip>
        {instance.isClusterScoped ? (
          <MetaChip label="Scope">
            <span className="font-mono text-xs text-[var(--text-primary)]">Cluster</span>
          </MetaChip>
        ) : (
          instance.namespace && (
            <MetaChip label="Namespace">
              <span className="font-mono text-xs text-[var(--text-primary)]">{instance.namespace}</span>
            </MetaChip>
          )
        )}
        <MetaChip label="Source">
          <InstanceMetadataSection instance={instance} isGitOps={isGitOps} />
        </MetaChip>
        {instance.createdAt && (
          <MetaChip label="Created">
            <span className="text-xs text-[var(--text-primary)]">{formatDistanceToNow(instance.createdAt)}</span>
          </MetaChip>
        )}
      </div>

      {/* Health rollup strip */}
      <div
        className="grid grid-cols-2 lg:grid-cols-4 border-t border-[var(--border-subtle)] bg-[var(--surface-bg)]"
        data-testid="health-rollup"
      >
        <RollupCell
          label="Conditions"
          value={rollup.conditionsTotal > 0 ? `${rollup.conditionsPassing}/${rollup.conditionsTotal}` : "—"}
          sub={rollup.conditionsTotal > 0 ? (allConditionsPassing ? "passing" : "failing") : undefined}
          tone={rollup.conditionsTotal === 0 ? "neutral" : allConditionsPassing ? "ok" : "warn"}
          onClick={() => onSelectTab("status")}
        />
        <RollupCell
          label="Resources"
          value={rollup.resourcesTotal > 0 ? `${rollup.resourcesReady}/${rollup.resourcesTotal}` : "—"}
          sub={
            rollup.resourcesTotal > 0
              ? `ready${rollup.resourcesFailing > 0 ? ` · ${rollup.resourcesFailing} failing` : ""}`
              : undefined
          }
          tone={rollup.resourcesTotal === 0 ? "neutral" : allResourcesReady ? "ok" : "warn"}
          onClick={() => onSelectTab("children")}
        />
        <RollupCell
          label="Events"
          value={String(rollup.eventsCount)}
          sub={`${rollup.eventsWarnings} warning${rollup.eventsWarnings === 1 ? "" : "s"}`}
          tone={rollup.eventsWarnings > 0 ? "warn" : "neutral"}
          onClick={() => onSelectTab("events")}
        />
        <RollupCell
          label="Last reconciled"
          value={
            <span className="font-medium text-sm text-[var(--text-primary)]">
              {rollup.lastReconciled ? formatDistanceToNow(rollup.lastReconciled) : "—"}
            </span>
          }
          sub="by kro"
          onClick={() => onSelectTab("deployment-history")}
        />
      </div>
    </div>
  );
}
