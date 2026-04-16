// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useRef, useState, useCallback, useMemo } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Sidebar, TopBar, Breadcrumbs } from "@/components/layout";
import { SidebarDrawer } from "@/components/layout/SidebarDrawer";
import { ProtectedRoute } from "@/components/auth";
import { WebSocketProvider, useWebSocketContext } from "@/context";
import { Announcer } from "@/components/accessibility";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import { usePrefetchAfterIdle } from "@/hooks/usePrefetchAfterIdle";
import { useIsTablet } from "@/hooks/useIsTablet";
import { useIsMobile } from "@/hooks/useIsMobile";
import { BottomNav } from "@/components/layout/BottomNav";
import { routePreloads } from "@/lib/route-preloads";
import { useSessionStatus, useSessionError } from "@/hooks/useAuth";
import { useUserStore } from "@/stores/userStore";
import { CommandPalette } from "@/components/command-palette/command-palette";
import { useCommandPaletteShortcut } from "@/components/command-palette/use-command-palette-shortcut";
import { Loader2, RefreshCw } from "@/lib/icons";
import { cn } from "@/lib/utils";

function DashboardLayoutInner() {
  const sessionStatus = useSessionStatus();
  const sessionError = useSessionError();
  const location = useLocation();
  const [isDrawerOpen, setIsDrawerOpen] = useState<boolean>(false);
  const isTablet = useIsTablet();
  const isMobile = useIsMobile();
  const { open: commandPaletteOpen, setOpen: setCommandPaletteOpen } = useCommandPaletteShortcut();

  // Set data-layout attribute on body for CSS targeting
  useEffect(() => {
    if (isMobile) {
      document.body.setAttribute("data-layout", "mobile");
    } else if (isTablet) {
      document.body.setAttribute("data-layout", "tablet");
    } else {
      document.body.removeAttribute("data-layout");
    }
    return () => document.body.removeAttribute("data-layout");
  }, [isTablet, isMobile]);

  // Focus management: move focus to page h1 on route change for keyboard/screen reader users
  const isFirstRender = useRef(true);
  useEffect(() => {
    // Skip initial mount — don't steal focus on first page load
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    // Small delay to let the new route render its h1
    const timer = setTimeout(() => {
      const h1 = document.querySelector('h1');
      if (h1 instanceof HTMLElement) {
        h1.focus();
      } else {
        // Fallback: focus main content area when no h1 exists
        const main = document.getElementById('main-content');
        if (main instanceof HTMLElement) {
          main.focus();
        }
      }
    }, 100);
    return () => clearTimeout(timer);
  }, [location.pathname]);

  // Prefetch most common routes after idle (Catalog + Instances)
  const idlePreloads = useMemo(
    () => [routePreloads["/catalog"], routePreloads["/instances"]].filter(Boolean),
    []
  );
  usePrefetchAfterIdle(idlePreloads);

  // Accessibility: announcements
  const { announcements, announce, handleAnnouncementRead } = useAnnouncements();
  const { status: wsStatus, error: wsError } = useWebSocketContext();

  // Strip HTML tags and truncate for accessibility announcements
  const sanitizeMessage = useCallback((msg: string): string => {
    // Loop until stable to prevent bypass via nested tags (e.g., "<<script>script>")
    let result = msg;
    let prev: string;
    do {
      prev = result;
      result = result.replace(/<[^>]*>/g, '');
    } while (result !== prev);
    return result.substring(0, 200);
  }, []);

  // Announce WebSocket connection status changes
  useEffect(() => {
    if (wsStatus === "connected") {
      announce("Connected to server", "polite");
    } else if (wsStatus === "error" || wsError) {
      announce(sanitizeMessage(wsError || "Connection error"), "assertive");
    } else if (wsStatus === "disconnected") {
      announce("Disconnected from server", "polite");
    }
  }, [wsStatus, wsError, announce, sanitizeMessage]);

  const handleRetry = useCallback(() => {
    useUserStore.getState().setSessionStatus('idle');
  }, []);

  // Content area: gated by session status
  let content: React.ReactNode;
  if (sessionStatus === 'idle' || sessionStatus === 'validating') {
    content = (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  } else if (sessionStatus === 'error') {
    content = (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <div className="text-center space-y-2">
          <p className="text-lg font-medium text-destructive">Connection Error</p>
          <p className="text-sm text-muted-foreground">{sessionError || 'Unable to connect to server.'}</p>
        </div>
        <button
          onClick={handleRetry}
          className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          <RefreshCw className="h-4 w-4" />
          Retry
        </button>
      </div>
    );
  } else {
    content = <Outlet />;
  }

  return (
    <div className="min-h-screen bg-background">
      <Announcer
        announcements={announcements}
        onAnnouncementRead={handleAnnouncementRead}
      />

      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground"
      >
        Skip to main content
      </a>

      {!isMobile && <Sidebar />}

      {!isMobile && <SidebarDrawer open={isDrawerOpen} onOpenChange={setIsDrawerOpen} />}

      <CommandPalette open={commandPaletteOpen} onOpenChange={setCommandPaletteOpen} />

      {!isMobile && <TopBar onMobileMenuToggle={() => setIsDrawerOpen(true)} />}

      <div className={cn(
        "transition-all duration-300",
        isMobile ? "pt-0 pb-16" : "pt-14 ml-0 lg:ml-[260px]"
      )}>
        {!isMobile && <Breadcrumbs />}

        <main
          key={location.pathname}
          id="main-content"
          tabIndex={-1}
          className={cn(
            "outline-none",
            "animate-token-fade-in mx-auto min-h-[calc(100vh-4rem)] max-w-[1280px]",
            isMobile ? "px-4 py-4 overflow-x-hidden" : "px-6 pt-4 pb-8 lg:px-10"
          )}
        >
          {content}
        </main>
      </div>

      {isMobile && <BottomNav />}
    </div>
  );
}

export function DashboardLayout() {
  const sessionStatus = useSessionStatus();

  return (
    <ProtectedRoute>
      <WebSocketProvider debug={import.meta.env.DEV} autoConnect={sessionStatus === 'valid'}>
        <DashboardLayoutInner />
      </WebSocketProvider>
    </ProtectedRoute>
  );
}
