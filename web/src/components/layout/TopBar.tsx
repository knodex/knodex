// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { LogOut, User, Menu, Sun, Moon } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useAuth } from "@/hooks/useAuth";
import { useSettings } from "@/hooks/useSettings";
import { useTheme } from "@/hooks/useTheme";

interface TopBarProps {
  onMobileMenuToggle?: () => void;
}

const PAGE_TITLES: Record<string, string> = {
  "/catalog": "Catalog",
  "/instances": "Instances",
  "/compliance": "Compliance",
  "/audit": "Audit",
  "/settings": "Settings",
  "/settings/repositories": "Repositories",
  "/settings/projects": "Projects",
  "/settings/sso": "SSO",
  "/settings/audit": "Audit Settings",
  "/user-info": "Account",
};

function getPageTitle(pathname: string): string {
  // Exact match first
  if (PAGE_TITLES[pathname]) return PAGE_TITLES[pathname];

  // Check prefix matches (for nested routes like /catalog/:name, /settings/projects/:name)
  const segments = pathname.split("/").filter(Boolean);
  for (let i = segments.length; i > 0; i--) {
    const prefix = "/" + segments.slice(0, i).join("/");
    if (PAGE_TITLES[prefix]) return PAGE_TITLES[prefix];
  }

  // View routes
  if (pathname.startsWith("/views/")) return "Views";

  return "";
}

export function TopBar({ onMobileMenuToggle }: TopBarProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { logout, user } = useAuth();
  const { isDark, toggleTheme } = useTheme();
  const { data: settings } = useSettings();
  const showOrgName = !!settings?.organization && settings.organization !== 'default';

  const pageTitle = useMemo(() => getPageTitle(location.pathname), [location.pathname]);

  const handleLogout = useCallback(() => {
    logout();
    navigate('/login');
  }, [logout, navigate]);

  return (
    <TooltipProvider>
      <header className="fixed top-0 left-0 right-0 z-30 h-16 bg-background/95 backdrop-blur-sm lg:pl-16">
        <div className="container mx-auto flex h-full items-center justify-between px-4 sm:px-6 lg:px-8">
          {/* Left Section: Mobile Menu + Org Name */}
          <div className="flex items-center gap-3">
            {/* Mobile Menu Button */}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={onMobileMenuToggle}
                  className="lg:hidden text-muted-foreground hover:text-white"
                  aria-label="Toggle menu"
                >
                  <Menu className="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom">
                <p>Menu</p>
              </TooltipContent>
            </Tooltip>

            {/* Page Title */}
            {pageTitle && (
              <h1 className="text-lg font-semibold text-foreground hidden sm:block">
                {pageTitle}
              </h1>
            )}

            {/* Organization Name */}
            {showOrgName && (
              <>
                {pageTitle && <span className="hidden sm:block text-border">|</span>}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span data-testid="org-name" className="hidden sm:inline-block text-sm font-medium text-muted-foreground truncate max-w-[160px]">
                      {settings?.organization}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    <p>{settings?.organization}</p>
                  </TooltipContent>
                </Tooltip>
              </>
            )}
          </div>

          {/* Right Section: User Actions */}
          <div className="flex items-center gap-2">
            {/* Theme Toggle */}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={toggleTheme}
                  className="text-muted-foreground hover:text-white"
                >
                  {isDark ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom">
                <p>{isDark ? "Switch to light mode" : "Switch to dark mode"}</p>
              </TooltipContent>
            </Tooltip>

            {/* User Profile & Logout */}
            {user && (
              <div className="flex items-center gap-2 pl-2">
                <button
                  className="flex items-center gap-2 rounded-md px-1 py-1 hover:bg-accent transition-colors cursor-pointer"
                  onClick={() => navigate('/user-info')}
                  aria-label="View account info"
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
                    <User className="h-4 w-4" />
                  </div>
                  <div className="hidden md:block text-sm">
                    <div className="font-medium text-foreground">{user.email?.split('@')[0]}</div>
                  </div>
                </button>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={handleLogout}
                      className="text-muted-foreground hover:text-white"
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
