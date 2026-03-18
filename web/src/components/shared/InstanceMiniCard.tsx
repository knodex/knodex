// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { AlertCircle, Loader2 } from "lucide-react";
import type { Instance } from "@/types/rgd";
import { HealthBadge } from "@/components/instances/HealthBadge";

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
}

export function InstanceMiniCard({
  instance,
  isLoading,
  notFound,
  action,
  label,
  kindLabel,
  namespaceLabel,
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
          <AlertCircle className="h-4 w-4 text-amber-500" />
          <span className="text-sm font-medium text-foreground">{label}</span>
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
        <p className="text-xs text-amber-600 mt-2">Not deployed</p>
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
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <span className="px-2 py-0.5 rounded text-xs font-mono bg-secondary text-muted-foreground">
          {instance.kind}
        </span>
        <span className="px-2 py-0.5 rounded text-xs font-mono bg-secondary text-muted-foreground">
          {instance.namespace}
        </span>
      </div>
      <div className="mt-auto">{action}</div>
    </div>
  );
}
