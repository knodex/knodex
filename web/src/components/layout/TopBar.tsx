// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Menu } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useSettings } from "@/hooks/useSettings";
import { CmdKTrigger } from "@/components/command-palette/cmd-k-trigger";
import { cn } from "@/lib/utils";
import { Breadcrumbs } from "./Breadcrumbs";
import { ProjectSelector } from "./ProjectSelector";

interface TopBarProps {
  onMobileMenuToggle?: () => void;
  /** Opens the command palette. Called when CmdK trigger is clicked. */
  onCommandPaletteOpen: () => void;
  /** Adjusts the left padding to match the desktop sidebar width (260px vs 64px rail). */
  isSidebarCollapsed?: boolean;
}

export function TopBar({ onMobileMenuToggle, onCommandPaletteOpen, isSidebarCollapsed }: TopBarProps) {
  const { data: settings } = useSettings();
  const showOrgName = !!settings?.organization && settings.organization !== "default";

  return (
    <TooltipProvider>
      <header
        className={cn(
          "fixed top-0 left-0 right-0 z-30 h-[52px] border-b border-[var(--border-subtle)] bg-background/90 backdrop-blur-md",
          isSidebarCollapsed ? "lg:pl-16" : "lg:pl-[260px]"
        )}
      >
        <div
          className={cn(
            "relative flex h-full items-center justify-between gap-3 px-6 lg:px-10 mx-auto",
            isSidebarCollapsed ? "max-w-none" : "max-w-[1280px]"
          )}
        >
          {/* Left cluster: hamburger (mobile) + org? + breadcrumbs */}
          <div className="flex items-center gap-2 min-w-0">
            <Button
              variant="ghost"
              size="icon"
              onClick={onMobileMenuToggle}
              className="lg:hidden min-h-[44px] min-w-[44px] text-muted-foreground hover:text-foreground"
              aria-label="Open navigation menu"
              data-testid="topbar-menu-trigger"
            >
              <Menu className="h-5 w-5" />
            </Button>

            {showOrgName && (
              <>
                <span
                  data-testid="org-name"
                  className="hidden sm:inline-block text-[var(--text-size-sm)] text-muted-foreground truncate max-w-[160px]"
                >
                  {settings?.organization}
                </span>
                <span
                  aria-hidden="true"
                  className="hidden sm:inline-block text-muted-foreground/60 select-none"
                >
                  /
                </span>
              </>
            )}

            <Breadcrumbs className="min-w-0" hideHome />
          </div>

          {/* Centered search trigger — absolutely positioned so left/right
              clusters don't pull it off-center. Hidden on narrow widths where
              it would overlap the breadcrumb. */}
          <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 hidden md:block pointer-events-none">
            <div className="pointer-events-auto">
              <CmdKTrigger onOpen={onCommandPaletteOpen} />
            </div>
          </div>

          {/* Right cluster: project filter */}
          <div className="flex items-center gap-2 flex-shrink-0">
            <ProjectSelector />
          </div>
        </div>
      </header>
    </TooltipProvider>
  );
}
