import React, { useMemo } from "react";
import { Box, Clock, Package } from "lucide-react";
import type { Instance } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { HealthBadge } from "./HealthBadge";
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

export const InstanceCard = React.memo(function InstanceCard({
  instance,
  onClick,
  isUpdating = false,
}: InstanceCardProps) {
  const formattedTime = useMemo(
    () => formatRelativeTime(instance.createdAt),
    [instance.createdAt]
  );

  return (
    <div
      data-testid="instance-card"
      role="button"
      aria-label={`View details for ${instance.name}`}
      className={cn(
        "group relative cursor-pointer rounded-lg border border-border/60 bg-card p-5",
        "transition-all duration-200 ease-out",
        "hover:border-primary/30 hover:bg-accent/5",
        isUpdating && "animate-update-flash"
      )}
      onClick={() => onClick?.(instance)}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3 mb-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <Box className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <Tooltip>
              <TooltipTrigger asChild>
                <h3 className="font-semibold text-foreground truncate text-base group-hover:text-primary transition-colors duration-200">
                  {instance.name}
                </h3>
              </TooltipTrigger>
              <TooltipContent>
                <p>{instance.name}</p>
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <p className="text-xs text-muted-foreground font-mono truncate mt-1">
                  {instance.namespace}
                </p>
              </TooltipTrigger>
              <TooltipContent>
                <p>{instance.namespace}</p>
              </TooltipContent>
            </Tooltip>
          </div>
        </div>
        <HealthBadge health={instance.health} size="sm" />
      </div>

      {/* RGD Info */}
      <p className="text-sm text-muted-foreground mb-4 leading-relaxed">
        <span className="text-xs">RGD:</span>{" "}
        <span className="font-mono text-foreground">{instance.rgdName}</span>
      </p>

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
      <div className="flex items-center justify-between pt-3 border-t border-border/50 text-xs text-muted-foreground">
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

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 30) return date.toLocaleDateString();
  if (diffDays > 0) return `${diffDays}d ago`;
  if (diffHours > 0) return `${diffHours}h ago`;
  if (diffMins > 0) return `${diffMins}m ago`;
  return "just now";
}
