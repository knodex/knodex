// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useMemo } from "react";
import { Clock, Package } from "@/lib/icons";
import type { Instance } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { formatDistanceToNow } from "@/lib/date";
import { RGDIcon } from "@/components/ui/rgd-icon";
import { HealthBadge } from "./HealthBadge";
import { ScopeIndicator } from "@/components/shared/ScopeIndicator";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface InstanceCardProps {
  instance: Instance;
  onClick?: (instance: Instance) => void;
  /** Show visual flash when instance was recently updated via WebSocket */
  isUpdating?: boolean;
}

// Health-based left border color (CSS variable names from index.css)
const HEALTH_BORDER_VAR: Record<string, string> = {
  Healthy: "var(--success)",
  Degraded: "var(--warning)",
  Progressing: "var(--warning)",
  Unhealthy: "var(--destructive)",
  Unknown: "var(--border)",
};

export const InstanceCard = React.memo(function InstanceCard({
  instance,
  onClick,
  isUpdating = false,
}: InstanceCardProps) {
  const formattedTime = useMemo(
    () => formatDistanceToNow(instance.createdAt),
    [instance.createdAt]
  );

  const healthBorderVar = HEALTH_BORDER_VAR[instance.health] || HEALTH_BORDER_VAR.Unknown;

  return (
    <div
      data-testid="instance-card"
      role="button"
      tabIndex={0}
      aria-label={`View details for ${instance.name}`}
      className={cn(
        "group relative cursor-pointer rounded-lg border border-border/60 bg-card p-5",
        "border-l-[3px]",
        "transition-all duration-200 ease-out",
        "hover:border-primary/30 hover:bg-accent/5",
        isUpdating && "animate-update-flash"
      )}
      style={{ borderLeftColor: `hsl(${healthBorderVar})` }}
      onClick={() => onClick?.(instance)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick?.(instance); } }}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3 mb-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <RGDIcon icon={instance.rgdIcon} category={instance.rgdCategory || "uncategorized"} />
          </div>
          <div className="min-w-0">
            <Tooltip>
              <TooltipTrigger asChild>
                <h3 className="font-semibold text-foreground line-clamp-2 text-base group-hover:text-primary transition-colors duration-200">
                  {instance.name}
                </h3>
              </TooltipTrigger>
              <TooltipContent>
                <p>{instance.name}</p>
              </TooltipContent>
            </Tooltip>
            <div className="mt-1">
              <ScopeIndicator
                isClusterScoped={instance.isClusterScoped}
                namespace={instance.namespace}
                variant="badge"
              />
            </div>
          </div>
        </div>
        <HealthBadge health={instance.health} size="sm" />
      </div>

      {/* RGD Info */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground mb-4 leading-relaxed">
        <span>
          <span className="text-xs">RGD:</span>{" "}
          <span className="font-mono text-foreground">{instance.rgdName}</span>
        </span>
      </div>

      {/* Kind/API Tags */}
      <div className="flex flex-wrap gap-1.5 mb-4">
        <span className="px-2 py-0.5 rounded-md text-xs font-semibold bg-primary/10 text-primary">
          {instance.kind}
        </span>
        <span className="px-2 py-0.5 rounded-md text-xs font-medium text-muted-foreground bg-muted/60">
          {instance.apiVersion}
        </span>
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between pt-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1.5 font-medium">
          <Clock className="h-3.5 w-3.5 text-muted-foreground/70" />
          {formattedTime}
        </span>
        {instance.conditions && instance.conditions.length > 0 && (
          <span className="flex items-center gap-1.5 font-medium">
            <Package className="h-3.5 w-3.5 text-muted-foreground/70" />
            {instance.conditions.filter((c) => c.status === "True").length}/
            {instance.conditions.length} conditions
          </span>
        )}
      </div>
    </div>
  );
});
