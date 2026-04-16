// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useCallback, useMemo } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  LayoutGrid,
  Box,
  ExternalLink,
  Settings,
  ShieldCheck,
  ScrollText,
  ChevronLeft,
  KeyRound,
  FolderOpen,
  GitBranch,
  FileText,
  Shield,
  AlertTriangle,
} from "@/lib/icons";
import type { LucideProps } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { getLucideIcon } from "@/lib/icons";
import { routePreloads } from "@/lib/route-preloads";
import { useRGDCount } from "@/hooks/useRGDs";
import { useRGDList } from "@/hooks/useRGDs";
import { useInstanceCount, useInstanceList } from "@/hooks/useInstances";
import { useViolationCount, isEnterprise } from "@/hooks/useCompliance";
import { useCategoriesEnabled } from "@/hooks/useCategories";
import { useCanI } from "@/hooks/useCanI";
import { useCurrentProject } from "@/hooks/useAuth";
import { useProjects } from "@/hooks/useProjects";
import { filterByProjectNamespaces } from "@/lib/project-utils";

type NavTab = "catalog" | "instances" | "compliance" | "settings" | "projects" | "repositories" | string;

interface SidebarNavProps {
  onNavItemClick?: () => void;
}

interface NavItem {
  id: NavTab;
  label: string;
  icon: React.ComponentType<LucideProps>;
  badge?: number;
  to: string;
}

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
            isActive
              ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-secondary)]"
              : "bg-[rgba(255,255,255,0.06)] text-[var(--text-muted)]"
          )}
          aria-label={`${item.badge} items`}
        >
          {item.badge}
        </span>
      )}
    </Link>
  );
});

/**
 * SidebarNav renders the sidebar navigation content (logo, nav sections, footer).
 * Used by both the desktop Sidebar and the tablet/mobile SidebarDrawer.
 */
export function SidebarNav({ onNavItemClick }: SidebarNavProps) {
  const location = useLocation();

  // Get categories (OSS feature — Casbin-filtered per user)
  const { categories } = useCategoriesEnabled();

  // Derive active tab from current route
  const categorySlugMatch = location.pathname.match(/^\/catalog\/categories\/([^/]+)/);
  const activeTab: NavTab =
    location.pathname.startsWith('/projects') ? 'projects' :
    location.pathname.startsWith('/repositories') ? 'repositories' :
    location.pathname.startsWith('/settings') ? 'settings' :
    location.pathname.startsWith('/audit') ? 'audit' :
    location.pathname.startsWith('/compliance') ? 'compliance' :
    location.pathname.startsWith('/secrets') ? 'secrets' :
    location.pathname.startsWith('/catalog') ? 'catalog' :
    location.pathname.startsWith('/deploy') ? 'catalog' :
    location.pathname.startsWith('/instances') ? 'instances' :
    'instances';

  const handleNavItemClick = useCallback(() => {
    onNavItemClick?.();
  }, [onNavItemClick]);

  // Project-aware counts for sidebar badges
  const currentProject = useCurrentProject();
  const { data: rgdCountData } = useRGDCount();
  const { data: instanceCountData } = useInstanceCount();
  const { data: violationCount } = useViolationCount();

  // When a project is selected, use full lists (already cached by pages) to compute filtered counts
  const { data: rgdListData } = useRGDList();
  const { data: instanceListData } = useInstanceList();
  const { data: projectsData } = useProjects();

  // Secrets nav visibility: only shown when user has any secrets permission
  const { allowed: canViewSecrets } = useCanI("secrets", "get", "-");

  const rgdCount = useMemo(() => {
    // When categories are available, sum their counts for an RBAC-accurate total.
    // The WebSocket-pushed rgdCount is cluster-wide (unfiltered), so we prefer
    // the per-category counts which are already Casbin-filtered by the server.
    if (!currentProject && categories && categories.length > 0) {
      return categories.reduce((sum, cat) => sum + cat.count, 0);
    }
    if (!currentProject) return rgdCountData?.count ?? 0;
    if (!rgdListData?.items) return 0;
    return rgdListData.items.filter(
      (rgd) => rgd.labels?.["knodex.io/project"] === currentProject
    ).length;
  }, [currentProject, rgdCountData, rgdListData, categories]);

  const instanceCount = useMemo(() => {
    if (!currentProject) return instanceCountData?.count ?? 0;
    if (!instanceListData?.items) return 0;
    const selectedProject = projectsData?.items?.find((p) => p.name === currentProject);
    return filterByProjectNamespaces(instanceListData.items, selectedProject).length;
  }, [currentProject, instanceCountData, instanceListData, projectsData]);

  // Trigger route chunk preload on hover/focus
  const handlePreload = useCallback((to: string) => {
    const preload = routePreloads[to];
    if (preload) preload().catch(() => {});
  }, []);

  // --- Section definitions ---

  const infrastructureItems: NavItem[] = useMemo(() => [
    { id: "catalog", label: "Catalog", icon: LayoutGrid, badge: rgdCount, to: "/catalog" },
    { id: "instances", label: "Instances", icon: Box, badge: instanceCount, to: "/instances" },
  ], [rgdCount, instanceCount]);

  const manageItems: NavItem[] = useMemo(() => {
    const items: NavItem[] = [];
    if (canViewSecrets === true) {
      items.push({ id: "secrets", label: "Secrets", icon: KeyRound, to: "/secrets" });
    }
    items.push({ id: "projects", label: "Projects", icon: FolderOpen, to: "/projects" });
    items.push({ id: "repositories", label: "Repositories", icon: GitBranch, to: "/repositories" });
    return items;
  }, [canViewSecrets]);

  const enterpriseItems: NavItem[] = useMemo(() => {
    if (!isEnterprise()) return [];
    return [
      { id: "compliance", label: "Compliance", icon: ShieldCheck, badge: violationCount, to: "/compliance" },
      { id: "audit", label: "Audit", icon: ScrollText, to: "/audit" },
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

  const isOnCatalogRoute = location.pathname.startsWith('/catalog');
  const isOnComplianceRoute = location.pathname.startsWith('/compliance');

  // Catalog sub-nav: "All Resources" + each Casbin-filtered category
  const catalogSubNav: NavItem[] = useMemo(() => {
    const items: NavItem[] = [
      { id: "catalog-all", label: "All Resources", icon: LayoutGrid, badge: rgdCount, to: "/catalog" },
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
  }, [rgdCount, categories]);

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
    { id: "compliance-overview", label: "Overview", icon: ShieldCheck, to: "/compliance" },
    { id: "compliance-templates", label: "Templates", icon: FileText, to: "/compliance/templates" },
    { id: "compliance-constraints", label: "Constraints", icon: Shield, to: "/compliance/constraints" },
    { id: "compliance-violations", label: "Violations", icon: AlertTriangle, badge: violationCount, to: "/compliance/violations" },
  ], [violationCount]);

  const complianceActiveTab = useMemo(() => {
    if (location.pathname.startsWith('/compliance/templates')) return "compliance-templates";
    if (location.pathname.startsWith('/compliance/constraints')) return "compliance-constraints";
    if (location.pathname.startsWith('/compliance/violations')) return "compliance-violations";
    return "compliance-overview";
  }, [location.pathname]);

  // Catalog sub-sidebar — shown when navigating within /catalog/*
  if (isOnCatalogRoute && catalogSubNav.length > 1) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex h-16 items-center px-4">
          <div className="flex items-center gap-3 min-w-0">
            <img src="/logo.svg" alt="Knodex" className="h-10 w-10 shrink-0" />
            <span className="text-sm font-semibold text-[var(--text-primary)] whitespace-nowrap overflow-hidden">
              Knodex
            </span>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 py-2" aria-label="Catalog navigation">
          <Link
            to="/instances"
            onClick={handleNavItemClick}
            className="flex items-center gap-2 px-3 py-2 mb-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            Back
          </Link>
          <div className="space-y-0.5">
            {catalogSubNav.map((item) => {
              const Icon = item.icon;
              const isActive = catalogActiveTab === item.id;
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
                        isActive
                          ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-secondary)]"
                          : "bg-[rgba(255,255,255,0.06)] text-[var(--text-muted)]"
                      )}
                    >
                      {item.badge}
                    </span>
                  )}
                </Link>
              );
            })}
          </div>
        </nav>
      </div>
    );
  }

  if (isOnComplianceRoute && isEnterprise()) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex h-16 items-center px-4">
          <div className="flex items-center gap-3 min-w-0">
            <img src="/logo.svg" alt="Knodex" className="h-10 w-10 shrink-0" />
            <span className="text-sm font-semibold text-[var(--text-primary)] whitespace-nowrap overflow-hidden">
              Knodex
            </span>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 py-2" aria-label="Compliance navigation">
          <Link
            to="/instances"
            onClick={handleNavItemClick}
            className="flex items-center gap-2 px-3 py-2 mb-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            Back
          </Link>
          <div className="space-y-0.5">
            {complianceSubNav.map((item) => {
              const Icon = item.icon;
              const isActive = complianceActiveTab === item.id;
              return (
                <Link
                  key={item.id}
                  to={item.to}
                  onClick={handleNavItemClick}
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
                        isActive
                          ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-secondary)]"
                          : "bg-[rgba(255,255,255,0.06)] text-[var(--text-muted)]"
                      )}
                    >
                      {item.badge}
                    </span>
                  )}
                </Link>
              );
            })}
          </div>
        </nav>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Logo */}
      <div className="flex h-16 items-center px-4">
        <div className="flex items-center gap-3 min-w-0">
          <img src="/logo.svg" alt="Knodex" className="h-10 w-10 shrink-0" />
          <span className="text-sm font-semibold text-[var(--text-primary)] whitespace-nowrap overflow-hidden">
            Knodex
          </span>
        </div>
      </div>

      {/* Primary Navigation */}
      <nav className="flex-1 overflow-y-auto px-2 py-4" aria-label="Main navigation">
        {/* Infrastructure Section */}
        {renderSection("nav-section-infrastructure", "Infrastructure", infrastructureItems)}

        {/* Manage Section */}
        {renderSection("nav-section-manage", "Manage", manageItems, true)}

        {/* Enterprise Section — conditionally rendered */}
        {isEnterprise() && renderSection("nav-section-enterprise", "Enterprise", enterpriseItems, true)}
      </nav>

      {/* Footer — Settings + Documentation (bottom-pinned) */}
      <div className="px-2 py-2">
        <Link
          to="/settings"
          onClick={handleNavItemClick}
          onMouseEnter={() => handlePreload("/settings")}
          onFocus={() => handlePreload("/settings")}
          className={cn(
            "w-full flex items-center gap-3 px-3 py-[9px] rounded-[var(--radius-token-md)] text-[14px] font-medium transition-all duration-150",
            activeTab === "settings"
              ? "bg-[rgba(255,255,255,0.1)] text-[var(--text-primary)]"
              : "text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)]"
          )}
          aria-label="Settings"
          aria-current={activeTab === "settings" ? "page" : undefined}
        >
          <Settings className="h-5 w-5 flex-shrink-0" aria-hidden="true" />
          <span className="flex-1 text-left whitespace-nowrap overflow-hidden">
            Settings
          </span>
        </Link>
      </div>

      <div className="px-2 pb-4">
        <a
          href="https://knodex.io/docs"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Documentation"
          className="flex items-center gap-3 px-3 py-[9px] rounded-[var(--radius-token-md)] text-sm text-[var(--text-secondary)] hover:bg-[rgba(255,255,255,0.04)] hover:text-[var(--text-primary)] transition-colors"
        >
          <ExternalLink className="h-5 w-5 flex-shrink-0" aria-hidden="true" />
          <span className="whitespace-nowrap overflow-hidden">Documentation</span>
        </a>
      </div>
    </div>
  );
}

/**
 * Desktop sidebar — fixed, always visible at lg+ (1024px and above).
 */
export function Sidebar() {
  return (
    <aside className="hidden lg:block fixed left-0 top-0 z-50 h-screen w-[260px] bg-background border-r border-[var(--border-default)]">
      <SidebarNav />
    </aside>
  );
}
