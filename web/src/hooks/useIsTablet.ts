// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useEffect } from "react";

const TABLET_QUERY = "(min-width: 768px) and (max-width: 1023px)";

/**
 * Returns true when the viewport matches the tablet breakpoint (768-1023px).
 */
export function useIsTablet(): boolean {
  const [isTablet, setIsTablet] = useState(() =>
    typeof window !== "undefined" ? window.matchMedia(TABLET_QUERY).matches : false
  );

  useEffect(() => {
    const mql = window.matchMedia(TABLET_QUERY);
    const handler = (e: MediaQueryListEvent) => setIsTablet(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, []);

  return isTablet;
}
