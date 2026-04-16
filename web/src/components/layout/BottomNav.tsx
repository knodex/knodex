// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link, useLocation } from "react-router-dom";
import { Box, LayoutGrid, FolderOpen, User } from "@/lib/icons";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  to: string;
  match: (pathname: string) => boolean;
}

const NAV_ITEMS: NavItem[] = [
  {
    label: "Instances",
    icon: Box,
    to: "/instances",
    match: (p) => p.startsWith("/instances"),
  },
  {
    label: "Catalog",
    icon: LayoutGrid,
    to: "/catalog",
    match: (p) => p.startsWith("/catalog"),
  },
  {
    label: "Projects",
    icon: FolderOpen,
    to: "/projects",
    match: (p) => p.startsWith("/projects"),
  },
  {
    label: "Account",
    icon: User,
    to: "/user-info",
    match: (p) => p.startsWith("/user-info"),
  },
];

export function BottomNav() {
  const location = useLocation();

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-50 flex h-16 items-center justify-around border-t border-[rgba(255,255,255,0.08)] bg-[var(--surface-primary)]"
      aria-label="Mobile navigation"
    >
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = item.match(location.pathname);

        return (
          <Link
            key={item.label}
            to={item.to}
            className={cn(
              "relative flex min-h-[44px] min-w-[44px] flex-col items-center justify-center gap-0.5 px-3 py-1 text-[11px] font-medium transition-colors",
              isActive
                ? "text-[var(--brand-primary)]"
                : "text-muted-foreground hover:text-foreground"
            )}
            aria-current={isActive ? "page" : undefined}
            aria-label={item.label}
          >
            <Icon className="h-5 w-5" />
            <span>{item.label}</span>
            {isActive && (
              <span
                className="absolute top-0 h-0.5 w-8 rounded-b bg-[var(--brand-primary)]"
                aria-hidden="true"
              />
            )}
          </Link>
        );
      })}
    </nav>
  );
}
