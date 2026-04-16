// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { Loader2, AlertCircle, ExternalLink, Puzzle } from "@/lib/icons";
import { useRGDList } from "@/hooks/useRGDs";
import { Button } from "@/components/ui/button";
import { RGDMiniCard } from "@/components/shared/RGDMiniCard";

// Maximum number of add-ons to fetch per request.
// Add-ons are a secondary detail; pagination is not expected for typical catalogs.
const ADDONS_PAGE_SIZE = 100;

interface AddOnsTabProps {
  kind: string;
}

export function AddOnsTab({ kind }: AddOnsTabProps) {
  const { data, isLoading, error } = useRGDList({ extendsKind: kind, pageSize: ADDONS_PAGE_SIZE });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-[200px]">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin" />
          <span className="text-sm">Loading add-ons...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-[200px] gap-2">
        <AlertCircle className="h-6 w-6 text-destructive" />
        <p className="text-sm text-destructive">Failed to load add-ons</p>
      </div>
    );
  }

  if (!data?.items?.length) {
    return (
      <div className="flex flex-col items-center justify-center h-[200px] gap-2">
        <Puzzle className="h-6 w-6 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">No add-ons available</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Puzzle className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">
          Available Add-ons
        </h3>
        <span className="text-xs text-muted-foreground">
          RGDs that can be deployed on top of {kind} instances
        </span>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {data.items.map((addon) => (
          <RGDMiniCard
            key={addon.name}
            rgd={addon}
            action={
              <Button asChild size="sm" variant="outline" className="w-full gap-1.5">
                <Link to={`/catalog/${encodeURIComponent(addon.name)}`}>
                  <ExternalLink className="h-3.5 w-3.5" />
                  Deploy
                </Link>
              </Button>
            }
          />
        ))}
      </div>
    </div>
  );
}
