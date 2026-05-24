// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Button } from "@/components/ui/button";
import { BookOpen, ExternalLink } from "@/lib/icons";

interface DocsButtonProps {
  docsUrl?: string;
  /** Display name used in the aria-label so screen readers know which RGD this links to. */
  rgdLabel: string;
  /** Override the visible button label (default: "Docs"). */
  label?: string;
}

// Accept only absolute http(s) URLs — guards against javascript: and other schemes
// that could otherwise reach the DOM via an annotation set by an RGD author.
function isSafeHttpUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * External documentation link sourced from the `knodex.io/docs-url` annotation.
 * Renders nothing when no URL is provided or the URL is not a safe http(s) link.
 */
export function DocsButton({ docsUrl, rgdLabel, label = "Docs" }: DocsButtonProps) {
  if (!docsUrl || !isSafeHttpUrl(docsUrl)) return null;

  return (
    <Button variant="outline" asChild>
      <a
        href={docsUrl}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={`Open documentation for ${rgdLabel} (opens in new tab)`}
      >
        <BookOpen className="h-4 w-4" aria-hidden="true" />
        <span>{label}</span>
        <ExternalLink className="!size-3 opacity-70" aria-hidden="true" />
      </a>
    </Button>
  );
}
