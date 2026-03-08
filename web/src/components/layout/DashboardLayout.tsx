// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useState, useCallback } from "react";
import { Outlet, useNavigate } from "react-router-dom";
import { Sidebar, TopBar, Breadcrumbs } from "@/components/layout";
import { ProtectedRoute } from "@/components/auth";
import { WebSocketProvider, useWebSocketContext } from "@/context";
import { Announcer } from "@/components/accessibility";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import { useAuth } from "@/hooks/useAuth";

function DashboardLayoutInner() {
  const navigate = useNavigate();
  const { logout, isTokenExpired } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState<boolean>(false);

  // Accessibility: announcements
  const { announcements, announce, handleAnnouncementRead } = useAnnouncements();
  const { status: wsStatus, error: wsError } = useWebSocketContext();

  // Check token expiry periodically (JWT is in HttpOnly cookie, check stored expiry)
  useEffect(() => {
    const interval = setInterval(() => {
      if (isTokenExpired()) {
        logout();
        navigate('/login');
      }
    }, 60000); // Check every minute

    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only run on mount - Zustand store actions are stable

  // Sanitize messages to prevent XSS
  const sanitizeMessage = useCallback((msg: string): string => {
    // Strip HTML tags and limit length
    return msg.replace(/<[^>]*>/g, '').substring(0, 200);
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

  return (
    <div className="min-h-screen bg-background">
      {/* Accessibility: Screen reader announcements */}
      <Announcer
        announcements={announcements}
        onAnnouncementRead={handleAnnouncementRead}
      />

      {/* Skip link */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground"
      >
        Skip to main content
      </a>

      {/* Sidebar Navigation */}
      <Sidebar
        isMobileOpen={isMobileMenuOpen}
        onMobileClose={() => setIsMobileMenuOpen(false)}
      />

      {/* Top Bar */}
      <TopBar onMobileMenuToggle={() => setIsMobileMenuOpen(!isMobileMenuOpen)} />

      {/* Main Content Area - Always offset by collapsed sidebar width */}
      <div className="transition-all duration-300 pt-16 ml-0 lg:ml-16">
        {/* Breadcrumbs */}
        <Breadcrumbs />

        {/* Main Content - child routes render here */}
        <main id="main-content" className="container mx-auto min-h-[calc(100vh-4rem)] px-4 sm:px-6 lg:px-8 py-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export function DashboardLayout() {
  return (
    <ProtectedRoute>
      <WebSocketProvider debug={import.meta.env.DEV}>
        <DashboardLayoutInner />
      </WebSocketProvider>
    </ProtectedRoute>
  );
}
