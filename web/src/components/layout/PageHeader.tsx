// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { type ReactNode } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "@/lib/icons";
import { cn } from "@/lib/utils";

interface BreadcrumbItem {
  /** Label to display */
  label: string;
  /** Link destination (optional for last item) */
  href?: string;
}

interface PageHeaderProps {
  /** Page title (required) */
  title: string;
  /** Optional subtitle/description below the title */
  description?: string;
  /** Alias for description (compliance pages use this name) */
  subtitle?: string;
  /** Optional count badge displayed next to the title */
  count?: number;
  /** Breadcrumb navigation items */
  breadcrumbs?: BreadcrumbItem[];
  /** Optional right-aligned action elements (named slot) */
  actions?: ReactNode;
  /** Optional right-aligned action elements (children slot, same position as actions) */
  children?: ReactNode;
  /** Optional className for customization */
  className?: string;
}

/**
 * Shared page header component for consistent layout across all pages.
 * Supports breadcrumbs, count badges, and right-aligned actions.
 */
export function PageHeader({
  title,
  description,
  subtitle,
  count: _count,
  breadcrumbs,
  actions,
  children,
  className,
}: PageHeaderProps) {
  const desc = description || subtitle;
  const rightContent = actions || children;

  return (
    <div className={cn("space-y-2", className)}>
      {/* Breadcrumbs */}
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav
          aria-label="Breadcrumb"
          className="flex items-center text-sm text-muted-foreground"
        >
          {breadcrumbs.map((item, index) => (
            <span key={item.label} className="flex items-center">
              {index > 0 && (
                <ChevronRight className="h-4 w-4 mx-1 flex-shrink-0" />
              )}
              {item.href ? (
                <Link
                  to={item.href}
                  className="hover:text-foreground transition-colors"
                >
                  {item.label}
                </Link>
              ) : (
                <span className="text-foreground font-medium">
                  {item.label}
                </span>
              )}
            </span>
          ))}
        </nav>
      )}

      {/* SR-only h1 for accessibility focus — visible title is in TopBar */}
      <h1 tabIndex={-1} className="sr-only outline-none">{title}</h1>

      {/* Actions row */}
      {rightContent && (
        <div className="flex items-center justify-end gap-2">
          {rightContent}
        </div>
      )}
      {desc && (
        <p className="text-[var(--text-size-base)] text-[var(--text-secondary)]">{desc}</p>
      )}
    </div>
  );
}

export default PageHeader;
