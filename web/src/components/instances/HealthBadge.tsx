import { cn } from "@/lib/utils";
import type { InstanceHealth } from "@/types/rgd";
import {
  CheckCircle,
  AlertTriangle,
  XCircle,
  Loader2,
  HelpCircle,
} from "lucide-react";

const HEALTH_CONFIG: Record<
  InstanceHealth,
  { icon: typeof CheckCircle; className: string; label: string }
> = {
  Healthy: {
    icon: CheckCircle,
    className: "text-primary bg-primary/10 border-primary/20",
    label: "Healthy",
  },
  Degraded: {
    icon: AlertTriangle,
    className: "text-status-warning bg-status-warning/10 border-status-warning/20",
    label: "Degraded",
  },
  Unhealthy: {
    icon: XCircle,
    className: "text-destructive bg-destructive/10 border-destructive/20",
    label: "Unhealthy",
  },
  Progressing: {
    icon: Loader2,
    className: "text-status-info bg-status-info/10 border-status-info/20",
    label: "Progressing",
  },
  Unknown: {
    icon: HelpCircle,
    className: "text-muted-foreground bg-secondary border-border",
    label: "Unknown",
  },
};

interface HealthBadgeProps {
  health: InstanceHealth;
  size?: "sm" | "md";
  showLabel?: boolean;
}

export function HealthBadge({
  health,
  size = "md",
  showLabel = true,
}: HealthBadgeProps) {
  const config = HEALTH_CONFIG[health] || HEALTH_CONFIG.Unknown;
  const Icon = config.icon;
  const isAnimated = health === "Progressing";

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border font-medium",
        config.className,
        size === "sm" ? "px-2 py-0.5 text-xs" : "px-2.5 py-1 text-xs"
      )}
    >
      <Icon
        className={cn(
          size === "sm" ? "h-3 w-3" : "h-3.5 w-3.5",
          isAnimated && "animate-spin"
        )}
      />
      {showLabel && <span>{config.label}</span>}
    </span>
  );
}
