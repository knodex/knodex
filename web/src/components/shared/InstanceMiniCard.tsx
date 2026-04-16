// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, type ReactNode } from "react";
import { ExternalLink, Loader2 } from "@/lib/icons";
import type { Instance } from "@/types/rgd";
import { HealthBadge } from "@/components/instances/HealthBadge";
import { ScopeIndicator } from "./ScopeIndicator";

interface InstanceMiniCardProps {
  /** Resolved instance (undefined when loading or not found) */
  instance?: Instance;
  isLoading: boolean;
  /** True when the instance could not be resolved */
  notFound?: boolean;
  /** Bottom action slot (e.g. link to instance detail) */
  action: ReactNode;
  /** Display name for loading and not-found states */
  label?: string;
  /** Kind badge text for the not-found state */
  kindLabel?: string;
  /** Namespace badge text for the not-found state */
  namespaceLabel?: string;
  /** Optional badge rendered next to the name (e.g. Secret indicator) */
  badge?: ReactNode;
}

export const InstanceMiniCard = memo(function InstanceMiniCard({
  instance,
  isLoading,
  notFound,
  action,
  label,
  kindLabel,
  namespaceLabel,
  badge,
}: InstanceMiniCardProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-secondary/30 p-4 flex items-center gap-2">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        <span className="text-sm text-muted-foreground">Resolving {label}...</span>
      </div>
    );
  }

  if (notFound || !instance) {
    return (
      <div className="rounded-lg border border-border bg-secondary/30 p-4">
        <div className="flex items-center gap-2 mb-1">
          <ExternalLink className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">{label}</span>
          {badge}
        </div>
        <div className="flex flex-wrap gap-1.5 mt-1">
          {kindLabel && (
            <span className="px-2 py-0.5 rounded text-xs font-mono bg-secondary text-muted-foreground">
              {kindLabel}
            </span>
          )}
          {namespaceLabel && (
            <span className="px-2 py-0.5 rounded text-xs font-mono bg-secondary text-muted-foreground">
              {namespaceLabel}
            </span>
          )}
        </div>
        <p className="text-xs text-muted-foreground mt-2">External resource</p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-secondary/30 p-4 flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-foreground">
            {instance.name}
          </span>
          <HealthBadge health={instance.health} />
          {badge}
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <span className="px-2 py-0.5 rounded text-xs font-mono bg-secondary text-muted-foreground">
          {instance.kind}
        </span>
        <ScopeIndicator
          isClusterScoped={instance.isClusterScoped}
          namespace={instance.namespace}
          variant="badge"
        />
      </div>
      <div className="mt-auto">{action}</div>
    </div>
  );
});
