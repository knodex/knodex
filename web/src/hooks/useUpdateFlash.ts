// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useEffect, useRef, useCallback } from "react";

/**
 * Hook to track when a value updates and provide a flash state
 * Useful for visual indication when data changes via WebSocket
 */
export function useUpdateFlash<T>(
  value: T,
  duration: number = 1500
): {
  hasUpdated: boolean;
  triggerFlash: () => void;
} {
  const [hasUpdated, setHasUpdated] = useState(false);
  const prevValueRef = useRef<T>(value);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const triggerFlash = useCallback(() => {
    setHasUpdated(true);

    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    timeoutRef.current = setTimeout(() => {
      setHasUpdated(false);
    }, duration);
  }, [duration]);

  useEffect(() => {
    // Skip initial render
    if (prevValueRef.current === value) {
      return;
    }

    // Deep comparison for objects
    const prevStr = JSON.stringify(prevValueRef.current);
    const currStr = JSON.stringify(value);

    if (prevStr !== currStr) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional flash trigger on value change detection
      triggerFlash();
    }

    prevValueRef.current = value;
  }, [value, triggerFlash]);

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return { hasUpdated, triggerFlash };
}

/**
 * Hook to track multiple items by key and return which ones have updated
 */
export function useMultiUpdateFlash<K extends string>(
  duration: number = 1500
): {
  updatedKeys: Set<K>;
  markUpdated: (key: K) => void;
  clearKey: (key: K) => void;
} {
  const [updatedKeys, setUpdatedKeys] = useState<Set<K>>(new Set());
  const timeoutsRef = useRef<Map<K, ReturnType<typeof setTimeout>>>(new Map());

  const markUpdated = useCallback(
    (key: K) => {
      setUpdatedKeys((prev) => new Set(prev).add(key));

      // Clear existing timeout for this key
      const existing = timeoutsRef.current.get(key);
      if (existing) {
        clearTimeout(existing);
      }

      // Set new timeout
      const timeout = setTimeout(() => {
        setUpdatedKeys((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
        timeoutsRef.current.delete(key);
      }, duration);

      timeoutsRef.current.set(key, timeout);
    },
    [duration]
  );

  const clearKey = useCallback((key: K) => {
    setUpdatedKeys((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });

    const existing = timeoutsRef.current.get(key);
    if (existing) {
      clearTimeout(existing);
      timeoutsRef.current.delete(key);
    }
  }, []);

  // Cleanup all timeouts on unmount
  useEffect(() => {
    const timeouts = timeoutsRef.current;
    return () => {
      timeouts.forEach((timeout) => clearTimeout(timeout));
      timeouts.clear();
    };
  }, []);

  return { updatedKeys, markUpdated, clearKey };
}
