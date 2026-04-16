// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Instance } from "@/types/rgd";
import { Panel } from "./instance-detail-layout";

interface ConfigPanelProps {
  instance: Instance;
}

export function ConfigPanel({ instance }: ConfigPanelProps) {
  const spec = instance.spec ?? {};
  const entries = Object.entries(spec);

  return (
    <Panel title="Configuration">
      {entries.length === 0 ? (
        <p className="text-sm" style={{ color: "var(--text-muted)" }}>
          No configuration values
        </p>
      ) : (
        <div className="space-y-1">
          {entries.map(([key, value]) => (
            <div key={key} className="flex items-center justify-between text-sm">
              <span style={{ color: "var(--text-secondary)" }}>{key}</span>
              <span className="font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                {typeof value === "object" ? JSON.stringify(value) : String(value ?? "")}
              </span>
            </div>
          ))}
        </div>
      )}
    </Panel>
  );
}
