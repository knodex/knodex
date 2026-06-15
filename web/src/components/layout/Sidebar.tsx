// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useCallback, useEffect, useMemo, useRef } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  ChevronLeft,
  ChevronDown,
  FileText,
  Shield,
  AlertTriangle,
  User,
  Settings,
  ExternalLink,
  LogOut,
  PanelLeft,
} from "@/lib/icons";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { LucideProps } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { getLucideIcon } from "@/lib/icons";
import { NAV_ITEMS } from "@/lib/nav-items";
import { getAgentsNavItems, resolveActiveAgentsId } from "@/components/agents/agents-nav";
import { routePreloads } from "@/lib/route-preloads";
import { useRGDList } from "@/hooks/useRGDs";
import { useViolationCount, useComplianceSummary, isEnterprise } from "@/hooks/useCompliance";
import { useCategoriesEnabled } from "@/hooks/useCategories";
import { useCanI } from "@/hooks/useCanI";
import { useAuth } from "@/hooks/useAuth";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type NavTab = "catalog" | "instances" | "compliance" | "projects" | "repositories" | string;

interface SidebarNavProps {
  onNavItemClick?: () => void;
  /** Optional toggle for collapse/expand — renders a PanelLeft button next to the Knodex logo when provided. */
  onToggleCollapse?: () => void;
  /** When true, render the compact icon-rail layout. */
  isCollapsed?: boolean;
}

function LogoHeader({
  onToggleCollapse,
  isCollapsed,
}: {
  onToggleCollapse?: () => void;
  isCollapsed?: boolean;
}) {
  const toggleRef = useRef<HTMLButtonElement | null>(null);
  const prevCollapsed = useRef(isCollapsed);

  // Restore focus to the toggle button on collapse-state change so keyboard
  // users don't lose position when a focused nav item is unmounted by the
  // expanded ↔ rail switch.
  useEffect(() => {
    if (prevCollapsed.current !== isCollapsed) {
      prevCollapsed.current = isCollapsed;
      const active = document.activeElement;
      if (!active || active === document.body) {
        toggleRef.current?.focus();
      }
    }
  }, [isCollapsed]);

  const toggleButton = onToggleCollapse && (
    <button
      ref={toggleRef}
      type="button"
      onClick={onToggleCollapse}
      aria-label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      aria-expanded={!isCollapsed}
      data-testid="sidebar-collapse-trigger"
      className={cn(
        "flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-[var(--radius-token-md)]",
        "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]",
        "transition-colors duration-150 outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30"
      )}
    >
      <PanelLeft className="h-4 w-4" aria-hidden="true" />
    </button>
  );

  if (isCollapsed) {
    // Collapsed rail: just the toggle button, centered.
    return (
      <div className="flex h-16 items-center justify-center px-2">
        {toggleButton}
      </div>
    );
  }

  return (
    <div className="flex h-16 items-center justify-between px-4">
      <div className="flex items-center gap-3 min-w-0">
        <img src="/logo.svg" alt="Knodex" className="h-10 w-10 shrink-0" />
        <span className="text-sm font-semibold text-[var(--text-primary)] whitespace-nowrap overflow-hidden">
          Knodex
        </span>
      </div>
      {toggleButton}
    </div>
  );
}

function CollapsedNavIcon({
  item,
  isActive,
  onClick,
  onPreload,
}: {
  item: NavItem;
  isActive: boolean;
  onClick: () => void;
  onPreload: (to: string) => void;
}) {
  const Icon = item.icon;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to={item.to}
          onClick={onClick}
          onMouseEnter={() => onPreload(item.to)}
          onFocus={() => onPreload(item.to)}
          className={cn(
            "relative flex h-10 w-10 items-center justify-center rounded-[var(--radius-token-md)]",
            "transition-colors duration-150 outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30",
            isActive
              ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
              : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]"
          )}
          aria-label={item.label}
          aria-current={isActive ? "page" : undefined}
        >
          <Icon className="h-5 w-5" aria-hidden="true" />
          {item.badge !== undefined && item.badge > 0 && (
            <span
              className={cn(
                "absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-medium",
                item.badgeVariant === 'warning'
                  ? undefined
                  : "bg-[var(--brand-primary)] text-white"
              )}
              style={item.badgeVariant === 'warning' ? WARNING_BADGE_STYLE : undefined}
              aria-label={`${item.badge} items`}
            >
              {item.badge}
            </span>
          )}
        </Link>
      </TooltipTrigger>
      <TooltipContent side="right">{item.label}</TooltipContent>
    </Tooltip>
  );
}


interface NavItem {
  id: NavTab;
  label: string;
  icon: React.ComponentType<LucideProps>;
  badge?: number;
  /** Visual treatment for the badge. 'warning' fills with the amber status token. */
  badgeVariant?: 'neutral' | 'warning';
  to: string;
}

// Inline style for the warning badge — avoids Tailwind v4 CSS-var alpha
// shorthand quirks. Used wherever item.badgeVariant === 'warning' && item.badge > 0.
const WARNING_BADGE_STYLE = {
  backgroundColor: 'hsl(var(--status-warning-hsl) / 0.18)',
  color: 'var(--status-warning)',
} as const;

/**
 * NavItemLink — extracted as a standalone component to avoid recreating on every
 * SidebarNav render (addresses inline renderNavItem finding).
 */
const NavItemLink = React.memo(function NavItemLink({
  item,
  isActive,
  onClick,
  onPreload,
}: {
  item: NavItem;
  isActive: boolean;
  onClick: () => void;
  onPreload: (to: string) => void;
}) {
  const Icon = item.icon;
  return (
    <Link
      to={item.to}
      onClick={onClick}
      onMouseEnter={() => onPreload(item.to)}
      onFocus={() => onPreload(item.to)}
      className={cn(
        "w-full flex items-center gap-3 px-3 rounded-[var(--radius-token-md)] text-[14px] font-medium transition-all duration-150",
        "py-[9px]",
        isActive
          ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
          : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]",
      )}
      aria-label={item.label}
      aria-current={isActive ? "page" : undefined}
    >
      <Icon className="h-5 w-5 flex-shrink-0" aria-hidden="true" />
      <span className="flex-1 text-left whitespace-nowrap overflow-hidden text-ellipsis">
        {item.label}
      </span>
      {item.badge !== undefined && item.badge > 0 && (
        <span
          className={cn(
            "flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-medium",
            item.badgeVariant === 'warning'
              ? undefined
              : isActive
                ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-secondary)]"
                : "bg-[rgba(255,255,255,0.06)] text-[var(--text-muted)]"
          )}
          style={item.badgeVariant === 'warning' ? WARNING_BADGE_STYLE : undefined}
          aria-label={`${item.badge} items`}
        >
          {item.badge}
        </span>
      )}
    </Link>
  );
});

/**
 * UserMenu — avatar trigger at the bottom of the sidebar opening a dropdown
 * with Profile, Documentation, Settings, and Logout.
 */
const UserMenu = React.memo(function UserMenu({
  onNavItemClick,
  isCollapsed,
}: {
  onNavItemClick?: () => void;
  isCollapsed?: boolean;
}) {
  const navigate = useNavigate();
  const { user, logout } = useAuth();

  const handleProfile = useCallback(() => {
    navigate("/user-info");
    onNavItemClick?.();
  }, [navigate, onNavItemClick]);

  const handleSettings = useCallback(() => {
    navigate("/settings");
    onNavItemClick?.();
  }, [navigate, onNavItemClick]);

  const handleLogout = useCallback(() => {
    logout();
    onNavItemClick?.();
  }, [logout, onNavItemClick]);

  if (!user) {
    if (isCollapsed) {
      return (
        <div
          className="h-10 w-10 mx-auto rounded-full bg-[rgba(255,255,255,0.06)] animate-pulse"
          aria-busy="true"
        />
      );
    }
    return (
      <div
        className="w-full flex items-center gap-3 px-3 py-[9px] rounded-[var(--radius-token-md)] text-[14px] font-medium text-[var(--text-muted)]"
        aria-busy="true"
      >
        <div className="h-6 w-6 flex-shrink-0 rounded-full bg-[rgba(255,255,255,0.06)] animate-pulse" />
        <div className="flex-1 h-3 rounded bg-[rgba(255,255,255,0.06)] animate-pulse" />
      </div>
    );
  }

  const displayName = user.email?.split("@")[0] ?? "User";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(
          "rounded-[var(--radius-token-md)] text-[var(--text-secondary)]",
          "hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]",
          "data-[state=open]:bg-[rgba(255,255,255,0.06)] data-[state=open]:text-[var(--text-primary)]",
          "transition-all duration-150 outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30",
          isCollapsed
            ? "flex h-10 w-10 mx-auto items-center justify-center rounded-full bg-[rgba(255,255,255,0.08)]"
            : "w-full flex items-center gap-3 px-3 py-[9px] text-[14px] font-medium"
        )}
        aria-label={isCollapsed ? `Account: ${displayName}` : "Open user menu"}
        data-testid="user-menu-trigger"
      >
        {isCollapsed ? (
          <User className="h-4 w-4" aria-hidden="true" />
        ) : (
          <>
            <div className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-[rgba(255,255,255,0.08)]">
              <User className="h-3.5 w-3.5" aria-hidden="true" />
            </div>
            <span className="flex-1 text-left whitespace-nowrap overflow-hidden text-ellipsis">
              {displayName}
            </span>
            <ChevronDown className="h-4 w-4 flex-shrink-0 opacity-70" aria-hidden="true" />
          </>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side={isCollapsed ? "right" : "top"}
        align={isCollapsed ? "end" : "start"}
        sideOffset={8}
        // z-[70] so the menu sits ABOVE the sidebar (z-[60]). Radix portals
        // the content to <body> with the primitive's default z-50, which
        // landed UNDER the lifted sidebar and made the menu invisible even
        // though it opened.
        className="z-[70] w-[244px]"
      >
        <DropdownMenuItem
          onSelect={handleProfile}
          data-testid="user-menu-profile"
          aria-label="My Access"
        >
          <User className="h-4 w-4" aria-hidden="true" />
          <div className="flex flex-col min-w-0">
            <span className="truncate text-sm font-medium">{displayName}</span>
            {user.email && (
              <span className="truncate text-xs text-muted-foreground">{user.email}</span>
            )}
            <span className="truncate text-xs text-muted-foreground">My Access</span>
          </div>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={handleSettings} data-testid="user-menu-settings">
          <Settings className="h-4 w-4" aria-hidden="true" />
          Settings
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild data-testid="user-menu-documentation">
          <a
            href="https://knodex.io/docs"
            target="_blank"
            rel="noopener noreferrer"
            onClick={onNavItemClick}
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            Documentation
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={handleLogout} data-testid="user-menu-logout">
          <LogOut className="h-4 w-4" aria-hidden="true" />
          Logout
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
});

/**
 * SidebarNav renders the sidebar navigation content (logo, nav sections, footer).
 * Used by both the desktop Sidebar and the tablet/mobile SidebarDrawer.
 */
export function SidebarNav({ onNavItemClick, onToggleCollapse, isCollapsed }: SidebarNavProps) {
  const location = useLocation();

  // Get categories (OSS feature — Casbin-filtered per user)
  const { categories } = useCategoriesEnabled();

  // Derive active tab from current route
  const categorySlugMatch = location.pathname.match(/^\/catalog\/categories\/([^/]+)/);
  const activeTab: NavTab =
    location.pathname.startsWith('/projects') ? 'projects' :
    location.pathname.startsWith('/repositories') ? 'repositories' :
    location.pathname.startsWith('/audit') ? 'audit' :
    location.pathname.startsWith('/compliance') ? 'compliance' :
    location.pathname.startsWith('/secrets') ? 'secrets' :
    location.pathname.startsWith('/catalog') ? 'catalog' :
    location.pathname.startsWith('/deploy/') ? 'catalog' :
    location.pathname.startsWith('/instances') ? 'instances' :
    location.pathname.startsWith('/agents') ? 'agents' :
    'instances';

  const handleNavItemClick = useCallback(() => {
    onNavItemClick?.();
  }, [onNavItemClick]);

  const { data: violationCount } = useViolationCount();
  // Compliance sub-nav count chips (Templates/Constraints). Deduped by React
  // Query with useViolationCount() above — same queryKey ["compliance","summary"]
  // — so this adds no extra network request.
  const { data: complianceSummary } = useComplianceSummary();

  // Cached RGD list — used to map an RGD detail route back to its category for
  // sidebar highlighting. Called with no params so the query stays disabled
  // (`enabled: params !== undefined`); the cache key `["rgds", undefined]` is
  // populated by route-preloads.ts. On cache miss (e.g., hard refresh to a
  // detail page) the highlight falls through to "All Resources".
  const { data: rgdListData } = useRGDList();

  // Secrets nav visibility: only shown when user has any secrets permission
  const { allowed: canViewSecrets } = useCanI("secrets", "get", "-");

  // Trigger route chunk preload on hover/focus
  const handlePreload = useCallback((to: string) => {
    const preload = routePreloads[to];
    if (preload) preload().catch(() => {});
  }, []);

  // --- Section definitions ---

  const infrastructureItems: NavItem[] = useMemo(() => [
    // Story 49.1: Agents hub — same section as Catalog (equal visual weight, UX-DR1).
    { ...NAV_ITEMS.agents, to: NAV_ITEMS.agents.path },
    { ...NAV_ITEMS.catalog, to: NAV_ITEMS.catalog.path },
    { ...NAV_ITEMS.instances, to: NAV_ITEMS.instances.path },
  ], []);

  const manageItems: NavItem[] = useMemo(() => {
    const items: NavItem[] = [];
    if (canViewSecrets === true) {
      items.push({ ...NAV_ITEMS.secrets, to: NAV_ITEMS.secrets.path });
    }
    items.push({ ...NAV_ITEMS.projects, to: NAV_ITEMS.projects.path });
    items.push({ ...NAV_ITEMS.repositories, to: NAV_ITEMS.repositories.path });
    return items;
  }, [canViewSecrets]);

  const enterpriseItems: NavItem[] = useMemo(() => {
    if (!isEnterprise()) return [];
    return [
      { ...NAV_ITEMS.compliance, to: NAV_ITEMS.compliance.path, badge: violationCount, badgeVariant: 'warning' as const },
      { ...NAV_ITEMS.audit, to: NAV_ITEMS.audit.path },
    ];
  }, [violationCount]);

  // Arrow key navigation within sections
  const handleSectionKeyDown = useCallback((e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;

    const section = e.currentTarget;
    const focusableItems = Array.from(
      section.querySelectorAll<HTMLElement>('a[href], button')
    );
    const currentIndex = focusableItems.indexOf(e.target as HTMLElement);
    if (currentIndex === -1) return;

    e.preventDefault();
    let nextIndex: number;
    if (e.key === "ArrowDown") {
      nextIndex = currentIndex < focusableItems.length - 1 ? currentIndex + 1 : 0;
    } else {
      nextIndex = currentIndex > 0 ? currentIndex - 1 : focusableItems.length - 1;
    }
    focusableItems[nextIndex]?.focus();
  }, []);

  // Render a single nav item — uses NavItemLink extracted below
  const renderNavItem = useCallback((item: NavItem) => (
    <NavItemLink
      key={item.id}
      item={item}
      isActive={activeTab === item.id}
      onClick={handleNavItemClick}
      onPreload={handlePreload}
    />
  ), [activeTab, handleNavItemClick, handlePreload]);

  // Render a section with a delimiter line (label is sr-only for accessibility)
  const renderSection = (labelId: string, label: string, items: NavItem[], showDivider = false) => (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- keyboard nav within section group
    <div
      role="group"
      aria-label={label}
      onKeyDown={handleSectionKeyDown}
    >
      {showDivider && (
        <div className="mx-3 my-2 border-t border-[rgba(255,255,255,0.06)]" />
      )}
      <div className="space-y-0.5">
        {items.map((item) => renderNavItem(item))}
      </div>
    </div>
  );

  // /deploy/:rgdName opens as a drawer on top of the catalog; keep the catalog
  // sub-sidebar visible so the user doesn't lose category context. /deploy-rgd
  // is a standalone page (generated spec hand-off) and must NOT trigger the
  // catalog sub-sidebar.
  const isOnCatalogRoute =
    location.pathname.startsWith('/catalog') ||
    location.pathname.startsWith('/deploy/');
  const isOnComplianceRoute = location.pathname.startsWith('/compliance');

  // Catalog sub-nav: "All Resources" + each Casbin-filtered category
  const catalogSubNav: NavItem[] = useMemo(() => {
    const items: NavItem[] = [
      { id: "catalog-all", label: "All Resources", icon: NAV_ITEMS.catalog.icon, to: "/catalog" },
    ];
    if (categories && categories.length > 0) {
      categories.forEach((category) => {
        items.push({
          id: `catalog-category-${category.slug}`,
          label: category.name,
          icon: getLucideIcon(category.icon),
          badge: category.count,
          to: `/catalog/categories/${category.slug}`,
        });
      });
    }
    return items;
  }, [categories]);

  // When on an RGD detail page (/catalog/:rgdName), find the RGD's category
  // from the cached list so we can keep the correct sidebar category highlighted.
  const rgdDetailCategory = useMemo(() => {
    if (categorySlugMatch) return null;
    const rgdDetailMatch = location.pathname.match(/^\/catalog\/([^/]+)$/);
    if (!rgdDetailMatch) return null;
    const rgdName = decodeURIComponent(rgdDetailMatch[1]);
    const rgd = rgdListData?.items?.find((r) => r.name === rgdName);
    if (!rgd?.category) return null;
    const slug = rgd.category.toLowerCase();
    const matched = categories?.find((c) => c.slug === slug);
    return matched ? `catalog-category-${matched.slug}` : null;
  }, [location.pathname, categorySlugMatch, rgdListData?.items, categories]);

  const catalogActiveTab = useMemo(() => {
    if (categorySlugMatch) return `catalog-category-${categorySlugMatch[1]}`;
    if (rgdDetailCategory) return rgdDetailCategory;
    return "catalog-all";
  }, [categorySlugMatch, rgdDetailCategory]);

  const complianceSubNav: NavItem[] = useMemo(() => [
    { id: "compliance-overview", label: "Overview", icon: NAV_ITEMS.compliance.icon, to: "/compliance" },
    { id: "compliance-templates", label: "Templates", icon: FileText, badge: complianceSummary?.totalTemplates, badgeVariant: 'neutral' as const, to: "/compliance/templates" },
    { id: "compliance-constraints", label: "Constraints", icon: Shield, badge: complianceSummary?.totalConstraints, badgeVariant: 'neutral' as const, to: "/compliance/constraints" },
    { id: "compliance-violations", label: "Violations", icon: AlertTriangle, badge: violationCount, badgeVariant: 'warning' as const, to: "/compliance/violations" },
  ], [violationCount, complianceSummary]);

  const complianceActiveTab = useMemo(() => {
    if (location.pathname.startsWith('/compliance/templates')) return "compliance-templates";
    if (location.pathname.startsWith('/compliance/constraints')) return "compliance-constraints";
    if (location.pathname.startsWith('/compliance/violations')) return "compliance-violations";
    return "compliance-overview";
  }, [location.pathname]);

  // Agents sub-sidebar — mirrors the Catalog secondary sidebar. Items come from
  // the shared agents nav model (Overview/Agents/Models); intentionally no count
  // badges. Active highlighting reuses resolveActiveAgentsId so a deep chat route
  // (/agents/list/:ns/:name/...) keeps "Agents" lit.
  const isOnAgentsRoute = location.pathname.startsWith('/agents');
  const agentsSubNav: NavItem[] = useMemo(
    () =>
      getAgentsNavItems().map((it) => ({
        id: `agents-${it.id}`,
        label: it.label,
        icon: it.icon,
        to: it.to,
      })),
    [],
  );
  const agentsActiveTab = useMemo(() => {
    const id = resolveActiveAgentsId(location.pathname, getAgentsNavItems());
    return id ? `agents-${id}` : "agents-overview";
  }, [location.pathname]);

  // Collapsed icon rail — overrides catalog/compliance sub-navs when collapsed.
  if (isCollapsed) {
    const railItems: NavItem[] = [
      ...infrastructureItems,
      ...manageItems,
      ...enterpriseItems,
    ];
    return (
      <TooltipProvider delayDuration={150}>
        <div className="flex h-full flex-col">
          <LogoHeader onToggleCollapse={onToggleCollapse} isCollapsed={isCollapsed} />

          <nav
            className="flex-1 overflow-y-auto py-4 flex flex-col items-center gap-1.5"
            aria-label="Main navigation (collapsed)"
          >
            {railItems.map((item) => (
              <CollapsedNavIcon
                key={item.id}
                item={item}
                isActive={activeTab === item.id}
                onClick={handleNavItemClick}
                onPreload={handlePreload}
              />
            ))}
          </nav>

          <div className="pb-4 pt-2 border-t border-[rgba(255,255,255,0.06)]">
            <UserMenu onNavItemClick={handleNavItemClick} isCollapsed={isCollapsed} />
          </div>
        </div>
      </TooltipProvider>
    );
  }

  const renderUserMenuFooter = () => (
    <div className="px-2 pb-4 pt-2 border-t border-[rgba(255,255,255,0.06)]">
      <UserMenu onNavItemClick={handleNavItemClick} />
    </div>
  );

  // Shared secondary-sidebar shell (Catalog / Compliance / Agents). "Back"
  // returns to /instances; items render with optional count badges (Agents
  // passes none).
  const renderSubSidebar = (ariaLabel: string, items: NavItem[], activeId: string) => (
    <div className="flex h-full flex-col">
      <LogoHeader onToggleCollapse={onToggleCollapse} isCollapsed={isCollapsed} />

      <nav className="flex-1 overflow-y-auto px-2 py-2" aria-label={ariaLabel}>
        <Link
          to="/instances"
          onClick={handleNavItemClick}
          className="flex items-center gap-2 px-3 py-2 mb-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
        >
          <ChevronLeft className="h-3.5 w-3.5" />
          Back
        </Link>
        <div className="space-y-0.5">
          {items.map((item) => {
            const Icon = item.icon;
            const isActive = activeId === item.id;
            return (
              <Link
                key={item.id}
                to={item.to}
                onClick={handleNavItemClick}
                onMouseEnter={() => handlePreload(item.to)}
                onFocus={() => handlePreload(item.to)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 rounded-[var(--radius-token-md)] text-[14px] font-medium transition-all duration-150",
                  "py-[9px]",
                  isActive
                    ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
                    : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]"
                )}
                aria-current={isActive ? "page" : undefined}
              >
                <Icon className="h-5 w-5 flex-shrink-0" aria-hidden="true" />
                <span className="flex-1 text-left whitespace-nowrap overflow-hidden text-ellipsis">
                  {item.label}
                </span>
                {item.badge !== undefined && item.badge > 0 && (
                  <span
                    className={cn(
                      "flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-medium",
                      item.badgeVariant === 'warning'
                        ? undefined
                        : isActive
                          ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-secondary)]"
                          : "bg-[rgba(255,255,255,0.06)] text-[var(--text-muted)]"
                    )}
                    style={item.badgeVariant === 'warning' ? WARNING_BADGE_STYLE : undefined}
                  >
                    {item.badge}
                  </span>
                )}
              </Link>
            );
          })}
        </div>
      </nav>

      {renderUserMenuFooter()}
    </div>
  );

  // Catalog sub-sidebar — shown when navigating within /catalog/*
  if (isOnCatalogRoute && catalogSubNav.length > 1) {
    return renderSubSidebar("Catalog navigation", catalogSubNav, catalogActiveTab);
  }

  // Agents sub-sidebar — shown when navigating within /agents/*
  if (isOnAgentsRoute) {
    return renderSubSidebar("Agents navigation", agentsSubNav, agentsActiveTab);
  }

  if (isOnComplianceRoute && isEnterprise()) {
    return renderSubSidebar("Compliance navigation", complianceSubNav, complianceActiveTab);
  }

  return (
    <div className="flex h-full flex-col">
      <LogoHeader onToggleCollapse={onToggleCollapse} isCollapsed={isCollapsed} />

      {/* Primary Navigation */}
      <nav className="flex-1 overflow-y-auto px-2 py-4" aria-label="Main navigation">
        {/* Infrastructure Section */}
        {renderSection("nav-section-infrastructure", "Infrastructure", infrastructureItems)}

        {/* Manage Section */}
        {renderSection("nav-section-manage", "Manage", manageItems, true)}

        {/* Enterprise Section — conditionally rendered */}
        {isEnterprise() && renderSection("nav-section-enterprise", "Enterprise", enterpriseItems, true)}

        {/* Cloud-tenant feature pages (Plan/Billing/Marketplace/Team) are not in
            the sidebar — they live under Settings (see getSettingsNavItems). */}
      </nav>

      {renderUserMenuFooter()}
    </div>
  );
}

interface SidebarProps {
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
}

/**
 * Desktop sidebar — fixed at lg+ (1024px and above). When collapsed, renders
 * a 64px icon rail (still visible, with the PanelLeft toggle to re-expand).
 */
export function Sidebar({ isCollapsed, onToggleCollapse }: SidebarProps = {}) {
  return (
    <aside
      className={cn(
        // z-[60] so the sidebar sits ABOVE Radix Dialog overlays (z-50) like
        // the Deploy drawer's backdrop — without this the overlay portals
        // after the sidebar in the DOM and dims it.
        "hidden lg:block fixed left-0 top-0 z-[60] h-screen bg-background border-r border-[var(--border-default)]",
        "transition-[width] duration-200",
        isCollapsed ? "w-16" : "w-[260px]"
      )}
      data-testid="desktop-sidebar"
      data-collapsed={isCollapsed ? "true" : "false"}
    >
      <SidebarNav onToggleCollapse={onToggleCollapse} isCollapsed={isCollapsed} />
    </aside>
  );
}
