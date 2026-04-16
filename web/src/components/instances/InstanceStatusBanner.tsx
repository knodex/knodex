// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import type { InstanceHealth } from "@/types/rgd";
import {
  CheckCircle,
  AlertTriangle,
  XCircle,
  Loader2,
  HelpCircle,
} from "@/lib/icons";

const BANNER_CONFIG: Record<
  InstanceHealth,
  { icon: typeof CheckCircle; className: string; message: string }
> = {
  Healthy: {
    icon: CheckCircle,
    className: "bg-primary/10 text-primary border-primary/20",
    message: "This instance is healthy and operating normally.",
  },
  Progressing: {
    icon: Loader2,
    className: "bg-status-info/10 text-status-info border-status-info/20",
    message: "This instance is being provisioned or updated.",
  },
  Degraded: {
    icon: AlertTriangle,
    className: "bg-status-warning/10 text-status-warning border-status-warning/20",
    message: "This instance is experiencing degraded performance.",
  },
  Unhealthy: {
    icon: XCircle,
    className: "bg-destructive/10 text-destructive border-destructive/20",
    message: "This instance is unhealthy and may require attention.",
  },
  Unknown: {
    icon: HelpCircle,
    className: "bg-secondary text-muted-foreground border-border",
    message: "The status of this instance is unknown.",
  },
};

interface InstanceStatusBannerProps {
  health: InstanceHealth;
  /** KRO state (e.g. DELETING, ERROR) — overrides health message when set */
  state?: string;
  /** True when knodex.io/gitops-initial-phase is set — instance is awaiting its first GitOps sync */
  gitopsInitialPhase?: boolean;
}

export function InstanceStatusBanner({ health, state, gitopsInitialPhase }: InstanceStatusBannerProps) {
  const isDeleting = state === "DELETING";

  if (gitopsInitialPhase) {
    return (
      <div
        className="flex items-center gap-3 rounded-lg border px-4 py-3 bg-status-warning/10 text-status-warning border-status-warning/20"
        role="status"
        data-testid="instance-status-banner"
      >
        <AlertTriangle className="h-5 w-5 shrink-0" />
        <span className="text-sm font-medium">This instance is waiting for GitOps synchronisation to provision it.</span>
      </div>
    );
  }

  if ((health === "Healthy" && !isDeleting) || state === "ERROR") {
    return null;
  }

  const config = BANNER_CONFIG[health] || BANNER_CONFIG.Unknown;
  const Icon = isDeleting ? AlertTriangle : config.icon;
  const isAnimated = health === "Progressing" && !isDeleting;

  const bannerClass = isDeleting
    ? "bg-status-warning/10 text-status-warning border-status-warning/20"
    : config.className;

  const message = isDeleting ? "This instance is being deleted." : config.message;

  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-lg border px-4 py-3",
        bannerClass
      )}
      role="status"
      data-testid="instance-status-banner"
    >
      <Icon
        className={cn("h-5 w-5 shrink-0", isAnimated && "animate-spin")}
      />
      <span className="text-sm font-medium">{message}</span>
    </div>
  );
}
