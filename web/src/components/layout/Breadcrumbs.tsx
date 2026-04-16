// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { ChevronRight, Home } from "@/lib/icons";
import { Link, useLocation, useParams } from "react-router-dom";
import { cn } from "@/lib/utils";

export interface BreadcrumbItem {
  label: string;
  to?: string;
  icon?: React.ReactNode;
}

interface BreadcrumbsProps {
  className?: string;
}

export function Breadcrumbs({ className }: BreadcrumbsProps) {
  const location = useLocation();
  const params = useParams();

  const items = useMemo((): BreadcrumbItem[] => {
    const path = location.pathname;
    const breadcrumbs: BreadcrumbItem[] = [
      { label: 'Home', to: '/' },
    ];

    if (path.startsWith('/deploy') && params.rgdName) {
      breadcrumbs.push({ label: 'Catalog', to: '/catalog' });
      breadcrumbs.push({
        label: decodeURIComponent(params.rgdName),
        to: `/catalog/${params.rgdName}`
      });
      breadcrumbs.push({ label: 'Deploy' });
    } else if (path.startsWith('/catalog')) {
      breadcrumbs.push({ label: 'Catalog', to: '/catalog' });

      if (params.rgdName) {
        breadcrumbs.push({
          label: decodeURIComponent(params.rgdName),
          to: `/catalog/${params.rgdName}`
        });

        if (path.includes('/deploy')) {
          breadcrumbs.push({ label: 'Deploy' });
        }
      }
    } else if (path.startsWith('/instances')) {
      breadcrumbs.push({ label: 'Instances', to: '/instances' });

      if (params.name) {
        const instanceLabel = params.kind
          ? `${decodeURIComponent(params.namespace || '')}/${decodeURIComponent(params.kind)}/${decodeURIComponent(params.name)}`
          : `${decodeURIComponent(params.namespace || '')}/${decodeURIComponent(params.name)}`;
        breadcrumbs.push({
          label: instanceLabel
        });
      }
    } else if (path.startsWith('/audit')) {
      breadcrumbs.push({ label: 'Audit' });
    } else if (path.startsWith('/compliance')) {
      breadcrumbs.push({ label: 'Compliance', to: '/compliance' });

      if (path.includes('/templates/')) {
        breadcrumbs.push({ label: 'Templates', to: '/compliance/templates' });
        const templateName = path.split('/templates/')[1];
        if (templateName) {
          breadcrumbs.push({ label: decodeURIComponent(templateName) });
        }
      } else if (path.includes('/constraints/')) {
        breadcrumbs.push({ label: 'Constraints', to: '/compliance/constraints' });
        const constraintName = path.split('/constraints/')[1];
        if (constraintName) {
          breadcrumbs.push({ label: decodeURIComponent(constraintName) });
        }
      }
    } else if (path.startsWith('/secrets')) {
      breadcrumbs.push({ label: 'Secrets' });
    } else if (path.startsWith('/settings')) {
      breadcrumbs.push({ label: 'Settings', to: '/settings' });

      if (path.includes('/repositories')) {
        breadcrumbs.push({ label: 'Repositories' });
      } else if (path.includes('/projects')) {
        breadcrumbs.push({ label: 'Projects' });
      } else if (path.includes('/audit')) {
        breadcrumbs.push({ label: 'Audit' });
      } else if (path.includes('/sso')) {
        breadcrumbs.push({ label: 'SSO' });
      }
    } else if (path.startsWith('/views')) {
      breadcrumbs.push({ label: 'Views', to: '/views' });

      const viewSlug = path.match(/^\/views\/([^/]+)/);
      if (viewSlug) {
        breadcrumbs.push({ label: decodeURIComponent(viewSlug[1]) });
      }
    }

    return breadcrumbs;
  }, [location, params]);

  // Hide breadcrumbs — title is in TopBar, detail pages use back button
  return null;

  return (
    <div>
      <div className="px-6 lg:px-10 max-w-[1280px] mx-auto py-2">
        <nav
          aria-label="Breadcrumb"
          className={cn("flex items-center text-[var(--text-size-sm)]", className)}
        >
          <ol className="flex items-center gap-1.5">
            {items.map((item, index) => {
              const isLast = index === items.length - 1;
              const isClickable = !isLast && item.to;

              return (
                <li key={index} className="flex items-center gap-1.5">
                  {index > 0 && (
                    <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
                  )}
                  {isClickable ? (
                    <Link
                      to={item.to!}
                      className="flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
                    >
                      {index === 0 && !item.icon && (
                        <Home className="h-3.5 w-3.5" />
                      )}
                      {item.icon}
                      <span>{item.label}</span>
                    </Link>
                  ) : (
                    <span
                      className={cn(
                        "flex items-center gap-1.5",
                        isLast ? "text-foreground font-medium" : "text-muted-foreground"
                      )}
                      aria-current={isLast ? "page" : undefined}
                    >
                      {index === 0 && !item.icon && (
                        <Home className="h-3.5 w-3.5" />
                      )}
                      {item.icon}
                      <span>{item.label}</span>
                    </span>
                  )}
                </li>
              );
            })}
          </ol>
        </nav>
      </div>
    </div>
  );
}
