// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { type LucideIcon } from "@/lib/icons";
import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface EmptyStateProps {
  /** Lucide icon component to display */
  icon: LucideIcon;
  /** Main heading text */
  title: string;
  /** Supporting description text */
  description: string;
  /** Optional action button or link */
  action?: ReactNode;
  /** Optional additional className */
  className?: string;
}

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-16 text-center",
        className
      )}
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary mb-4">
        <Icon className="h-6 w-6" style={{ color: "var(--text-muted)" }} />
      </div>
      <h3
        className="text-base font-medium mb-1"
        style={{ color: "var(--text-primary)" }}
      >
        {title}
      </h3>
      <p
        className="text-[13px] max-w-sm"
        style={{ color: "var(--text-secondary)" }}
      >
        {description}
      </p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
