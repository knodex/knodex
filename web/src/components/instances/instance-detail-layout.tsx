// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PanelProps {
  title: string;
  children: ReactNode;
  className?: string;
}

export function Panel({ title, children, className }: PanelProps) {
  return (
    <div
      className={cn("overflow-y-auto border p-4", className)}
      style={{
        backgroundColor: "var(--surface-primary)",
        borderColor: "rgba(255,255,255,0.08)",
        borderRadius: "var(--radius-token-lg)",
      }}
    >
      <h3
        className="text-sm font-medium mb-3"
        style={{ color: "var(--text-secondary)" }}
      >
        {title}
      </h3>
      {children}
    </div>
  );
}

interface InstanceDetailLayoutProps {
  statusPanel: ReactNode;
  resourceTreePanel: ReactNode;
  configPanel: ReactNode;
}

export function InstanceDetailLayout({
  statusPanel,
  resourceTreePanel,
  configPanel,
}: InstanceDetailLayoutProps) {
  return (
    <div className="space-y-4" data-testid="instance-detail-layout">
      {/* Top row: Status + Resource Tree side by side on desktop */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {statusPanel}
        {resourceTreePanel}
      </div>
      {/* Bottom row: Config panel full width */}
      {configPanel}
    </div>
  );
}
