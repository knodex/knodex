// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, type ReactNode } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { cn } from "@/lib/utils";

export interface BreadcrumbItem {
  label: string;
  to?: string;
}

interface BreadcrumbsProps {
  className?: string;
  /** Rendered as the first crumb (e.g., a project chip). When set, the auto-Home crumb is omitted. */
  leadingSlot?: ReactNode;
  /** Suppresses the auto-Home crumb (e.g., when an org name already anchors the root). */
  hideHome?: boolean;
}

/**
 * decodeURIComponent throws URIError on malformed percent-encoding
 * (e.g., "%E0%A4%A"). The breadcrumb must never crash the chrome,
 * so fall back to the raw segment on failure.
 */
function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

function buildInstanceLabel(params: ReturnType<typeof useParams>): string {
  // Cluster-scoped routes have no namespace; filter empty parts so the label
  // doesn't end up with a stray leading slash like "/Pod/pod1".
  const parts = [params.namespace, params.kind, params.name].filter(
    (p): p is string => Boolean(p),
  );
  return parts.map(safeDecode).join("/");
}

function deriveItems(path: string, params: ReturnType<typeof useParams>): BreadcrumbItem[] {
  const items: BreadcrumbItem[] = [];

  if (path.startsWith("/deploy") && params.rgdName) {
    items.push({ label: "Catalog", to: "/catalog" });
    items.push({
      label: safeDecode(params.rgdName),
      to: `/catalog/${params.rgdName}`,
    });
    items.push({ label: "Deploy" });
  } else if (path.startsWith("/catalog")) {
    items.push({ label: "Catalog", to: "/catalog" });

    const categoryMatch = path.match(/^\/catalog\/categories\/([^/]+)/);
    if (categoryMatch) {
      items.push({ label: safeDecode(categoryMatch[1]) });
    } else if (params.rgdName) {
      items.push({
        label: safeDecode(params.rgdName),
        to: `/catalog/${params.rgdName}`,
      });
      if (path.includes("/deploy")) {
        items.push({ label: "Deploy" });
      }
    }
  } else if (path.startsWith("/instances")) {
    items.push({ label: "Instances", to: "/instances" });

    if (params.name) {
      items.push({ label: buildInstanceLabel(params) });
    }
  } else if (path.startsWith("/audit")) {
    items.push({ label: "Audit" });
  } else if (path.startsWith("/compliance")) {
    items.push({ label: "Compliance", to: "/compliance" });

    if (path.includes("/templates/")) {
      items.push({ label: "Templates", to: "/compliance/templates" });
      const templateName = path.split("/templates/")[1];
      if (templateName) {
        items.push({ label: safeDecode(templateName) });
      }
    } else if (path.includes("/constraints/")) {
      items.push({ label: "Constraints", to: "/compliance/constraints" });
      const constraintName = path.split("/constraints/")[1];
      if (constraintName) {
        items.push({ label: safeDecode(constraintName) });
      }
    }
  } else if (path.startsWith("/secrets")) {
    items.push({ label: "Secrets" });
  } else if (path.startsWith("/settings")) {
    items.push({ label: "Settings", to: "/settings" });

    if (path.includes("/repositories")) {
      items.push({ label: "Repositories" });
    } else if (path.includes("/projects")) {
      items.push({ label: "Projects" });
    } else if (path.includes("/audit")) {
      items.push({ label: "Audit" });
    } else if (path.includes("/sso")) {
      items.push({ label: "SSO" });
    } else if (path.includes("/license")) {
      items.push({ label: "License" });
    }
  } else if (path.startsWith("/repositories")) {
    // Top-level Repositories route (App.tsx:262), distinct from /settings/repositories.
    items.push({ label: "Repositories" });
  } else if (path.startsWith("/projects")) {
    // Top-level Projects route (App.tsx:272). Handles both /projects and /projects/:name.
    items.push({ label: "Projects", to: "/projects" });
    if (params.name) {
      items.push({ label: safeDecode(params.name) });
    }
  } else if (path.startsWith("/user-info")) {
    items.push({ label: "Account" });
  } else if (path.startsWith("/views")) {
    items.push({ label: "Views", to: "/views" });

    const viewSlug = path.match(/^\/views\/([^/]+)/);
    if (viewSlug) {
      items.push({ label: safeDecode(viewSlug[1]) });
    }
  }

  return items;
}

export function Breadcrumbs({ className, leadingSlot, hideHome }: BreadcrumbsProps) {
  const location = useLocation();
  const params = useParams();

  const items = useMemo(() => {
    const derived = deriveItems(location.pathname, params);
    // When no leading slot is provided, prepend Home so users have a way back to root.
    if (!leadingSlot && !hideHome) {
      return [{ label: "Home", to: "/" } satisfies BreadcrumbItem, ...derived];
    }
    return derived;
  }, [location.pathname, params, leadingSlot, hideHome]);

  return (
    <nav
      aria-label="Breadcrumb"
      className={cn(
        "flex items-center min-w-0 text-[var(--text-size-sm)]",
        className,
      )}
      data-testid="breadcrumbs"
    >
      <ol className="flex items-center gap-1.5 min-w-0 list-none m-0 p-0">
        {leadingSlot && (
          <li className="flex items-center min-w-0">
            {leadingSlot}
          </li>
        )}
        {items.map((item, index) => {
          const isLast = index === items.length - 1;
          const isClickable = !isLast && item.to;
          const showSeparatorBefore = index > 0 || Boolean(leadingSlot);

          return (
            <li
              key={item.to ?? `${item.label}-${index}`}
              className="flex items-center gap-1.5 min-w-0"
            >
              {showSeparatorBefore && (
                <span
                  aria-hidden="true"
                  className="text-muted-foreground/60 select-none"
                >
                  /
                </span>
              )}
              {isClickable ? (
                <Link
                  to={item.to!}
                  className="min-w-0 truncate text-muted-foreground hover:text-foreground transition-colors"
                >
                  {item.label}
                </Link>
              ) : (
                <span
                  className={cn(
                    "min-w-0 truncate",
                    isLast ? "text-foreground font-medium" : "text-muted-foreground",
                  )}
                  aria-current={isLast ? "page" : undefined}
                >
                  {item.label}
                </span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
