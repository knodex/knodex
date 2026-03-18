// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import type { CatalogRGD } from "@/types/rgd";

interface RGDMiniCardProps {
  rgd: CatalogRGD;
  action: ReactNode;
}

export function RGDMiniCard({ rgd, action }: RGDMiniCardProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 flex flex-col gap-3">
      <div>
        <Link
          to={`/catalog/${encodeURIComponent(rgd.name)}`}
          className="text-sm font-medium text-foreground hover:text-primary hover:underline transition-colors"
        >
          {rgd.title || rgd.name}
        </Link>
        {rgd.description && (
          <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
            {rgd.description}
          </p>
        )}
      </div>

      {rgd.tags && rgd.tags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {rgd.tags.slice(0, 3).map((tag) => (
            <Badge key={tag} variant="secondary" className="text-xs">
              {tag}
            </Badge>
          ))}
          {rgd.tags.length > 3 && (
            <span className="text-xs text-muted-foreground">
              +{rgd.tags.length - 3}
            </span>
          )}
        </div>
      )}

      <div className="mt-auto">{action}</div>
    </div>
  );
}
