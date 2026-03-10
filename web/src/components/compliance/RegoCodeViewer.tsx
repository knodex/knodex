// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { logger } from "@/lib/logger";

interface RegoCodeViewerProps {
  /** Rego policy code to display */
  code: string;
  /** Optional title for the code block */
  title?: string;
  /** Maximum height before scrolling (default: 400px) */
  maxHeight?: string;
  /** Optional className for customization */
  className?: string;
  /** Whether to show line numbers */
  showLineNumbers?: boolean;
}

/**
 * Code viewer component for displaying Rego policy code
 * AC-TPL-06: Rego code displayed in syntax-highlighted code block
 *
 * Note: Uses basic styling. For full syntax highlighting,
 * consider adding react-syntax-highlighter or similar.
 */
export function RegoCodeViewer({
  code,
  title,
  maxHeight = "400px",
  className,
  showLineNumbers = true,
}: RegoCodeViewerProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      logger.error("[RegoCodeViewer] Failed to copy code:", err);
    }
  };

  const lines = code.split("\n");

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-muted/50 overflow-hidden",
        className
      )}
    >
      {/* Header with title and copy button */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-muted">
        <span className="text-sm font-medium text-muted-foreground">
          {title || "Rego Policy"}
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleCopy}
          className="h-7 px-2 text-muted-foreground hover:text-foreground"
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5 mr-1" />
              Copied
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5 mr-1" />
              Copy
            </>
          )}
        </Button>
      </div>

      {/* Code content */}
      <div
        className="overflow-auto"
        style={{ maxHeight }}
      >
        <pre className="p-4 text-sm leading-relaxed">
          <code className="font-mono text-foreground">
            {showLineNumbers ? (
              <table className="border-collapse">
                <tbody>
                  {lines.map((line, index) => (
                    <tr key={index} className="leading-relaxed">
                      <td className="pr-4 text-right text-muted-foreground select-none w-8 align-top">
                        {index + 1}
                      </td>
                      <td className="whitespace-pre">
                        <RegoLine line={line} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              lines.map((line, index) => (
                <div key={index}>
                  <RegoLine line={line} />
                </div>
              ))
            )}
          </code>
        </pre>
      </div>
    </div>
  );
}

/**
 * Basic Rego syntax highlighting for a single line
 * Highlights: comments, strings, keywords, and operators
 */
function RegoLine({ line }: { line: string }) {
  // Handle empty lines
  if (!line.trim()) {
    return <span>&nbsp;</span>;
  }

  // Simple tokenization for basic syntax highlighting
  const tokens = tokenizeRegoLine(line);

  return (
    <>
      {tokens.map((token, index) => (
        <span key={index} className={getTokenClass(token.type)}>
          {token.value}
        </span>
      ))}
    </>
  );
}

type TokenType = "keyword" | "comment" | "string" | "operator" | "boolean" | "text";

interface Token {
  type: TokenType;
  value: string;
}

const REGO_KEYWORDS = [
  "package",
  "import",
  "default",
  "not",
  "with",
  "as",
  "some",
  "in",
  "every",
  "if",
  "contains",
  "else",
];

function tokenizeRegoLine(line: string): Token[] {
  const tokens: Token[] = [];
  let remaining = line;

  while (remaining.length > 0) {
    // Check for comment (starts with #)
    if (remaining.startsWith("#")) {
      tokens.push({ type: "comment", value: remaining });
      break;
    }

    // Check for string (double quotes)
    const stringMatch = remaining.match(/^"(?:[^"\\]|\\.)*"/);
    if (stringMatch) {
      tokens.push({ type: "string", value: stringMatch[0] });
      remaining = remaining.slice(stringMatch[0].length);
      continue;
    }

    // Check for string (backticks for raw strings)
    const rawStringMatch = remaining.match(/^`[^`]*`/);
    if (rawStringMatch) {
      tokens.push({ type: "string", value: rawStringMatch[0] });
      remaining = remaining.slice(rawStringMatch[0].length);
      continue;
    }

    // Check for operators
    const operatorMatch = remaining.match(/^(:=|==|!=|>=|<=|::|[{}[\]()=<>:;,.|&])/);
    if (operatorMatch) {
      tokens.push({ type: "operator", value: operatorMatch[0] });
      remaining = remaining.slice(operatorMatch[0].length);
      continue;
    }

    // Check for keywords and identifiers
    const wordMatch = remaining.match(/^[a-zA-Z_][a-zA-Z0-9_]*/);
    if (wordMatch) {
      const word = wordMatch[0];
      if (REGO_KEYWORDS.includes(word)) {
        tokens.push({ type: "keyword", value: word });
      } else if (word === "true" || word === "false") {
        tokens.push({ type: "boolean", value: word });
      } else {
        tokens.push({ type: "text", value: word });
      }
      remaining = remaining.slice(word.length);
      continue;
    }

    // Check for numbers
    const numberMatch = remaining.match(/^\d+(\.\d+)?/);
    if (numberMatch) {
      tokens.push({ type: "boolean", value: numberMatch[0] }); // reuse boolean styling for numbers
      remaining = remaining.slice(numberMatch[0].length);
      continue;
    }

    // Whitespace and other characters
    const whitespaceMatch = remaining.match(/^\s+/);
    if (whitespaceMatch) {
      tokens.push({ type: "text", value: whitespaceMatch[0] });
      remaining = remaining.slice(whitespaceMatch[0].length);
      continue;
    }

    // Single character fallback
    tokens.push({ type: "text", value: remaining[0] });
    remaining = remaining.slice(1);
  }

  return tokens;
}

function getTokenClass(type: TokenType): string {
  switch (type) {
    case "keyword":
      return "text-purple-500 dark:text-purple-400 font-semibold";
    case "comment":
      return "text-gray-500 dark:text-gray-400 italic";
    case "string":
      return "text-green-600 dark:text-green-400";
    case "operator":
      return "text-amber-600 dark:text-amber-400";
    case "boolean":
      return "text-blue-500 dark:text-blue-400";
    default:
      return "";
  }
}

export default RegoCodeViewer;
