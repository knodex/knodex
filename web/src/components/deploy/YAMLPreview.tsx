// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { Copy, Check, Code, ChevronDown, ChevronRight } from "lucide-react";
import yaml from "js-yaml";
import { cn } from "@/lib/utils";
import { logger } from "@/lib/logger";

interface YAMLPreviewProps {
  apiVersion: string;
  kind: string;
  name: string;
  namespace: string;
  spec: Record<string, unknown>;
  className?: string;
}

export function YAMLPreview({
  apiVersion,
  kind,
  name,
  namespace,
  spec,
  className,
}: YAMLPreviewProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [copied, setCopied] = useState(false);

  // Generate YAML from form values
  const yamlContent = useMemo(() => {
    const resource = {
      apiVersion,
      kind,
      metadata: {
        name: name || "<instance-name>",
        namespace,
      },
      spec: cleanSpec(spec),
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
  }, [apiVersion, kind, name, namespace, spec]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(yamlContent);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      logger.error("[YAMLPreview] Failed to copy to clipboard");
    }
  };

  return (
    <div className={cn("rounded-lg border border-border bg-card", className)}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between p-4 hover:bg-secondary/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          {isOpen ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          <Code className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">YAML Preview</span>
        </div>
        <span className="text-xs text-muted-foreground">
          {isOpen ? "Click to collapse" : "Click to expand"}
        </span>
      </button>

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
              <YAMLHighlight content={yamlContent} />
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

// Clean spec by removing undefined and empty values
function cleanSpec(spec: Record<string, unknown>): Record<string, unknown> {
  const cleaned: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(spec)) {
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

// Simple YAML syntax highlighting
function YAMLHighlight({ content }: { content: string }) {
  const lines = content.split("\n");

  return (
    <>
      {lines.map((line, i) => (
        <div key={i}>
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
