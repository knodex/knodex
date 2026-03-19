// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useState, useCallback } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar, TopBar, Breadcrumbs } from "@/components/layout";
import { ProtectedRoute } from "@/components/auth";
import { WebSocketProvider, useWebSocketContext } from "@/context";
import { Announcer } from "@/components/accessibility";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import { useSessionStatus, useSessionError } from "@/hooks/useAuth";
import { useUserStore } from "@/stores/userStore";
import { Loader2, RefreshCw } from "lucide-react";

function DashboardLayoutInner() {
  const sessionStatus = useSessionStatus();
  const sessionError = useSessionError();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState<boolean>(false);

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

      <Sidebar
        isMobileOpen={isMobileMenuOpen}
        onMobileClose={() => setIsMobileMenuOpen(false)}
      />

      <TopBar onMobileMenuToggle={() => setIsMobileMenuOpen(!isMobileMenuOpen)} />

      <div className="transition-all duration-300 pt-16 ml-0 lg:ml-16">
        <Breadcrumbs />

        <main id="main-content" className="container mx-auto min-h-[calc(100vh-4rem)] px-4 sm:px-6 lg:px-8 py-6">
          {content}
        </main>
      </div>
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
