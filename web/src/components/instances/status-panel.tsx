// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Instance } from "@/types/rgd";
import { Panel } from "./instance-detail-layout";
import { formatDistanceToNow } from "@/lib/date";

interface StatusPanelProps {
  instance: Instance;
}

export function StatusPanel({ instance }: StatusPanelProps) {
  const age = formatDistanceToNow(instance.createdAt);

  return (
    <Panel title="Status">
      <div className="space-y-3">
        {/* Health indicator */}
        <div className="flex items-center gap-2">
          <span
            className="h-2.5 w-2.5 rounded-full"
            style={{
              backgroundColor:
                instance.health === "Healthy"
                  ? "var(--status-healthy)"
                  : instance.health === "Degraded" || instance.health === "Unhealthy"
                    ? "var(--status-error)"
                    : instance.health === "Progressing"
                      ? "var(--status-info)"
                      : "var(--status-inactive)",
            }}
          />
          <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
            {instance.health}
          </span>
        </div>

        {/* Instance details */}
        <div className="space-y-2 text-sm">
          <DetailRow label="Name" value={instance.name} />
          <DetailRow label="Kind" value={instance.kind} />
          {instance.namespace && <DetailRow label="Namespace" value={instance.namespace} />}
          {instance.labels?.["knodex.io/project"] && (
            <DetailRow label="Project" value={instance.labels["knodex.io/project"]} />
          )}
          <DetailRow label="Age" value={age} />
        </div>
      </div>
    </Panel>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span style={{ color: "var(--text-muted)" }}>{label}</span>
      <span className="font-mono text-xs" style={{ color: "var(--text-primary)" }}>{value}</span>
    </div>
  );
}
