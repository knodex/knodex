// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "knodex.sidebar.collapsed";

function readInitial(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * Desktop sidebar collapse state, persisted to localStorage.
 * Mobile/tablet drawer state is handled separately by DashboardLayout.
 */
export function useSidebarCollapsed(): {
  isCollapsed: boolean;
  setCollapsed: (next: boolean) => void;
  toggle: () => void;
} {
  const [isCollapsed, setCollapsedState] = useState<boolean>(readInitial);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, isCollapsed ? "1" : "0");
    } catch {
      // localStorage may be unavailable (private mode, quota) — silent no-op.
    }
  }, [isCollapsed]);

  const setCollapsed = useCallback((next: boolean) => setCollapsedState(next), []);
  const toggle = useCallback(() => setCollapsedState((prev) => !prev), []);

  return { isCollapsed, setCollapsed, toggle };
}
