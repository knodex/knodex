// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, useState, useMemo } from "react";
import yaml from "js-yaml";
import { Badge } from "@/components/ui/badge";
import { CopyButton } from "@/components/ui/copy-button";
import { cn } from "@/lib/utils";
import type { InstanceCondition } from "@/types/rgd";
import { Check, X, ExternalLink, CheckCircle, XCircle, AlertTriangle, ChevronDown } from "@/lib/icons";
import { formatConditionMessage } from "@/lib/condition-message";

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

/**
 * Past this nesting depth, sections stop and StatusFieldValue falls back to
 * raw JSON. Shared so the section cutoff and the JSON guard can't drift apart.
 */
const MAX_STATUS_DEPTH = 5;

/**
 * KRO structured status fields (kro.run/docs/concepts/rgd/schema#structured-status-fields):
 * a plain-object field is an author-declared category, promoted to its own section.
 * Empty objects stay in the flat list so they keep the "-" rendering.
 */
function isCategoryGroup(value: unknown): value is Record<string, unknown> {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    Object.keys(value).length > 0
  );
}

function partitionCustomFields(customFields: Record<string, unknown>): {
  scalarEntries: [string, unknown][];
  groupEntries: [string, Record<string, unknown>][];
} {
  const scalarEntries: [string, unknown][] = [];
  const groupEntries: [string, Record<string, unknown>][] = [];
  for (const [key, value] of Object.entries(customFields)) {
    // Empty keys would render an empty section heading; keep them in the flat list.
    if (key !== "" && isCategoryGroup(value)) {
      groupEntries.push([key, value]);
    } else {
      scalarEntries.push([key, value]);
    }
  }
  return { scalarEntries, groupEntries };
}

/**
 * Section testids are dot-joined key paths. Literal dots (and backslashes) in
 * status keys are escaped so a key like "a.b" can't collide with nested path a→b.
 */
function escapeTestIdSegment(key: string): string {
  return key.replaceAll("\\", "\\\\").replaceAll(".", "\\.");
}

/** Check if a value looks like a URL */
function isUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  return /^https?:\/\/\S+$/.test(value);
}

/**
 * Resource-ID-style paths (Azure IDs, K8s self-links): dim the path prefix and
 * emphasize the leaf so the identifying segment survives truncation.
 */
function splitResourcePath(value: string): { path: string; leaf: string } | null {
  if (!value.startsWith("/") || /\s/.test(value)) return null;
  const idx = value.lastIndexOf("/");
  if (idx <= 0 || idx === value.length - 1) return null;
  if (value.split("/").length - 1 < 3) return null;
  return { path: value.slice(0, idx + 1), leaf: value.slice(idx + 1) };
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
  if (depth > MAX_STATUS_DEPTH) {
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
    const resourcePath = splitResourcePath(value);
    if (resourcePath) {
      return (
        <span className="flex items-center min-w-0 text-sm font-mono" title={value}>
          {/* rtl + truncate keeps the leaf-adjacent end of the path visible */}
          <span className="truncate text-muted-foreground [direction:rtl] text-left">
            {resourcePath.path}
          </span>
          <span className="whitespace-nowrap text-foreground">{resourcePath.leaf}</span>
        </span>
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
      <div className={cn("space-y-2", depth > 0 && "pl-4 border-l border-[var(--border-subtle)]")}>
        {entries.map(([k, v]) => (
          <StatusFieldRow key={k} fieldKey={k} value={v} depth={depth + 1} />
        ))}
      </div>
    );
  }

  // fallback
  return <span className="text-sm font-mono text-foreground">{String(value)}</span>;
});

// ─── Field Rows & Category Sections ─────────────────────────────────────────

interface StatusFieldRowProps {
  fieldKey: string;
  value: unknown;
  depth?: number;
}

/** The single label/value row layout shared by flat fields, section members, and nested objects. */
function StatusFieldRow({ fieldKey, value, depth = 0 }: StatusFieldRowProps) {
  const copyable = typeof value === "string" || typeof value === "number";
  return (
    <div className="group/fieldrow grid grid-cols-[minmax(120px,auto)_1fr] gap-x-4 items-start">
      <span className="text-xs text-muted-foreground whitespace-nowrap">
        {formatLabel(fieldKey)}
      </span>
      <div className="flex items-start gap-1 min-w-0">
        <div className="min-w-0 flex-1">
          <StatusFieldValue value={value} depth={depth} />
        </div>
        {copyable && (
          <CopyButton
            text={String(value)}
            variant="ghost"
            aria-label={`Copy ${formatLabel(fieldKey)}`}
            className="h-5 w-5 shrink-0 p-0 text-[var(--text-muted)] opacity-0 transition-opacity group-hover/fieldrow:opacity-100 focus-visible:opacity-100 hover:text-primary"
            iconClassName="h-3 w-3"
          />
        )}
      </div>
    </div>
  );
}

interface CategoryFieldsProps {
  fields: Record<string, unknown>;
  /** Escaped dot-joined key path of this section (see escapeTestIdSegment). */
  pathPrefix: string;
  level: number;
}

/**
 * Recursive body of a structured-status section: scalar/array members first
 * (standard rows), then one indented sub-section per object member.
 * Depth parity with StatusFieldValue: members of a section at level L render
 * at depth L+1; objects only become sub-sections while level < MAX_STATUS_DEPTH,
 * beyond that they fall back to StatusFieldValue so the JSON guard fires as before.
 */
const CategoryFields = memo(function CategoryFieldsInner({
  fields,
  pathPrefix,
  level,
}: CategoryFieldsProps) {
  const { scalarEntries, groupEntries } = partitionCustomFields(fields);

  return (
    <div className="space-y-3">
      {scalarEntries.map(([key, value]) => (
        <StatusFieldRow key={key} fieldKey={key} value={value} depth={level + 1} />
      ))}
      {groupEntries.map(([key, value]) => {
        if (level >= MAX_STATUS_DEPTH) {
          return <StatusFieldRow key={key} fieldKey={key} value={value} depth={level + 1} />;
        }
        const childPath = `${pathPrefix}.${escapeTestIdSegment(key)}`;
        return (
          <div key={key} data-testid={`status-group-${childPath}`}>
            {/* aria-level keeps the heading outline nested (h4 group → 5, 6, …) where the tag caps at h5 */}
            <h5
              aria-level={5 + level}
              className="text-[11px] font-medium text-[var(--text-muted)] uppercase tracking-wider mb-2"
            >
              {formatLabel(key)}
            </h5>
            <div className="pl-4 border-l border-[var(--border-subtle)]">
              <CategoryFields fields={value} pathPrefix={childPath} level={level + 1} />
            </div>
          </div>
        );
      })}
    </div>
  );
});

// ─── Main Component ────────────────────────────────────────────────────────

function InstanceStatusCardInner({ status, conditions }: InstanceStatusCardProps) {
  const state = status?.state as string | undefined;
  const customFields = useMemo(() => status ? getCustomFields(status) : {}, [status]);
  const hasCustomFields = Object.keys(customFields).length > 0;
  const hasConditions = conditions && conditions.length > 0;

  // Auto-expand when any condition is not True (developer needs to see what's wrong)
  const hasFailingCondition = hasConditions && conditions.some(c => c.status !== "True");
  const [conditionsOpen, setConditionsOpen] = useState(hasFailingCondition);

  const trueCount = useMemo(
    () => hasConditions ? conditions.filter(c => c.status === "True").length : 0,
    [hasConditions, conditions]
  );
  const totalCount = hasConditions ? conditions.length : 0;

  const { scalarEntries, groupEntries } = useMemo(
    () => partitionCustomFields(customFields),
    [customFields]
  );

  const fieldCount = Object.keys(customFields).length;
  const copyAllText = useMemo(
    () => (hasCustomFields ? yaml.dump(customFields, { lineWidth: 120, noRefs: true }) : ""),
    [hasCustomFields, customFields]
  );

  // AC-8: If nothing to show, render nothing
  if (!state && !hasCustomFields && !hasConditions) {
    return null;
  }

  return (
    <div
      className="rounded-lg border overflow-hidden border-[var(--border-default)] bg-[var(--surface-primary)]"
      data-testid="instance-status-card"
    >
      {/* Header: title + field count + copy all + state badge */}
      <div className="px-5 py-3 flex items-center justify-between border-b border-[var(--border-subtle)]">
        <div className="flex items-center gap-2.5">
          <h3 className="text-sm font-medium text-[var(--text-primary)]">
            {hasCustomFields ? "Outputs" : "Status"}
          </h3>
          {fieldCount > 0 && (
            <span className="text-xs text-[var(--text-muted)]">
              {fieldCount} field{fieldCount === 1 ? "" : "s"}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {hasCustomFields && (
            <CopyButton
              text={copyAllText}
              label="Copy all"
              variant="ghost"
              className="h-7 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              iconClassName="h-3 w-3"
            />
          )}
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
      </div>

      {/* Custom status fields: flat fields first, then one section per structured category */}
      {hasCustomFields && (
        <div
          className={cn(hasConditions && "border-b border-[var(--border-subtle)]")}
          data-testid="custom-fields-section"
        >
          {scalarEntries.length > 0 && (
            <div className="px-5 py-4 space-y-3">
              {scalarEntries.map(([key, value]) => (
                <StatusFieldRow key={key} fieldKey={key} value={value} />
              ))}
            </div>
          )}
          {groupEntries.map(([groupKey, fields], groupIdx) => (
            <div
              key={groupKey}
              className={cn(
                "px-5 py-4",
                (scalarEntries.length > 0 || groupIdx > 0) && "border-t border-[var(--border-subtle)]"
              )}
              data-testid={`status-group-${escapeTestIdSegment(groupKey)}`}
            >
              <h4 className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wider mb-3">
                {formatLabel(groupKey)}
              </h4>
              <CategoryFields fields={fields} pathPrefix={escapeTestIdSegment(groupKey)} level={0} />
            </div>
          ))}
        </div>
      )}

      {/* Conditions — collapsible, auto-expanded when failing */}
      {hasConditions && (
        <div data-testid="conditions-section">
          <button
            type="button"
            onClick={() => setConditionsOpen(!conditionsOpen)}
            className="w-full px-5 py-2.5 flex items-center justify-between hover:bg-[var(--border-subtle)] transition-colors"
            aria-expanded={conditionsOpen}
          >
            <span className="flex items-center gap-2">
              <span className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wider">
                Conditions
              </span>
              <span className={cn(
                "text-xs font-medium",
                hasFailingCondition ? "text-destructive" : "text-[var(--text-muted)]"
              )}>
                {trueCount}/{totalCount}
              </span>
            </span>
            <ChevronDown className={cn(
              "h-4 w-4 text-[var(--text-muted)] transition-transform",
              conditionsOpen && "rotate-180"
            )} />
          </button>
          {conditionsOpen && (
            <div className="border-t border-[var(--border-subtle)]">
              {conditions.map((condition, idx) => (
                <div
                  key={`${condition.type}-${idx}`}
                  className={cn("px-5 py-3 flex items-center justify-between gap-4", idx < conditions.length - 1 && "border-b border-[var(--border-subtle)]")}
                >
                  <div className="flex items-center gap-3">
                    {condition.status === "True" ? (
                      <CheckCircle className="h-4 w-4 shrink-0 text-primary" />
                    ) : condition.status === "False" ? (
                      <XCircle className="h-4 w-4 shrink-0 text-destructive" />
                    ) : (
                      <AlertTriangle className="h-4 w-4 shrink-0 text-status-warning" />
                    )}
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
                          {formatConditionMessage(condition.message)}
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
          )}
        </div>
      )}
    </div>
  );
}

export const InstanceStatusCard = memo(InstanceStatusCardInner);
