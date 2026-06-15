// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { StatusIndicator, type StatusIndicatorStatus } from "@/components/ui/status-indicator";
import type { AgentRunStatus } from "@/api/agent-runs";

/**
 * Run status → StatusIndicator mapping (Story 49.4). The progressing pulse
 * animation IS the live spinner for in-flight runs (UX-DR6) — no hand-rolled
 * spinner.
 */
const RUN_STATUS_MAP: Record<AgentRunStatus, StatusIndicatorStatus> = {
  running: "progressing",
  completed: "healthy",
  failed: "error",
};

interface RunStatusBadgeProps {
  status: AgentRunStatus;
  className?: string;
}

/** Status badge for agent run history rows. */
export function RunStatusBadge({ status, className }: RunStatusBadgeProps) {
  return (
    <span data-testid="run-status-badge" data-status={status} className={className}>
      <StatusIndicator status={RUN_STATUS_MAP[status] ?? "unknown"} variant="dot-label" />
    </span>
  );
}
