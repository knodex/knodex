// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { Copy, Check, Code, ChevronDown, ChevronRight } from "@/lib/icons";
import yaml from "js-yaml";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { logger } from "@/lib/logger";
import { orderEntries } from "@/lib/order-properties";

interface YAMLPreviewProps {
  apiVersion: string;
  kind: string;
  name: string;
  namespace: string;
  spec: Record<string, unknown>;
  /** When true, the preview starts expanded (e.g. GitOps mode) */
  defaultExpanded?: boolean;
  /** Schema default values — lines differing from these get a visual highlight */
  defaultValues?: Record<string, unknown>;
  /** Display order for top-level spec keys */
  propertyOrder?: string[];
  className?: string;
}

export function YAMLPreview({
  apiVersion,
  kind,
  name,
  namespace,
  spec,
  defaultExpanded = false,
  defaultValues,
  propertyOrder,
  className,
}: YAMLPreviewProps) {
  const [isOpen, setIsOpen] = useState(defaultExpanded);
  const [copied, setCopied] = useState(false);

  // Compute which top-level spec keys differ from defaults
  const modifiedKeys = useMemo(() => {
    if (!defaultValues) return new Set<string>();
    const cleaned = cleanSpec(spec, propertyOrder);
    const keys = new Set<string>();
    for (const key of Object.keys(cleaned)) {
      if (JSON.stringify(cleaned[key]) !== JSON.stringify(defaultValues[key])) {
        keys.add(key);
      }
    }
    return keys;
  }, [spec, defaultValues, propertyOrder]);

  // Generate YAML from form values
  const yamlContent = useMemo(() => {
    const metadata: Record<string, string> = {
      name: name || "<instance-name>",
    };
    if (namespace) {
      metadata.namespace = namespace;
    }
    const resource = {
      apiVersion,
      kind,
      metadata,
      spec: cleanSpec(spec, propertyOrder),
    };

    try {
      return yaml.dump(resource, {
        indent: 2,
        lineWidth: 120,
        noRefs: true,
        sortKeys: false,
      });
    } catch {
      return "# Error generating YAML";
    }
  }, [apiVersion, kind, name, namespace, spec, propertyOrder]);

  const handleCopy = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    try {
      await navigator.clipboard.writeText(yamlContent);
      setCopied(true);
      toast.success("YAML copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      logger.error("[YAMLPreview] Failed to copy to clipboard");
    }
  };

  return (
    <div className={cn("rounded-lg border border-border bg-card", className)}>
      <div className="flex items-center justify-between p-4">
        <button
          type="button"
          onClick={() => setIsOpen(!isOpen)}
          className="flex items-center gap-2 hover:opacity-80 transition-opacity"
        >
          {isOpen ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          <Code className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">YAML Preview</span>
        </button>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors rounded-md hover:bg-secondary border border-border"
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5 text-status-success" />
              Copied!
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5" />
              Copy YAML
            </>
          )}
        </button>
      </div>

      {isOpen && (
        <div className="border-t border-border">
          <div className="flex items-center justify-between px-4 py-2 bg-secondary/30">
            <span className="text-xs font-mono text-muted-foreground">
              {apiVersion} / {kind}
            </span>
            <button
              type="button"
              onClick={handleCopy}
              className="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:text-foreground transition-colors rounded-md hover:bg-secondary"
            >
              {copied ? (
                <>
                  <Check className="h-3 w-3 text-status-success" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-3 w-3" />
                  Copy
                </>
              )}
            </button>
          </div>
          <div className="p-4 overflow-x-auto">
            <pre className="text-xs font-mono text-foreground whitespace-pre">
              <YAMLHighlight content={yamlContent} modifiedKeys={modifiedKeys} />
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

// Clean spec by removing undefined and empty values, preserving property order
function cleanSpec(spec: Record<string, unknown>, propOrder?: string[]): Record<string, unknown> {
  const cleaned: Record<string, unknown> = {};

  for (const [key, value] of orderEntries(Object.entries(spec), propOrder)) {
    if (value === undefined || value === "" || value === null) {
      continue;
    }

    if (Array.isArray(value)) {
      if (value.length > 0) {
        cleaned[key] = value.map((item) =>
          typeof item === "object" && item !== null
            ? cleanSpec(item as Record<string, unknown>)
            : item
        );
      }
    } else if (typeof value === "object" && value !== null) {
      const cleanedNested = cleanSpec(value as Record<string, unknown>);
      if (Object.keys(cleanedNested).length > 0) {
        cleaned[key] = cleanedNested;
      }
    } else {
      cleaned[key] = value;
    }
  }

  return cleaned;
}

// Simple YAML syntax highlighting with optional diff markers
function YAMLHighlight({
  content,
  modifiedKeys,
}: {
  content: string;
  modifiedKeys: Set<string>;
}) {
  const lines = content.split("\n");
  const hasModified = modifiedKeys.size > 0;

  // Determine which lines fall under a modified spec key.
  // Track: once we see "spec:", lines indented under it at indent=2 are top-level spec keys.
  // A top-level spec key and all its deeper children are highlighted if the key is in modifiedKeys.
  const lineModified: boolean[] = [];
  let inSpec = false;
  let specIndent = -1;
  let currentSpecKey = "";
  let currentKeyIndent = -1;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const stripped = line.trimStart();
    const indent = line.length - stripped.length;

    if (stripped.startsWith("spec:") && indent === 0) {
      inSpec = true;
      specIndent = indent;
      lineModified.push(false);
      continue;
    }

    if (inSpec) {
      // If we encounter a line at the same or lesser indent than spec (e.g., another top-level key), we left spec
      if (stripped.length > 0 && indent <= specIndent && !stripped.startsWith("spec:")) {
        inSpec = false;
        currentSpecKey = "";
        lineModified.push(false);
        continue;
      }

      // Top-level spec key (indent = specIndent + 2)
      const expectedKeyIndent = specIndent + 2;
      if (indent === expectedKeyIndent && stripped.includes(":")) {
        const colonIdx = stripped.indexOf(":");
        currentSpecKey = stripped.slice(0, colonIdx);
        currentKeyIndent = indent;
        lineModified.push(hasModified && modifiedKeys.has(currentSpecKey));
        continue;
      }

      // Deeper line under the current key
      if (currentSpecKey && indent > currentKeyIndent) {
        lineModified.push(hasModified && modifiedKeys.has(currentSpecKey));
        continue;
      }
    }

    lineModified.push(false);
  }

  return (
    <>
      {lines.map((line, i) => (
        <div
          key={i}
          className={cn(
            lineModified[i] && "bg-status-success/10 border-l-2 border-status-success/40 -ml-2 pl-2"
          )}
        >
          <HighlightedLine line={line} />
        </div>
      ))}
    </>
  );
}

function HighlightedLine({ line }: { line: string }) {
  // Comment
  if (line.trimStart().startsWith("#")) {
    return <span className="text-muted-foreground">{line}</span>;
  }

  // Key-value pair
  const colonIndex = line.indexOf(":");
  if (colonIndex > -1) {
    const key = line.slice(0, colonIndex);
    const rest = line.slice(colonIndex);

    // Check if value is a string literal
    const value = rest.slice(1).trim();
    const isString = value.startsWith('"') || value.startsWith("'");
    const isNumber = !isNaN(Number(value)) && value !== "";
    const isBool = value === "true" || value === "false";
    const isNull = value === "null" || value === "~";

    return (
      <>
        <span className="text-primary">{key}</span>
        <span className="text-muted-foreground">:</span>
        {value && (
          <>
            <span> </span>
            <span
              className={cn(
                isString && "text-syntax-string",
                isNumber && "text-syntax-number",
                isBool && "text-syntax-boolean",
                isNull && "text-muted-foreground"
              )}
            >
              {value}
            </span>
          </>
        )}
      </>
    );
  }

  // Array item
  if (line.trimStart().startsWith("-")) {
    const indent = line.match(/^\s*/)?.[0] || "";
    const rest = line.slice(indent.length + 1).trim();
    return (
      <>
        <span>{indent}</span>
        <span className="text-muted-foreground">-</span>
        {rest && (
          <>
            <span> </span>
            <HighlightedLine line={rest} />
          </>
        )}
      </>
    );
  }

  return <span>{line}</span>;
}
