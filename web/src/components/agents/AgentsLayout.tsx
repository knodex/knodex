// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";

/**
 * AgentsLayout — content shell for the Agents workspace. Navigation
 * (Overview/Agents/Models) lives in the left secondary sidebar (Sidebar.tsx,
 * agents sub-nav), mirroring the Catalog workspace — so this is now a thin
 * padded container, not a master-detail nav shell.
 *
 * Kept as a wrapper (rather than removed) so App.tsx route wiring and the
 * LazyRouteElement lazy seam stay unchanged.
 */
export function AgentsLayout({ children }: { children: ReactNode }) {
  return <div className="min-w-0">{children}</div>;
}

export default AgentsLayout;
