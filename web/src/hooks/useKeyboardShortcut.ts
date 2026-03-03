/**
 * Hook for registering keyboard shortcuts
 */
import { useEffect, useCallback } from "react";

export interface KeyboardShortcut {
  /** Key combination (e.g., "ctrl+k", "cmd+k", "/") */
  key: string;
  /** Callback when shortcut is triggered */
  callback: (event: KeyboardEvent) => void;
  /** Description for accessibility */
  description?: string;
  /** Whether shortcut is enabled */
  enabled?: boolean;
}

/**
 * Parse key combination string into modifier keys and key
 * Examples: "ctrl+k", "cmd+shift+p", "/"
 */
function parseKeyCombo(combo: string): {
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
  meta: boolean;
  key: string;
} {
  const parts = combo.toLowerCase().split("+");
  const key = parts[parts.length - 1];

  return {
    ctrl: parts.includes("ctrl"),
    shift: parts.includes("shift"),
    alt: parts.includes("alt"),
    meta: parts.includes("cmd") || parts.includes("meta"),
    key,
  };
}

/**
 * Hook to register keyboard shortcuts
 *
 * Usage:
 * ```tsx
 * useKeyboardShortcut({
 *   key: "ctrl+k",
 *   callback: () => openSearch(),
 *   description: "Open search",
 *   enabled: true
 * });
 * ```
 */
export function useKeyboardShortcut(shortcut: KeyboardShortcut) {
  const { key, callback, enabled = true } = shortcut;

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (!enabled) return;

      // Don't trigger if user is typing in an input
      const target = event.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.contentEditable === "true"
      ) {
        return;
      }

      const combo = parseKeyCombo(key);

      // Check if all modifiers and key match
      const matches =
        event.ctrlKey === combo.ctrl &&
        event.shiftKey === combo.shift &&
        event.altKey === combo.alt &&
        event.metaKey === combo.meta &&
        event.key.toLowerCase() === combo.key;

      if (matches) {
        event.preventDefault();
        callback(event);
      }
    },
    [key, callback, enabled]
  );

  useEffect(() => {
    if (!enabled) return;

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown, enabled]);
}

/**
 * Hook to register multiple keyboard shortcuts at once
 */
export function useKeyboardShortcuts(shortcuts: KeyboardShortcut[]) {
  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      shortcuts.forEach(({ key, callback, enabled = true }) => {
        if (!enabled) return;

        // Don't trigger if user is typing in an input
        const target = event.target as HTMLElement;
        if (
          target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.contentEditable === "true"
        ) {
          return;
        }

        const combo = parseKeyCombo(key);

        // Check if all modifiers and key match
        const matches =
          event.ctrlKey === combo.ctrl &&
          event.shiftKey === combo.shift &&
          event.altKey === combo.alt &&
          event.metaKey === combo.meta &&
          event.key.toLowerCase() === combo.key;

        if (matches) {
          event.preventDefault();
          callback(event);
        }
      });
    },
    [shortcuts]
  );

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);
}
