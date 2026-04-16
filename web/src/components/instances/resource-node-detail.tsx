// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { X } from "@/lib/icons";
import type { Instance } from "@/types/rgd";

interface ResourceNodeDetailProps {
  nodeId: string;
  instance: Instance;
  onClose: () => void;
}

export function ResourceNodeDetail({ nodeId, instance, onClose }: ResourceNodeDetailProps) {
  const conditions = instance.conditions ?? [];

  return (
    <div
      className="fixed top-0 right-0 z-50 h-full w-[480px] border-l overflow-y-auto bg-[var(--surface-elevated)] border-[var(--border-default)]"
      role="dialog"
      aria-label={`Details for ${nodeId}`}
    >
      <div className="flex items-center justify-between p-4 border-b border-[var(--border-default)]">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">
          {nodeId}
        </h3>
        <button type="button" onClick={onClose} aria-label="Close" className="p-1 text-[var(--text-muted)]">
          <X className="h-4 w-4" />
        </button>
      </div>

      <div className="p-4 space-y-4">
        {/* Status conditions */}
        <section>
          <h4 className="text-xs font-medium mb-2 text-[var(--text-muted)]">Conditions</h4>
          {conditions.length === 0 ? (
            <p className="text-xs text-[var(--text-muted)]">No conditions</p>
          ) : (
            <div className="space-y-2">
              {conditions.map((c, i) => (
                <div key={c.type || i} className="rounded-md p-2 text-xs bg-white/[0.03]">
                  <div className="flex justify-between">
                    <span className="text-[var(--text-primary)]">{c.type}</span>
                    <span
                      className={
                        c.status === "True"
                          ? "text-[var(--status-healthy)]"
                          : "text-[var(--status-error)]"
                      }
                    >
                      {c.status}
                    </span>
                  </div>
                  {c.message && <p className="mt-1 text-[var(--text-muted)]">{c.message}</p>}
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
