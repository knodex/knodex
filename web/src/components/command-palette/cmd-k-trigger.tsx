// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Search } from "@/lib/icons";
import { cn } from "@/lib/utils";

interface CmdKTriggerProps {
  /** Fired when the user clicks the trigger. ⌘K/Ctrl-K keybind is wired globally elsewhere. */
  onOpen: () => void;
  /** Placeholder copy shown in the search-shaped button */
  placeholder?: string;
  className?: string;
}

function detectMac(): boolean {
  if (typeof navigator === "undefined") return false;
  // navigator.platform is deprecated but still the most reliable in vitest/jsdom.
  return /Mac|iPhone|iPad/i.test(navigator.platform || navigator.userAgent || "");
}

export function CmdKTrigger({
  onOpen,
  placeholder = "Search…",
  className,
}: CmdKTriggerProps) {
  // Lazy initializer — computes once on first render so Macs don't flicker
  // "Ctrl K" → "⌘K" between the first paint and a follow-up effect.
  const [isMac] = useState(detectMac);

  return (
    <button
      type="button"
      onClick={onOpen}
      data-testid="cmdk-trigger"
      aria-label="Open command palette"
      aria-keyshortcuts={isMac ? "Meta+K" : "Control+K"}
      className={cn(
        "group inline-flex items-center gap-2 h-8 px-2.5 rounded-[var(--radius-token-md)]",
        "min-w-[220px] max-w-[280px] border border-[var(--border-subtle)]",
        "bg-[rgba(255,255,255,0.03)] hover:bg-[rgba(255,255,255,0.06)]",
        "text-[var(--text-size-sm)] text-muted-foreground",
        "transition-colors duration-150",
        "focus:outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/40",
        className,
      )}
    >
      <Search className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="flex-1 text-left truncate">{placeholder}</span>
      <kbd
        className={cn(
          "hidden sm:inline-flex items-center justify-center font-mono text-[11px]",
          "px-1.5 py-px rounded border border-[var(--border-subtle)]",
          "text-muted-foreground bg-[rgba(255,255,255,0.04)] shrink-0",
        )}
        aria-hidden="true"
      >
        {isMac ? "⌘K" : "Ctrl K"}
      </kbd>
    </button>
  );
}
