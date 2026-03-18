// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { Link2, ArrowRight, Loader2 } from "lucide-react";
import { useKindToRGDMap } from "@/hooks/useKindToRGDMap";
import type { CatalogRGD } from "@/types/rgd";
import { RGDMiniCard } from "@/components/shared/RGDMiniCard";

interface DependsOnTabProps {
  rgd: CatalogRGD;
}

export function DependsOnTab({ rgd }: DependsOnTabProps) {
  const dependsOnKinds = rgd.dependsOnKinds || [];

  // Shared hook for Kind-to-RGD resolution (deduplicates with DependsOnKindLink in Overview tab)
  const { kindToRGD, isLoading } = useKindToRGDMap();

  if (dependsOnKinds.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-8 text-center">
        <Link2 className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
        <p className="text-sm text-muted-foreground">No dependencies</p>
        <p className="text-xs text-muted-foreground mt-1">
          This RGD does not depend on any external resources.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Link2 className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">Dependencies</h3>
        <span className="text-xs text-muted-foreground">
          RGDs that must be deployed before this one
        </span>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {dependsOnKinds.map((kind) => (
          <DependsOnCard key={kind} kind={kind} parentRGD={kindToRGD.get(kind)} isLoading={isLoading} />
        ))}
      </div>
    </div>
  );
}

interface DependsOnCardProps {
  kind: string;
  parentRGD?: CatalogRGD;
  isLoading: boolean;
}

function DependsOnCard({ kind, parentRGD, isLoading }: DependsOnCardProps) {
  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 flex items-center gap-2">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        <span className="text-sm text-muted-foreground">Loading...</span>
      </div>
    );
  }

  if (!parentRGD) {
    return (
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex items-center gap-2 mb-2">
          <Link2 className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">{kind}</span>
        </div>
        <p className="text-xs text-muted-foreground">
          Not found in catalog
        </p>
      </div>
    );
  }

  return (
    <RGDMiniCard
      rgd={parentRGD}
      action={
        <Link
          to={`/catalog/${encodeURIComponent(parentRGD.name)}`}
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-primary transition-colors"
        >
          View details
          <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      }
    />
  );
}
