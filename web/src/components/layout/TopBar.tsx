// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { LogOut, User, Menu } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useAuth } from "@/hooks/useAuth";
import { useSettings } from "@/hooks/useSettings";
import { ProjectSelector } from "./ProjectSelector";

interface TopBarProps {
  onMobileMenuToggle?: () => void;
}

const PAGE_TITLES: Record<string, string> = {
  "/catalog": "Catalog",
  "/instances": "Instances",
  "/compliance": "Compliance",
  "/audit": "Audit",
  "/settings": "Settings",
  "/repositories": "Repositories",
  "/projects": "Projects",
  "/settings/sso": "SSO",
  "/settings/license": "License",
  "/settings/audit": "Audit Settings",
  "/secrets": "Secrets",
  "/user-info": "Account",
};

function getPageTitle(pathname: string): string {
  if (PAGE_TITLES[pathname]) return PAGE_TITLES[pathname];
  const segments = pathname.split("/").filter(Boolean);
  for (let i = segments.length; i > 0; i--) {
    const prefix = "/" + segments.slice(0, i).join("/");
    if (PAGE_TITLES[prefix]) return PAGE_TITLES[prefix];
  }
  if (pathname.startsWith("/catalog/categories/")) return "Catalog";
  if (pathname.startsWith("/deploy/")) return "Deploy";
  return "";
}

export function TopBar({ onMobileMenuToggle }: TopBarProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { logout, user } = useAuth();
  const { data: settings } = useSettings();
  const showOrgName = !!settings?.organization && settings.organization !== 'default';
  const pageTitle = useMemo(() => getPageTitle(location.pathname), [location.pathname]);

  const handleLogout = useCallback(() => {
    logout();
    navigate('/login');
  }, [logout, navigate]);

  return (
    <TooltipProvider>
      <header className="fixed top-0 left-0 right-0 z-30 h-14 border-b border-[var(--border-subtle)] bg-background/90 backdrop-blur-md lg:pl-[260px]">
        <div className="relative flex h-full items-center justify-between px-6 lg:px-10 max-w-[1280px] mx-auto">
          {/* Left Section: Mobile Menu + Org */}
          <div className="flex items-center gap-3 min-w-0 flex-shrink-0">
            {/* Hamburger — visible below lg */}
            <Button
              variant="ghost"
              size="icon"
              onClick={onMobileMenuToggle}
              className="lg:hidden min-h-[44px] min-w-[44px] text-muted-foreground hover:text-foreground"
              aria-label="Open navigation menu"
            >
              <Menu className="h-5 w-5" />
            </Button>

            {showOrgName && (
              <>
                <span data-testid="org-name" className="hidden sm:inline-block text-[var(--text-size-sm)] text-muted-foreground truncate max-w-[160px]">
                  {settings?.organization}
                </span>
                <span className="hidden sm:inline-block text-muted-foreground text-[var(--text-size-sm)]">/</span>
              </>
            )}
            <ProjectSelector />
          </div>

          {/* Center: Page Title */}
          {pageTitle && (
            <div className="absolute left-1/2 -translate-x-1/2 pointer-events-none">
              <span className="text-[var(--text-size-sm)] font-medium text-muted-foreground">
                {pageTitle}
              </span>
            </div>
          )}

          {/* Right Section: User */}
          <div className="flex items-center gap-1 flex-shrink-0">
            {user && (
              <div className="flex items-center gap-1">
                <button
                  className="flex items-center gap-2 rounded-[var(--radius-token-md)] px-2.5 py-1.5 text-muted-foreground hover:text-foreground hover:bg-[rgba(255,255,255,0.04)] transition-colors cursor-pointer"
                  onClick={() => navigate('/user-info')}
                  aria-label="View account info"
                >
                  <div className="flex h-6 w-6 items-center justify-center rounded-full bg-[rgba(255,255,255,0.08)]">
                    <User className="h-3.5 w-3.5" />
                  </div>
                  <span className="hidden md:block text-[var(--text-size-sm)]">
                    {user.email?.split('@')[0]}
                  </span>
                </button>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={handleLogout}
                      aria-label="Logout"
                      className="h-8 w-8 text-muted-foreground hover:text-foreground"
                    >
                      <LogOut className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    <p>Logout</p>
                  </TooltipContent>
                </Tooltip>
              </div>
            )}
          </div>
        </div>
      </header>
    </TooltipProvider>
  );
}
