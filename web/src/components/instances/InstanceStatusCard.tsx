import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { InstanceCondition } from "@/types/rgd";
import { Check, X, ExternalLink } from "lucide-react";

interface InstanceStatusCardProps {
  status?: Record<string, unknown>;
  conditions?: InstanceCondition[];
}

/** KRO instance state values */
type KroState = "ACTIVE" | "IN_PROGRESS" | "FAILED" | "DELETING" | "ERROR";

const STATE_STYLES: Record<KroState, string> = {
  ACTIVE: "bg-primary/10 text-primary border-primary/20",
  IN_PROGRESS: "bg-status-warning/10 text-status-warning border-status-warning/20",
  FAILED: "bg-destructive/10 text-destructive border-destructive/20",
  DELETING: "bg-status-warning/10 text-status-warning border-status-warning/20",
  ERROR: "bg-destructive/10 text-destructive border-destructive/20",
};

function getStateBadgeClass(state: string): string {
  return STATE_STYLES[state as KroState] ?? "bg-secondary text-secondary-foreground border-border";
}

/**
 * Extract custom status fields (everything except `state` and `conditions`).
 */
function getCustomFields(status: Record<string, unknown>): Record<string, unknown> {
  const custom: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(status)) {
    if (key === "state" || key === "conditions") continue;
    custom[key] = value;
  }
  return custom;
}

/** Check if a value looks like a URL */
function isUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  return /^https?:\/\/\S+$/.test(value);
}

/** Format a field key into a readable label */
function formatLabel(key: string): string {
  // snake_case → spaces, then camelCase → words
  const spaced = key
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1 $2");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

// ─── Value Renderers ───────────────────────────────────────────────────────

interface StatusFieldValueProps {
  value: unknown;
  depth?: number;
}

/**
 * Recursive renderer for status field values.
 * Handles: string, number, boolean, null/undefined, arrays, nested objects.
 * Memoized to prevent unnecessary re-renders of deeply nested status trees.
 */
const StatusFieldValue = memo(function StatusFieldValue({ value, depth = 0 }: StatusFieldValueProps) {
  // Guard against excessive nesting to prevent stack overflow
  if (depth > 5) {
    return <span className="text-sm font-mono text-muted-foreground">{JSON.stringify(value)}</span>;
  }

  // null / undefined
  if (value === null || value === undefined) {
    return <span className="text-sm text-muted-foreground">-</span>;
  }

  // boolean
  if (typeof value === "boolean") {
    return value ? (
      <span className="inline-flex items-center gap-1 text-sm text-primary">
        <Check className="h-3.5 w-3.5" />
        true
      </span>
    ) : (
      <span className="inline-flex items-center gap-1 text-sm text-destructive">
        <X className="h-3.5 w-3.5" />
        false
      </span>
    );
  }

  // number
  if (typeof value === "number") {
    return <span className="text-sm font-mono text-foreground">{value}</span>;
  }

  // string (URL detection)
  if (typeof value === "string") {
    if (isUrl(value)) {
      return (
        <a
          href={value}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm font-mono text-primary hover:underline inline-flex items-center gap-1"
        >
          {value}
          <ExternalLink className="h-3 w-3 shrink-0" />
        </a>
      );
    }
    return <span className="text-sm font-mono text-foreground">{value}</span>;
  }

  // array
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return <span className="text-sm text-muted-foreground">-</span>;
    }

    // If all items are primitive, render as chips
    const allPrimitive = value.every(
      (v) => typeof v === "string" || typeof v === "number" || typeof v === "boolean"
    );

    if (allPrimitive) {
      return (
        <div className="flex flex-wrap gap-1.5">
          {value.map((item, i) => (
            <span
              key={i}
              className="inline-flex items-center rounded-md bg-secondary px-2 py-0.5 text-xs font-mono text-secondary-foreground"
            >
              {String(item)}
            </span>
          ))}
        </div>
      );
    }

    // Mixed/object arrays: render as a list
    return (
      <div className="space-y-1">
        {value.map((item, i) => (
          <div key={i} className="flex items-start gap-2">
            <span className="text-xs text-muted-foreground mt-1 shrink-0">{i + 1}.</span>
            <StatusFieldValue value={item} depth={depth + 1} />
          </div>
        ))}
      </div>
    );
  }

  // nested object
  if (typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) {
      return <span className="text-sm text-muted-foreground">-</span>;
    }
    return (
      <div className={cn("space-y-2", depth > 0 && "pl-4 border-l border-border")}>
        {entries.map(([k, v]) => (
          <div key={k} className="grid grid-cols-[auto_1fr] gap-x-4 items-start">
            <span className="text-xs text-muted-foreground whitespace-nowrap">{formatLabel(k)}</span>
            <StatusFieldValue value={v} depth={depth + 1} />
          </div>
        ))}
      </div>
    );
  }

  // fallback
  return <span className="text-sm font-mono text-foreground">{String(value)}</span>;
});

// ─── Main Component ────────────────────────────────────────────────────────

export function InstanceStatusCard({ status, conditions }: InstanceStatusCardProps) {
  const state = status?.state as string | undefined;
  const customFields = status ? getCustomFields(status) : {};
  const hasCustomFields = Object.keys(customFields).length > 0;
  const hasConditions = conditions && conditions.length > 0;

  // AC-8: If nothing to show, render nothing
  if (!state && !hasCustomFields && !hasConditions) {
    return null;
  }

  return (
    <div className="rounded-lg border border-border bg-card" data-testid="instance-status-card">
      {/* Header: "Status" title + state badge */}
      <div className="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 className="text-sm font-medium text-foreground">Status</h3>
        {state && (
          <Badge
            className={cn("text-xs font-semibold", getStateBadgeClass(state))}
            data-testid="state-badge"
            aria-label={`Instance state: ${state}`}
          >
            {state}
          </Badge>
        )}
      </div>

      {/* Custom status fields section */}
      {hasCustomFields && (
        <div
          className={cn("px-4 py-3", hasConditions && "border-b border-border")}
          data-testid="custom-fields-section"
        >
          <div className="space-y-3">
            {Object.entries(customFields).map(([key, value]) => (
              <div key={key} className="grid grid-cols-[minmax(120px,auto)_1fr] gap-x-4 items-start">
                <span className="text-xs text-muted-foreground whitespace-nowrap">
                  {formatLabel(key)}
                </span>
                <StatusFieldValue value={value} />
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Conditions section */}
      {hasConditions && (
        <div data-testid="conditions-section">
          <div className="px-4 py-2 border-b border-border">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Conditions
            </span>
          </div>
          <div className="divide-y divide-border">
            {conditions.map((condition, idx) => (
              <div
                key={`${condition.type}-${idx}`}
                className="px-4 py-3 flex items-center justify-between gap-4"
              >
                <div className="flex items-center gap-3">
                  <span
                    className={cn(
                      "h-2 w-2 rounded-full shrink-0",
                      condition.status === "True"
                        ? "bg-primary"
                        : condition.status === "False"
                        ? "bg-destructive"
                        : "bg-status-warning"
                    )}
                  />
                  <div>
                    <span className="font-medium text-sm text-foreground">
                      {condition.type}
                    </span>
                    {condition.reason && (
                      <span className="ml-2 text-xs text-muted-foreground font-mono">
                        ({condition.reason})
                      </span>
                    )}
                    {condition.message && (
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {condition.message}
                      </p>
                    )}
                  </div>
                </div>
                <span
                  className={cn(
                    "px-2 py-0.5 rounded text-xs font-medium shrink-0",
                    condition.status === "True"
                      ? "bg-primary/10 text-primary"
                      : condition.status === "False"
                      ? "bg-destructive/10 text-destructive"
                      : "bg-status-warning/10 text-status-warning"
                  )}
                >
                  {condition.status}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
