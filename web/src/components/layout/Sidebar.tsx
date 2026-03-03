import { useState, useEffect, useCallback, useMemo } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  LayoutGrid,
  Box,
  ExternalLink,
  X,
  Settings,
  ShieldCheck,
  ScrollText,
  ChevronDown,
  FolderKanban,
} from "lucide-react";
import type { LucideProps } from "lucide-react";
import { cn } from "@/lib/utils";
import { getLucideIcon } from "@/lib/icons";
import { useRGDCount, useInstanceCount } from "@/hooks";
import { useViolationCount, isEnterprise } from "@/hooks/useCompliance";
import { useViewsEnabled } from "@/hooks/useViews";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type NavTab = "catalog" | "instances" | "compliance" | "settings" | string;

interface SidebarProps {
  onCollapseChange?: (isCollapsed: boolean) => void;
  isMobileOpen?: boolean;
  onMobileClose?: () => void;
}

interface NavItem {
  id: NavTab;
  label: string;
  icon: React.ComponentType<LucideProps>;
  badge?: number;
  to: string;
}

// Key for localStorage persistence
const VIEWS_COLLAPSED_KEY = "sidebar-views-collapsed";

export function Sidebar({
  onCollapseChange,
  isMobileOpen = false,
  onMobileClose,
}: SidebarProps) {
  const location = useLocation();
  // Sidebar is collapsed by default, expands on hover
  const [isHovered, setIsHovered] = useState(false);

  // Views section collapse state (persisted to localStorage)
  const [isViewsCollapsed, setIsViewsCollapsed] = useState(() => {
    const stored = localStorage.getItem(VIEWS_COLLAPSED_KEY);
    return stored === "true";
  });

  // Expand sidebar on hover (desktop only)
  const handleMouseEnter = useCallback(() => {
    setIsHovered(true);
  }, []);

  const handleMouseLeave = useCallback(() => {
    setIsHovered(false);
  }, []);

  // Sidebar is expanded when hovered
  const isExpanded = isHovered;

  // Get custom views (EE feature)
  const { views: customViews } = useViewsEnabled();

  // Derive active tab from current route
  // Check for view routes: /views/{slug}
  const viewSlugMatch = location.pathname.match(/^\/views\/([^/]+)/);
  const activeTab: NavTab =
    location.pathname.startsWith('/settings') ? 'settings' :
    location.pathname.startsWith('/audit') ? 'audit' :
    location.pathname.startsWith('/compliance') ? 'compliance' :
    location.pathname.startsWith('/instances') ? 'instances' :
    viewSlugMatch ? `view-${viewSlugMatch[1]}` :
    'catalog';

  // Auto-expand views section when navigating TO a view
  // Only runs when the view slug changes, not when collapse state changes
  useEffect(() => {
    if (viewSlugMatch) {
      setIsViewsCollapsed(false);
      localStorage.setItem(VIEWS_COLLAPSED_KEY, "false");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [viewSlugMatch?.[1]]); // Only depend on the slug value, not the collapse state

  const handleNavItemClick = () => {
    // Close mobile sidebar when navigation item is clicked
    onMobileClose?.();
  };

  const toggleViewsSection = useCallback(() => {
    setIsViewsCollapsed((prev) => {
      const newValue = !prev;
      localStorage.setItem(VIEWS_COLLAPSED_KEY, String(newValue));
      return newValue;
    });
  }, []);

  // Close sidebar on escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isMobileOpen) {
        onMobileClose?.();
      }
    };

    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isMobileOpen, onMobileClose]);

  // Notify parent of collapse state changes
  useEffect(() => {
    onCollapseChange?.(true); // Always collapsed for layout purposes (overlay mode)
  }, [onCollapseChange]);

  // Get RGD and Instance counts for badges using lightweight count endpoints
  const { data: rgdCountData } = useRGDCount();
  const { data: instanceCountData } = useInstanceCount();
  const { data: violationCount } = useViolationCount();

  const rgdCount = rgdCountData?.count ?? 0;
  const instanceCount = instanceCountData?.count ?? 0;

  // Build view nav items from config (EE feature)
  const viewNavItems: NavItem[] = useMemo(() => {
    if (!customViews || customViews.length === 0) {
      return [];
    }
    return customViews.map((view) => ({
      id: `view-${view.slug}`,
      label: view.name,
      icon: getLucideIcon(view.icon),
      badge: view.count,
      to: `/views/${view.slug}`,
    }));
  }, [customViews]);

  // Core navigation items (always visible)
  const coreNavItems: NavItem[] = [
    { id: "catalog", label: "Catalog", icon: LayoutGrid, badge: rgdCount, to: "/catalog" },
    { id: "instances", label: "Instances", icon: Box, badge: instanceCount, to: "/instances" },
  ];

  // Enterprise-only items after views
  const enterpriseNavItems: NavItem[] = isEnterprise() ? [
    {
      id: "audit" as const,
      label: "Audit",
      icon: ScrollText,
      to: "/audit",
    },
    {
      id: "compliance" as const,
      label: "Compliance",
      icon: ShieldCheck,
      badge: violationCount,
      to: "/compliance",
    },
  ] : [];

  // Render a single nav item
  const renderNavItem = (item: NavItem, indented: boolean = false) => {
    const Icon = item.icon;
    const isActive = activeTab === item.id;

    const linkContent = (
      <Link
        key={item.id}
        to={item.to}
        onClick={handleNavItemClick}
        className={cn(
          "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-150",
          isActive
            ? "bg-sidebar-accent/15 text-sidebar-foreground border-l-2 border-primary"
            : "text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground",
          !isExpanded && "justify-center px-2",
          indented && isExpanded && "ml-2"
        )}
        aria-label={item.label}
        aria-current={isActive ? "page" : undefined}
      >
        <Icon className={cn("flex-shrink-0", indented ? "h-4 w-4" : "h-5 w-5")} />

        {isExpanded && (
          <>
            <span className="flex-1 text-left whitespace-nowrap overflow-hidden text-ellipsis">
              {item.label}
            </span>
            {item.badge !== undefined && item.badge > 0 && (
              <span
                className={cn(
                  "flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-medium",
                  isActive
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground"
                )}
              >
                {item.badge}
              </span>
            )}
          </>
        )}
      </Link>
    );

    if (!isExpanded) {
      return (
        <Tooltip key={item.id}>
          <TooltipTrigger asChild>{linkContent}</TooltipTrigger>
          <TooltipContent side="right">
            <p>{item.label}{item.badge !== undefined && item.badge > 0 ? ` (${item.badge})` : ''}</p>
          </TooltipContent>
        </Tooltip>
      );
    }

    return linkContent;
  };

  // Render views section header
  const renderViewsSectionHeader = () => {
    if (viewNavItems.length === 0) return null;

    const headerContent = (
      <button
        onClick={toggleViewsSection}
        className={cn(
          "w-full flex items-center gap-2 px-3 py-2 text-xs font-semibold uppercase tracking-wider",
          "text-sidebar-foreground/50 hover:text-sidebar-foreground/70 transition-colors",
          !isExpanded && "justify-center px-2"
        )}
        aria-expanded={!isViewsCollapsed}
        aria-controls="view-nav-items"
        aria-label={`Views section, ${viewNavItems.length} views, ${isViewsCollapsed ? 'collapsed' : 'expanded'}`}
      >
        {isExpanded ? (
          <>
            <ChevronDown
              className={cn(
                "h-3 w-3 transition-transform duration-200",
                isViewsCollapsed && "-rotate-90"
              )}
              aria-hidden="true"
            />
            <span className="flex-1 text-left">Views</span>
          </>
        ) : (
          <FolderKanban className="h-4 w-4" aria-hidden="true" />
        )}
      </button>
    );

    if (!isExpanded) {
      return (
        <Tooltip>
          <TooltipTrigger asChild>{headerContent}</TooltipTrigger>
          <TooltipContent side="right" className="flex flex-col gap-1">
            <p className="font-semibold">Views</p>
            {viewNavItems.map((item) => (
              <p key={item.id} className="text-xs text-muted-foreground">
                {item.label} ({item.badge ?? 0})
              </p>
            ))}
          </TooltipContent>
        </Tooltip>
      );
    }

    return headerContent;
  };

  // Render flyout menu for collapsed sidebar
  const [showFlyout, setShowFlyout] = useState(false);
  const [flyoutTimeout, setFlyoutTimeout] = useState<NodeJS.Timeout | null>(null);

  const handleFlyoutEnter = useCallback(() => {
    if (flyoutTimeout) {
      clearTimeout(flyoutTimeout);
      setFlyoutTimeout(null);
    }
    setShowFlyout(true);
  }, [flyoutTimeout]);

  const handleFlyoutLeave = useCallback(() => {
    const timeout = setTimeout(() => {
      setShowFlyout(false);
    }, 150);
    setFlyoutTimeout(timeout);
  }, []);

  // Cleanup flyout timeout on unmount
  useEffect(() => {
    return () => {
      if (flyoutTimeout) {
        clearTimeout(flyoutTimeout);
      }
    };
  }, [flyoutTimeout]);

  return (
    <>
      {/* Mobile Backdrop */}
      {isMobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm lg:hidden"
          onClick={onMobileClose}
          aria-hidden="true"
        />
      )}

      {/* Sidebar - Overlay mode: expands on hover */}
      <aside
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
          className={cn(
            "fixed left-0 top-0 z-50 h-screen bg-sidebar border-r border-border transition-all duration-200 ease-in-out",
            // Mobile: overlay mode, hidden by default
            "w-64",
            isMobileOpen ? "translate-x-0" : "-translate-x-full",
            // Desktop: always visible, collapsed by default, expands on hover
            "lg:translate-x-0",
            isExpanded ? "lg:w-56" : "lg:w-16"
          )}
        >
          <div className="flex h-full flex-col">
          {/* Logo */}
          <div className="flex h-16 items-center px-4">
            <div className="flex items-center gap-3">
              <img src="/logo.svg" alt="Knodex" className="h-10 w-10" />
              {isExpanded && (
                <span className="text-sm font-semibold text-sidebar-foreground whitespace-nowrap overflow-hidden">
                  Knodex
                </span>
              )}
            </div>

            {/* Mobile Close Button */}
            <button
              onClick={onMobileClose}
              className="lg:hidden ml-auto text-sidebar-foreground/60 hover:text-sidebar-foreground transition-colors"
              aria-label="Close menu"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* Primary Navigation */}
          <nav className="flex-1 overflow-y-auto px-2 py-4" aria-label="Main navigation">
            {/* Core Nav Items */}
            <div className="space-y-1">
              {coreNavItems.map((item) => renderNavItem(item))}
            </div>

            {/* Views Section */}
            {viewNavItems.length > 0 && (
              <div className="mt-4">
                {/* Section Divider */}
                {isExpanded && (
                  <div className="mx-3 mb-2 border-t border-sidebar-border" />
                )}

                {/* Views Header */}
                <div
                  className="relative"
                  onMouseEnter={!isExpanded ? handleFlyoutEnter : undefined}
                  onMouseLeave={!isExpanded ? handleFlyoutLeave : undefined}
                >
                  {renderViewsSectionHeader()}

                  {/* Flyout Menu (collapsed sidebar only) */}
                  {!isExpanded && showFlyout && (
                    <div
                      className="absolute left-full top-0 ml-2 min-w-48 bg-sidebar border border-sidebar-border rounded-lg shadow-lg py-2 z-[60]"
                      role="menu"
                      aria-label="Views navigation"
                      onMouseEnter={handleFlyoutEnter}
                      onMouseLeave={handleFlyoutLeave}
                    >
                      <div className="px-3 py-1.5 text-xs font-semibold uppercase tracking-wider text-sidebar-foreground/50 border-b border-sidebar-border mb-1">
                        Views
                      </div>
                      {viewNavItems.map((item) => {
                        const Icon = item.icon;
                        const isActive = activeTab === item.id;
                        return (
                          <Link
                            key={item.id}
                            to={item.to}
                            onClick={handleNavItemClick}
                            role="menuitem"
                            className={cn(
                              "flex items-center gap-2 px-3 py-2 text-sm transition-colors",
                              isActive
                                ? "bg-sidebar-accent/15 text-sidebar-foreground border-l-2 border-primary"
                                : "text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground"
                            )}
                          >
                            <Icon className="h-4 w-4 flex-shrink-0" />
                            <span className="flex-1">{item.label}</span>
                            {item.badge !== undefined && item.badge > 0 && (
                              <span className="text-xs text-sidebar-foreground/50">
                                {item.badge}
                              </span>
                            )}
                          </Link>
                        );
                      })}
                    </div>
                  )}
                </div>

                {/* View Items (expanded sidebar, section not collapsed) */}
                {isExpanded && !isViewsCollapsed && (
                  <div
                    id="view-nav-items"
                    className="mt-1 space-y-0.5 overflow-hidden"
                    role="group"
                    aria-label="Custom views"
                  >
                    {viewNavItems.map((item) => renderNavItem(item, true))}
                  </div>
                )}
              </div>
            )}

            {/* Enterprise Nav Items */}
            {enterpriseNavItems.length > 0 && (
              <div className={cn("space-y-1", viewNavItems.length > 0 ? "mt-4" : "mt-1")}>
                {isExpanded && viewNavItems.length > 0 && (
                  <div className="mx-3 mb-2 border-t border-sidebar-border" />
                )}
                {enterpriseNavItems.map((item) => renderNavItem(item))}
              </div>
            )}
          </nav>

          {/* Settings Section - Always visible, API handles 403 for unauthorized access */}
          <div className="px-2 py-2 border-t border-sidebar-border">
            {!isExpanded ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Link
                    to="/settings"
                    onClick={handleNavItemClick}
                    className={cn(
                      "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-150",
                      activeTab === "settings"
                        ? "bg-sidebar-accent/15 text-sidebar-foreground border-l-2 border-primary"
                        : "text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground",
                      !isExpanded && "justify-center px-2"
                    )}
                    aria-label="Settings"
                    aria-current={activeTab === "settings" ? "page" : undefined}
                  >
                    <Settings className="h-5 w-5 flex-shrink-0" />
                  </Link>
                </TooltipTrigger>
                <TooltipContent side="right">
                  <p>Settings</p>
                </TooltipContent>
              </Tooltip>
            ) : (
              <Link
                to="/settings"
                onClick={handleNavItemClick}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-150",
                  activeTab === "settings"
                    ? "bg-sidebar-accent/15 text-sidebar-foreground border-l-2 border-primary"
                    : "text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground",
                  !isExpanded && "justify-center px-2"
                )}
                aria-label="Settings"
                aria-current={activeTab === "settings" ? "page" : undefined}
              >
                <Settings className="h-5 w-5 flex-shrink-0" />
                {isExpanded && (
                  <span className="flex-1 text-left whitespace-nowrap overflow-hidden">
                    Settings
                  </span>
                )}
              </Link>
            )}
          </div>

          {/* Footer */}
          <div className="px-2 pb-4">
            {!isExpanded ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <a
                    href="https://knodex.io/docs"
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="Documentation"
                    className={cn(
                      "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground transition-colors",
                      !isExpanded && "justify-center px-2"
                    )}
                  >
                    <ExternalLink className="h-5 w-5 flex-shrink-0" />
                  </a>
                </TooltipTrigger>
                <TooltipContent side="right">
                  <p>Documentation</p>
                </TooltipContent>
              </Tooltip>
            ) : (
              <a
                href="https://knodex.io/docs"
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Documentation"
                className={cn(
                  "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-sidebar-foreground/70 hover:bg-muted/50 hover:text-sidebar-foreground transition-colors",
                  !isExpanded && "justify-center px-2"
                )}
              >
                <ExternalLink className="h-5 w-5 flex-shrink-0" />
                {isExpanded && (
                  <span className="whitespace-nowrap overflow-hidden">Documentation</span>
                )}
              </a>
            )}
          </div>
        </div>
      </aside>
    </>
  );
}
