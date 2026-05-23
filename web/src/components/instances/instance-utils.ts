// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Instance, InstanceHealth } from "@/types/rgd";
import type { StatusIndicatorStatus } from "@/components/ui/status-indicator";

export const HEALTH_TO_STATUS: Record<InstanceHealth, StatusIndicatorStatus> = {
  Healthy: "healthy",
  Degraded: "warning",
  Unhealthy: "error",
  Progressing: "progressing",
  Unknown: "unknown",
};

export const LEFT_BORDER_COLOR: Partial<Record<InstanceHealth, string>> = {
  Unhealthy: "var(--status-error)",
  Degraded: "var(--status-warning)",
};

export function getInstanceUrl(instance: Instance): string | undefined {
  const url =
    instance.annotations?.["knodex.io/url"] ||
    instance.annotations?.["knodex.io/service-url"];
  if (url) return url;

  if (instance.status && typeof instance.status === "object") {
    const statusUrl =
      (instance.status as Record<string, unknown>).url ||
      (instance.status as Record<string, unknown>).endpoint;
    if (typeof statusUrl === "string" && statusUrl.startsWith("http")) {
      return statusUrl;
    }
  }
  return undefined;
}

export function safeHostname(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

/**
 * Short, human-readable label describing who/what last touched the instance.
 * Derived from existing fields on the Instance — no schema changes.
 *
 * Returns one of:
 *   - "manual edit"  → live spec drifted from desired (out-of-band write)
 *   - "via GitOps"   → deployment mode is gitops or hybrid
 *   - "via Knodex"   → direct (or unspecified) deployment mode
 */
export function deriveActorLabel(instance: Pick<Instance, "deploymentMode" | "gitopsDrift">): string {
  if (instance.gitopsDrift) return "manual edit";
  if (instance.deploymentMode === "gitops" || instance.deploymentMode === "hybrid") {
    return "via GitOps";
  }
  return "via Knodex";
}
