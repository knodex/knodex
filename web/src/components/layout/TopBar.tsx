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
          // z-[60] so the topbar sits ABOVE Radix Dialog overlays (z-50)
          // like the Deploy drawer's backdrop — keeps the breadcrumb crisp
          // instead of dimmed.
          //
          // At lg+ the desktop sidebar is fixed on the left, so the topbar
          // must START at the sidebar's right edge (NOT just pl-* the inner
          // content). When the header spanned `left-0 right-0` it overlaid
          // the sidebar's top 52px — burying the logo and collapse trigger
          // and swallowing their clicks. Anchor the left edge to the
          // sidebar width instead.
          "fixed top-0 left-0 right-0 z-[60] h-[52px] border-b border-[var(--border-subtle)] bg-background/90 backdrop-blur-md",
          isSidebarCollapsed ? "lg:left-16" : "lg:left-[260px]"
        )}
      >
        <div
          className={cn(
            "flex h-full items-center gap-3 px-6 lg:px-10 mx-auto",
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

          {/* Spacer pushes the CmdK trigger toward center-right and keeps the
              right cluster pinned to the edge without absolute positioning. */}
          <div className="flex-1" aria-hidden="true" />

          {/* Inline CmdK trigger — min-w-[220px] keeps the trigger's footprint
              stable so it doesn't crush the breadcrumb at 1024–1100px. */}
          <div className="hidden md:block min-w-[220px]">
            <CmdKTrigger onOpen={onCommandPaletteOpen} />
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
