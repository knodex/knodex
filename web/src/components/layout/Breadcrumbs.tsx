// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { ChevronRight, Home } from "lucide-react";
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
    const breadcrumbs: BreadcrumbItem[] = [];

    if (path.startsWith('/catalog')) {
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

      if (params.namespace && params.name) {
        const instanceLabel = `${decodeURIComponent(params.namespace)}/${decodeURIComponent(params.name)}`;
        breadcrumbs.push({
          label: instanceLabel
        });
      }
    }

    return breadcrumbs;
  }, [location, params]);

  // Don't show breadcrumbs if only one item (top-level page)
  if (items.length <= 1) {
    return null;
  }

  return (
    <div className="bg-card/30">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 py-3">
        <nav
          aria-label="Breadcrumb"
          className={cn("flex items-center text-sm", className)}
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
