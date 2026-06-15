// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import { getSettingsNavItems, groupSettingsNavItems, resolveActiveSettingsId } from "./settings-nav";

/**
 * SettingsLayout — Claude-style master-detail shell for the Settings area.
 *
 * Renders the "Settings" title, a persistent vertical menu, and the active
 * sub-page in a content panel. The app sidebar stays on the far left
 * (DashboardLayout); this shell lives inside the routed content area.
 *
 * Each settings route wraps its page in this shell (wrapper approach rather than
 * a nested `<Outlet/>`), so the cloud-tenant dynamic-import seam in App.tsx and
 * its `cloudLoading` fallback remain unchanged. Cloud pages import this module
 * directly (cloud → shared is allowed; the reverse is not). The menu model
 * lives in `./settings-nav` (kept separate so this file only exports a
 * component, satisfying react-refresh).
 */
export function SettingsLayout({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const items = getSettingsNavItems();
  const groups = groupSettingsNavItems(items);
  const activeId = resolveActiveSettingsId(pathname, items);

  return (
    <div className="py-6">
      <div className="flex flex-col gap-8 md:flex-row">
        {/* Settings menu — vertical on md+, horizontal-scroll row on small screens */}
        <nav
          aria-label="Settings sections"
          className={cn(
            "shrink-0 md:w-[180px]",
            "md:sticky md:top-[72px] md:self-start",
            "flex gap-1 overflow-x-auto pb-2 md:flex-col md:overflow-x-visible md:pb-0",
          )}
        >
          {groups.map((group) => (
            // Each group is a row on small screens (so the whole nav stays a
            // horizontal scroll) and a labelled column on md+.
            <div
              key={group.id ?? "general"}
              role="group"
              aria-label={group.label ?? undefined}
              className="flex gap-1 md:flex-col"
            >
              {group.label && (
                <h2 className="hidden px-3 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wider text-[var(--text-secondary)] opacity-70 md:block">
                  {group.label}
                </h2>
              )}
              {group.items.map((item) => {
                const Icon = item.icon;
                const active = item.id === activeId;
                return (
                  <Link
                    key={item.id}
                    to={item.to}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "flex items-center gap-3 whitespace-nowrap rounded-[var(--radius-token-md)] px-3 py-2 text-sm font-medium transition-colors duration-150",
                      active
                        ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
                        : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]",
                    )}
                  >
                    <Icon className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
                    {item.label}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        {/* Content panel */}
        <div className="min-w-0 flex-1">{children}</div>
      </div>
    </div>
  );
}

export default SettingsLayout;
