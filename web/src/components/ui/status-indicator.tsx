// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";

export type StatusIndicatorStatus =
  | "healthy"
  | "warning"
  | "error"
  | "progressing"
  | "inactive"
  | "unknown";

export type StatusIndicatorVariant = "dot" | "dot-label" | "dot-count";

export type StatusIndicatorProps = {
  status: StatusIndicatorStatus;
  className?: string;
} & (
  | { variant?: "dot" | "dot-label" }
  | { variant: "dot-count"; count: number }
);

const STATUS_LABELS: Record<StatusIndicatorStatus, string> = {
  healthy: "Healthy",
  warning: "Warning",
  error: "Failed",
  progressing: "Progressing",
  inactive: "Inactive",
  unknown: "Unknown",
};

const STATUS_COLOR: Record<StatusIndicatorStatus, string | undefined> = {
  healthy: "var(--status-healthy)",
  warning: "var(--status-warning)",
  error: "var(--status-error)",
  progressing: "var(--status-info)",
  inactive: "var(--status-inactive)",
  unknown: undefined,
};

export function StatusIndicator(props: StatusIndicatorProps) {
  const { status, className } = props;
  const variant = props.variant ?? "dot";
  const isProgressing = status === "progressing";
  const isUnknown = status === "unknown";
  const color = STATUS_COLOR[status];

  const dotStyle: React.CSSProperties = {
    width: "8px",
    height: "8px",
    borderRadius: "var(--radius-token-full)",
    ...(isUnknown
      ? { border: "1.5px dashed var(--status-inactive)" }
      : {
          backgroundColor: color,
          ...(status === "healthy"
            ? { boxShadow: "0 0 6px var(--status-healthy)" }
            : {}),
        }),
  };

  const dot = (
    <span
      data-testid="status-dot"
      className={cn(
        "inline-block shrink-0",
        isProgressing && "animate-status-pulse"
      )}
      style={dotStyle}
    />
  );

  return (
    <span
      role="status"
      aria-label={`Status: ${status}`}
      className={cn("inline-flex items-center gap-[var(--space-1)]", className)}
    >
      {dot}
      {variant === "dot-label" && (
        <span className="text-xs text-[var(--text-secondary)]">{STATUS_LABELS[status]}</span>
      )}
      {variant === "dot-count" && (
        <span className="text-xs tabular-nums text-[var(--text-secondary)]">
          {(props as { count: number }).count}
        </span>
      )}
    </span>
  );
}
