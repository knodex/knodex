// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useRef } from "react";

/**
 * Prefetch route chunks after the browser is idle for a given delay.
 *
 * Uses `requestIdleCallback` (with `setTimeout` fallback) to avoid
 * competing with critical rendering work.
 *
 * @param preloadFns - Array of `.preload()` functions from `lazyWithPreload` components
 * @param delayMs - Milliseconds to wait before scheduling idle prefetch (default: 2000)
 */
export function usePrefetchAfterIdle(
  preloadFns: Array<() => Promise<unknown>>,
  delayMs: number = 2000
): void {
  const calledRef = useRef(false);

  useEffect(() => {
    if (calledRef.current || preloadFns.length === 0) return;

    const scheduleIdle =
      typeof window.requestIdleCallback === "function"
        ? window.requestIdleCallback
        : (cb: () => void) => window.setTimeout(cb, 0);

    const cancelIdle =
      typeof window.cancelIdleCallback === "function"
        ? window.cancelIdleCallback
        : (id: number) => window.clearTimeout(id);

    let idleId: number;

    const timerId = window.setTimeout(() => {
      idleId = scheduleIdle(() => {
        calledRef.current = true;
        preloadFns.forEach((fn) => fn().catch(() => {}));
      });
    }, delayMs);

    return () => {
      window.clearTimeout(timerId);
      if (idleId !== undefined) {
        cancelIdle(idleId);
      }
    };
  }, [delayMs, preloadFns]);
}
