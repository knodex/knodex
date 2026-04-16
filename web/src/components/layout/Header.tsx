// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ExternalLink, Box, LayoutGrid, LogOut } from "@/lib/icons";
import { ConnectionStatus } from "@/components/ConnectionStatus";
import { NamespaceSelector } from "./NamespaceSelector";
import { useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type NavTab = "catalog" | "instances";

interface HeaderProps {
  activeTab: NavTab;
  onTabChange: (tab: NavTab) => void;
  globalNamespace: string;
  onGlobalNamespaceChange: (namespace: string) => void;
  availableNamespaces: string[];
}

export function Header({
  activeTab,
  onTabChange,
  globalNamespace,
  onGlobalNamespaceChange,
  availableNamespaces,
}: HeaderProps) {
  const navigate = useNavigate();
  const { logout, user } = useAuth();

  const handleLogout = useCallback(() => {
    logout();
    navigate('/login');
  }, [logout, navigate]);

  return (
    <header className="sticky top-0 z-40 bg-background">
      <div className="max-w-7xl mx-auto flex items-center justify-between h-16 px-4 sm:px-6">
        <div className="flex items-center gap-8">
          <div className="flex items-center gap-3">
            <img src="/favicon.svg" alt="Knodex" className="h-10 w-10" />
            <span className="text-lg font-semibold text-foreground">Knodex</span>
          </div>

          {/* Navigation tabs */}
          <nav className="hidden sm:flex items-center gap-2" aria-label="Primary">
            <button
              onClick={() => onTabChange("catalog")}
              className={cn(
                "flex items-center gap-1.5 h-11 px-4 rounded-md text-sm font-medium transition-colors",
                activeTab === "catalog"
                  ? "bg-secondary text-foreground"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
              )}
              aria-current={activeTab === "catalog" ? "page" : undefined}
            >
              <LayoutGrid className="h-4 w-4" />
              Catalog
            </button>
            <button
              onClick={() => onTabChange("instances")}
              className={cn(
                "flex items-center gap-1.5 h-11 px-4 rounded-md text-sm font-medium transition-colors",
                activeTab === "instances"
                  ? "bg-secondary text-foreground"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
              )}
              aria-current={activeTab === "instances" ? "page" : undefined}
            >
              <Box className="h-4 w-4" />
              Instances
            </button>
          </nav>
        </div>

        <div className="flex items-center gap-4">
          <NamespaceSelector
            value={globalNamespace}
            onChange={onGlobalNamespaceChange}
            namespaces={availableNamespaces}
          />

          <ConnectionStatus />

          <Tooltip>
            <TooltipTrigger asChild>
              <a
                href="https://knodex.io/docs"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <span className="hidden sm:inline">Docs</span>
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </TooltipTrigger>
            <TooltipContent>
              <p>Knodex Documentation</p>
            </TooltipContent>
          </Tooltip>

          {user && (
            <div className="flex items-center gap-2 pl-2">
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="text-sm text-muted-foreground hidden sm:inline truncate max-w-[200px]">
                    {user.email}
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{user.email}</p>
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleLogout}
                    className="flex items-center gap-1.5"
                  >
                    <LogOut className="h-4 w-4" />
                    <span className="hidden sm:inline">Logout</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Sign out</p>
                </TooltipContent>
              </Tooltip>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
