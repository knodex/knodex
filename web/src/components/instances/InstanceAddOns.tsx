// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { AlertCircle, ExternalLink, Loader2, Puzzle } from "@/lib/icons";
import { useRGDList } from "@/hooks/useRGDs";
import { Button } from "@/components/ui/button";
import { RGDMiniCard } from "@/components/shared/RGDMiniCard";

// Maximum number of add-ons to fetch per request.
// Add-ons are a secondary detail; pagination is not expected for typical catalogs.
const ADDONS_PAGE_SIZE = 100;

interface InstanceAddOnsProps {
  kind: string;
  instanceName: string;
  instanceNamespace: string;
}

export function InstanceAddOns({ kind, instanceName: _instanceName, instanceNamespace: _instanceNamespace }: InstanceAddOnsProps) {
  const { data, isLoading, error } = useRGDList({ extendsKind: kind, pageSize: ADDONS_PAGE_SIZE });

  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span className="text-sm">Loading add-ons...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-2 text-muted-foreground">
          <AlertCircle className="h-4 w-4 text-destructive" />
          <span className="text-sm">Failed to load add-ons for {kind}</span>
        </div>
      </div>
    );
  }

  if (!data?.items?.length) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Puzzle className="h-4 w-4" />
          <span className="text-sm">No add-ons available</span>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card p-6">
      <div className="flex items-center gap-2 mb-4">
        <Puzzle className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">Deploy on this instance</h3>
        <span className="text-xs text-muted-foreground">
          Add-ons available for {kind}
        </span>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {data.items.map((addon) => (
          <RGDMiniCard
            key={addon.name}
            rgd={addon}
            action={
              <Button asChild size="sm" variant="outline" className="w-full gap-1.5">
                <Link
                  to={`/catalog/${encodeURIComponent(addon.name)}`}
                >
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
