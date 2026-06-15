// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { cn } from "@/lib/utils";
import {
  parseSpec,
  isRGDSpec,
  summarizeRGDSpec,
  groupTraceability,
} from "./spec-extract";

interface SpecPreviewProps {
  /** The extracted YAML block (raw text, already trimmed). */
  yamlBlock: string;
  className?: string;
}

type PreviewTab = "structured" | "yaml";

/**
 * Structured spec preview (Story 50.1, AC #2): renders the generated spec as
 * a readable structured view — not raw YAML only — with a Structured | YAML
 * toggle. RGD specs get a dedicated layout (name, schema kind/apiVersion,
 * resources table); anything else falls back to a generic key/value list.
 * Unparseable YAML renders the raw block with a parse notice.
 */
export function SpecPreview({ yamlBlock, className }: SpecPreviewProps) {
  const [tab, setTab] = useState<PreviewTab>("structured");
  const spec = parseSpec(yamlBlock);

  // Parse failure: raw YAML only, with a notice (copy/download still work —
  // the caller owns those actions and the block text exists regardless).
  if (!spec) {
    return (
      <div className={className} data-testid="spec-preview">
        <p
          data-testid="spec-parse-notice"
          className="text-xs text-muted-foreground mb-2"
        >
          The spec could not be parsed as YAML — showing the raw block.
        </p>
        <YamlView yamlBlock={yamlBlock} />
      </div>
    );
  }

  return (
    <div className={className} data-testid="spec-preview">
      {/* Structured | YAML toggle */}
      <div
        role="tablist"
        aria-label="Spec view"
        className="inline-flex items-center gap-1 rounded-md bg-muted/60 p-0.5 mb-3"
      >
        <ToggleButton
          active={tab === "structured"}
          onClick={() => setTab("structured")}
          testId="spec-tab-structured"
        >
          Structured
        </ToggleButton>
        <ToggleButton
          active={tab === "yaml"}
          onClick={() => setTab("yaml")}
          testId="spec-tab-yaml"
        >
          YAML
        </ToggleButton>
      </div>

      {tab === "yaml" ? (
        <YamlView yamlBlock={yamlBlock} />
      ) : isRGDSpec(spec) ? (
        <RGDStructuredView spec={spec} />
      ) : (
        <GenericStructuredView spec={spec} />
      )}
    </div>
  );
}

function ToggleButton({
  active,
  onClick,
  testId,
  children,
}: {
  active: boolean;
  onClick: () => void;
  testId: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      data-testid={testId}
      onClick={onClick}
      className={cn(
        "px-3 py-1 rounded text-xs font-medium transition-colors",
        active
          ? "bg-card text-foreground shadow-sm"
          : "text-muted-foreground hover:text-foreground"
      )}
    >
      {children}
    </button>
  );
}

function YamlView({ yamlBlock }: { yamlBlock: string }) {
  return (
    <pre
      data-testid="spec-yaml-view"
      className="rounded-md border border-border/60 bg-muted/30 p-4 text-xs font-mono overflow-x-auto whitespace-pre"
    >
      {yamlBlock}
    </pre>
  );
}

/** Structured branch for kind: ResourceGraphDefinition. */
function RGDStructuredView({ spec }: { spec: Record<string, unknown> }) {
  const summary = summarizeRGDSpec(spec);
  return (
    <div
      data-testid="spec-structured-view"
      className="rounded-md border border-border/60 divide-y divide-border/60"
    >
      <dl className="grid grid-cols-1 sm:grid-cols-3 gap-x-6 gap-y-2 p-4 text-sm">
        <div>
          <dt className="text-xs text-muted-foreground">RGD name</dt>
          <dd className="font-medium text-foreground" data-testid="spec-rgd-name">
            {summary.name || "—"}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Schema kind</dt>
          <dd className="font-medium text-foreground" data-testid="spec-schema-kind">
            {summary.schemaKind || "—"}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Schema apiVersion</dt>
          <dd className="font-medium text-foreground" data-testid="spec-schema-apiversion">
            {summary.schemaApiVersion || "—"}
          </dd>
        </div>
      </dl>
      <div className="p-4">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
          Resources
        </h4>
        {summary.resources.length === 0 ? (
          <p className="text-sm text-muted-foreground">No resources in this spec.</p>
        ) : (
          <table className="w-full text-sm" data-testid="spec-resources-table">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="py-1 pr-4 font-medium">ID</th>
                <th className="py-1 pr-4 font-medium">Kind</th>
                <th className="py-1 font-medium">apiVersion</th>
              </tr>
            </thead>
            <tbody>
              {summary.resources.map((resource, index) => (
                <tr key={`${resource.id}-${index}`} className="border-t border-border/40">
                  <td className="py-1.5 pr-4 font-mono text-xs">{resource.id || "—"}</td>
                  <td className="py-1.5 pr-4">{resource.kind || "—"}</td>
                  <td className="py-1.5 text-muted-foreground">{resource.apiVersion || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {/* Requirement → resource traceability (Story 50.2 AC #2): which
          requirement produced which resource, grouped by the
          knodex.io/generated-from annotation. Hidden when there are no
          resources. Requirement text is untrusted agent output — rendered as
          TEXT content only (React escapes it). */}
      {summary.resources.length > 0 && (
        <div className="p-4" data-testid="spec-traceability">
          <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Generated from requirements
          </h4>
          <ul className="space-y-2">
            {groupTraceability(summary).map((group, index) => (
              <li
                key={`${group.requirement}-${index}`}
                className="flex flex-col gap-1.5 sm:flex-row sm:items-baseline sm:gap-3 text-sm"
                data-testid="spec-traceability-group"
              >
                <span className="min-w-0 sm:max-w-[50%] text-foreground">
                  {group.requirement ? (
                    <>&ldquo;{group.requirement}&rdquo;</>
                  ) : (
                    <span className="text-muted-foreground italic">
                      No requirement recorded
                    </span>
                  )}
                </span>
                <span className="flex flex-wrap gap-1.5">
                  {group.resources.map((resource, resourceIndex) => (
                    <span
                      key={`${resource.id}-${resourceIndex}`}
                      className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 text-xs"
                      data-testid="spec-traceability-resource"
                    >
                      <span className="font-mono">{resource.id || "—"}</span>
                      <span className="text-muted-foreground">{resource.kind || "—"}</span>
                    </span>
                  ))}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

/** Generic key/value fallback for non-RGD specs (agent output is untrusted). */
function GenericStructuredView({ spec }: { spec: Record<string, unknown> }) {
  return (
    <dl
      data-testid="spec-generic-view"
      className="rounded-md border border-border/60 p-4 space-y-2 text-sm"
    >
      {Object.entries(spec).map(([key, value]) => (
        <div key={key} className="grid grid-cols-[8rem_1fr] gap-3">
          <dt className="text-xs text-muted-foreground font-medium pt-0.5">{key}</dt>
          <dd className="font-mono text-xs text-foreground break-all">
            {typeof value === "string" ? value : JSON.stringify(value)}
          </dd>
        </div>
      ))}
    </dl>
  );
}
