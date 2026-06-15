// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useRef, useState } from "react";

export interface UseCopyToClipboardOptions {
  /** How long the `copied` flag stays true after a successful copy (ms). */
  resetDelay?: number;
  /** Called when the copy succeeds (after `copied` is set). */
  onSuccess?: (key: string | null) => void;
  /** Called when the copy fails (clipboard unavailable or write rejected). */
  onError?: (error: unknown) => void;
}

export interface UseCopyToClipboardResult {
  /**
   * Whether the most recent copy is still within the reset window. When a
   * `key` is supplied to `copy`, this reflects that specific key.
   */
  copied: boolean;
  /** The key of the most recently copied item, or null for keyless copies. */
  copiedKey: string | null;
  /**
   * Copy `text` to the clipboard. Pass an optional `key` to track which of
   * several items was copied (mirrors the keyed `copiedKey` UI pattern).
   * Returns true on success, false on failure.
   */
  copy: (text: string, key?: string) => Promise<boolean>;
}

const DEFAULT_RESET_DELAY = 2000;

/**
 * Centralizes the copy-to-clipboard-with-transient-feedback idiom: wraps
 * `navigator.clipboard.writeText`, sets a `copied` flag, and auto-resets it
 * after `resetDelay` ms (clearing the timer on unmount). Supports an optional
 * `key` so a single hook instance can drive keyed feedback (e.g. a list where
 * each row has its own copy button).
 */
export function useCopyToClipboard(
  options: UseCopyToClipboardOptions = {}
): UseCopyToClipboardResult {
  const { resetDelay = DEFAULT_RESET_DELAY, onSuccess, onError } = options;
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const copy = useCallback(
    async (text: string, key?: string): Promise<boolean> => {
      const resolvedKey = key ?? null;
      try {
        if (!navigator.clipboard) {
          throw new Error("Clipboard API unavailable");
        }
        await navigator.clipboard.writeText(text);
        setCopied(true);
        setCopiedKey(resolvedKey);
        onSuccess?.(resolvedKey);
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = setTimeout(() => {
          setCopied(false);
          setCopiedKey(null);
        }, resetDelay);
        return true;
      } catch (error) {
        onError?.(error);
        return false;
      }
    },
    [resetDelay, onSuccess, onError]
  );

  return { copied, copiedKey, copy };
}
