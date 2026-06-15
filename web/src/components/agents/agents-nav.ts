// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Bot, Boxes, FileCode, LayoutGrid } from "@/lib/icons";
import type { LucideIcon } from "@/lib/icons";

/**
 * Agents workspace menu model for the master-detail shell (AgentsLayout).
 * Component-free (mirrors settings-nav.ts) so the layout file only exports a
 * component and stays react-refresh clean. Fixed tabs:
 *   Overview (/agents) · Agents (/agents/list) · Templates (/agents/templates)
 *   · Models (/agents/models).
 */
export interface AgentsNavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  to: string;
  /** When true, the item is active only on an exact path match (Overview). */
  exact?: boolean;
}

export function getAgentsNavItems(): AgentsNavItem[] {
  return [
    // Overview is `exact` so /agents doesn't claim /agents/list via prefix.
    { id: "overview", label: "Overview", icon: LayoutGrid, to: "/agents", exact: true },
    { id: "agents", label: "Agents", icon: Bot, to: "/agents/list" },
    { id: "templates", label: "Templates", icon: FileCode, to: "/agents/templates" },
    { id: "models", label: "Models", icon: Boxes, to: "/agents/models" },
  ];
}

/**
 * resolveActiveAgentsId returns the id of the menu item owning the current
 * path, or null. Exact `to` match wins; otherwise the LONGEST segment-boundary
 * prefix (so /agents/list keeps the Agents tab active on a deeper chat route,
 * and Overview's exact /agents never claims /agents/list).
 */
export function resolveActiveAgentsId(pathname: string, items: AgentsNavItem[]): string | null {
  for (const item of items) {
    if (pathname === item.to) return item.id;
  }
  let best: AgentsNavItem | null = null;
  for (const item of items) {
    if (item.exact) continue;
    if (pathname.startsWith(item.to + "/") && (!best || item.to.length > best.to.length)) {
      best = item;
    }
  }
  return best?.id ?? null;
}
