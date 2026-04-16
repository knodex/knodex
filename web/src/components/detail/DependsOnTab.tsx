// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { Link2, ExternalLink, Loader2 } from "@/lib/icons";
import { useKindToRGDMap } from "@/hooks/useKindToRGDMap";
import type { CatalogRGD } from "@/types/rgd";
import { Button } from "@/components/ui/button";
import { RGDMiniCard } from "@/components/shared/RGDMiniCard";

interface DependsOnTabProps {
  rgd: CatalogRGD;
}

export function DependsOnTab({ rgd }: DependsOnTabProps) {
  const dependsOnKinds = rgd.dependsOnKinds || [];

  // Shared hook for Kind-to-RGD resolution (deduplicates with DependsOnKindLink in Overview tab)
  const { kindToRGD, isLoading } = useKindToRGDMap();

  // Only show dependencies that exist in the catalog
  const catalogKinds = isLoading
    ? dependsOnKinds
    : dependsOnKinds.filter((kind) => kindToRGD.has(kind));

  if (catalogKinds.length === 0 && !isLoading) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Link2 className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">Dependencies</h3>
        <span className="text-xs text-muted-foreground">
          RGDs that must be deployed before this one
        </span>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {catalogKinds.map((kind) => (
          <DependsOnCard key={kind} parentRGD={kindToRGD.get(kind)} isLoading={isLoading} />
        ))}
      </div>
    </div>
  );
}

interface DependsOnCardProps {
  parentRGD?: CatalogRGD;
  isLoading: boolean;
}

function DependsOnCard({ parentRGD, isLoading }: DependsOnCardProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 flex items-center gap-2">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        <span className="text-sm text-muted-foreground">Loading...</span>
      </div>
    );
  }

  if (!parentRGD) {
    return null;
  }

  return (
    <RGDMiniCard
      rgd={parentRGD}
      action={
        <Button asChild size="sm" variant="outline" className="w-full gap-1.5">
          <Link to={`/catalog/${encodeURIComponent(parentRGD.name)}`}>
            <ExternalLink className="h-3.5 w-3.5" />
            Deploy
          </Link>
        </Button>
      }
    />
  );
}
