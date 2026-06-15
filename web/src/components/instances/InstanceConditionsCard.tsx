// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import { Check, X, AlertTriangle } from "@/lib/icons";
import { formatConditionMessage } from "@/lib/condition-message";
import type { InstanceCondition } from "@/types/rgd";

function ConditionIcon({ status }: { status: string }) {
  if (status === "True") {
    return (
      <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-[var(--status-healthy)]/15 text-[var(--status-healthy)]">
        <Check className="h-2.5 w-2.5" strokeWidth={3} />
      </span>
    );
  }
  if (status === "False") {
    return (
      <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-[var(--status-error)]/15 text-[var(--status-error)]">
        <X className="h-2.5 w-2.5" strokeWidth={3} />
      </span>
    );
  }
  return (
    <span className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-[var(--status-warning)]/15 text-[var(--status-warning)]">
      <AlertTriangle className="h-2.5 w-2.5" />
    </span>
  );
}

interface InstanceConditionsCardProps {
  conditions: InstanceCondition[];
}

/** Compact "is it working" checklist — conditions as a tiled grid. */
export function InstanceConditionsCard({ conditions }: InstanceConditionsCardProps) {
  const passing = conditions.filter((c) => c.status === "True").length;
  const allPassing = passing === conditions.length;

  return (
    <div
      className="rounded-lg border overflow-hidden border-[var(--border-default)] bg-[var(--surface-primary)]"
      data-testid="instance-conditions-card"
    >
      <div className="flex items-center gap-2.5 px-4 py-3 border-b border-[var(--border-subtle)]">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Conditions</h3>
        <span
          className={cn(
            "text-xs",
            allPassing ? "text-[var(--text-muted)]" : "text-[var(--status-error)]"
          )}
        >
          {allPassing ? "all passing" : `${passing}/${conditions.length} passing`}
        </span>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-px bg-[var(--border-subtle)]">
        {conditions.map((condition, idx) => (
          <div
            key={`${condition.type}-${idx}`}
            className="flex flex-col gap-1 bg-[var(--surface-primary)] px-4 py-3"
          >
            <span className="flex items-center gap-2 text-xs font-semibold text-[var(--text-primary)]">
              <ConditionIcon status={condition.status} />
              {condition.type}
            </span>
            {(condition.message || condition.reason) && (
              <span className="text-[11px] leading-snug text-[var(--text-muted)] line-clamp-2">
                {condition.message ? formatConditionMessage(condition.message) : condition.reason}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
