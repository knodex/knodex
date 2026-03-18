// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, lazy, Suspense } from "react";
import { Link } from "react-router-dom";
import {
  ArrowLeft,
  Package,
  Clock,
  Layers,
  Box,
  Link2,
  Loader2,
  AlertCircle,
  ExternalLink,
  FolderKanban,
  AlertTriangle,
  Puzzle,
} from "lucide-react";
import { useRGD, useRGDResourceGraph, useRGDList } from "@/hooks/useRGDs";
import type { CatalogRGD } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { formatDateTime } from "@/lib/date";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { AddOnsTab } from "./AddOnsTab";
import { DependsOnTab } from "./DependsOnTab";
import { useKindToRGDMap } from "@/hooks/useKindToRGDMap";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab, ConditionalTab } from "@/hooks/useDynamicTabs";
import { TabBar } from "@/components/shared/TabBar";

// Lazy load ResourceGraphView to code-split @xyflow/react (~200KB)
// This ensures ReactFlow is only loaded when user views the Resources tab
const ResourceGraphView = lazy(() =>
  import("@/components/graph").then((m) => ({ default: m.ResourceGraphView }))
);

type TabId = "overview" | "resources" | "addons" | "depends-on";

const BASE_TABS: Tab<TabId>[] = [
  { id: "overview", label: "Overview", icon: <Layers className="h-4 w-4" /> },
  { id: "resources", label: "Resources", icon: <Box className="h-4 w-4" /> },
];

interface RGDDetailViewProps {
  rgd: CatalogRGD;
  onBack: () => void;
  onDeploy?: () => void;
}

export function RGDDetailView({ rgd, onBack, onDeploy }: RGDDetailViewProps) {
  // Fetch full RGD details
  const { data: fullRGD } = useRGD(rgd.name, rgd.namespace);
  const displayRGD = fullRGD || rgd;
  const isInactive = displayRGD.status !== "Active";

  // Fetch add-ons (RGDs that extend this RGD's Kind)
  const { data: addOnsData } = useRGDList(
    displayRGD.kind ? { extendsKind: displayRGD.kind, pageSize: 100 } : undefined
  );
  const addOnsCount = addOnsData?.totalCount ?? 0;

  const dependsOnCount = displayRGD.dependsOnKinds?.length || 0;

  // Build conditional tabs
  const conditionalTabs = useMemo<ConditionalTab<TabId>[]>(() => [
    {
      condition: dependsOnCount > 0,
      tab: { id: "depends-on", label: `Depends On (${dependsOnCount})`, icon: <Link2 className="h-4 w-4" /> },
    },
    {
      condition: addOnsCount > 0,
      tab: { id: "addons", label: `Add-ons (${addOnsCount})`, icon: <Puzzle className="h-4 w-4" /> },
    },
  ], [addOnsCount, dependsOnCount]);

  const { tabs, activeTab, setActiveTab } = useDynamicTabs(BASE_TABS, conditionalTabs, "overview" as TabId);

  // Reset to overview when navigating between RGDs
  const [prevRGDName, setPrevRGDName] = useState(rgd.name);
  if (prevRGDName !== rgd.name) {
    setPrevRGDName(rgd.name);
    setActiveTab("overview");
  }

  const normalizedTags = useMemo(() => {
    const category = (displayRGD.category || "uncategorized").toLowerCase();
    const uniqueTags = [...new Set(displayRGD.tags?.map((t) => t.toLowerCase()) || [])];
    return uniqueTags.filter((tag) => tag !== category);
  }, [displayRGD.tags, displayRGD.category]);

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Back button */}
      <button
        onClick={onBack}
        className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to catalog
      </button>

      {/* Header */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="h-12 w-12 rounded-lg bg-secondary flex items-center justify-center shrink-0">
              <Box className="h-6 w-6 text-muted-foreground" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold tracking-tight text-foreground">
                  {displayRGD.title || displayRGD.name}
                </h1>
                {isInactive && (
                  <Badge variant="destructive" className="gap-1">
                    <AlertTriangle className="h-3 w-3" />
                    Inactive
                  </Badge>
                )}
              </div>
              {displayRGD.title && displayRGD.title !== displayRGD.name && (
                <p className="text-sm text-muted-foreground font-mono mt-0.5">{displayRGD.name}</p>
              )}
              <div className="flex flex-wrap items-center gap-2 mt-2">
                {displayRGD.labels?.["knodex.io/project"] && (
                  <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-primary/10 text-primary border border-primary/20">
                    <FolderKanban className="h-3.5 w-3.5" />
                    {displayRGD.labels["knodex.io/project"]}
                  </span>
                )}
                {displayRGD.version && (
                  <span className="px-2.5 py-1 rounded-md text-xs font-mono font-medium text-muted-foreground bg-secondary border border-border">
                    {displayRGD.version}
                  </span>
                )}
              </div>
            </div>
          </div>

          {onDeploy && (
            <Button
              onClick={onDeploy}
              className="gap-2 shrink-0"
            >
              <ExternalLink className="h-4 w-4" />
              Deploy
            </Button>
          )}
        </div>

        {/* Description */}
        {displayRGD.description && (
          <p className="mt-4 text-sm text-muted-foreground border-t border-border pt-4">
            {displayRGD.description}
          </p>
        )}

        {/* Tags */}
        <div className="flex flex-wrap gap-2 mt-4">
          <span className="px-2.5 py-1 rounded-md text-xs font-medium bg-primary/10 text-primary">
            {(displayRGD.category || "uncategorized").toLowerCase()}
          </span>
          {normalizedTags.map((tag) => (
            <span
              key={tag}
              className="px-2.5 py-1 rounded-md text-xs text-muted-foreground bg-secondary"
            >
              {tag}
            </span>
          ))}
        </div>

        {/* Meta */}
        <div className="flex flex-wrap gap-6 mt-4 pt-4 border-t border-border text-xs text-muted-foreground">
          <Link
            to={`/instances?rgd=${encodeURIComponent(displayRGD.name)}`}
            className="flex items-center gap-1.5 hover:text-primary hover:underline transition-colors cursor-pointer"
          >
            <Package className="h-3.5 w-3.5" />
            {displayRGD.instances} instance{displayRGD.instances !== 1 ? "s" : ""}
          </Link>
          <span className="flex items-center gap-1.5">
            <Clock className="h-3.5 w-3.5" />
            Updated {formatDateTime(displayRGD.updatedAt)}
          </span>
        </div>
      </div>

      {/* Tabs */}
      <TabBar tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      {/* Tab content */}
      <div id={`panel-${activeTab}`} className="min-h-[300px]" role="tabpanel" aria-labelledby={`tab-${activeTab}`}>
        {activeTab === "overview" && (
          <OverviewTab rgd={displayRGD} />
        )}
        {activeTab === "resources" && (
          <ResourcesTab rgd={displayRGD} />
        )}
        {activeTab === "depends-on" && (
          <DependsOnTab rgd={displayRGD} />
        )}
        {activeTab === "addons" && displayRGD.kind && (
          <AddOnsTab kind={displayRGD.kind} />
        )}
      </div>
    </div>
  );
}

function OverviewTab({ rgd }: { rgd: CatalogRGD }) {
  const { kindToRGD } = useKindToRGDMap();

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="text-sm font-medium text-foreground mb-3">Details</h3>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Name</dt>
            <dd className="text-foreground font-mono">{rgd.name}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Status</dt>
            <dd className={cn(
              "text-foreground font-medium",
              rgd.status !== "Active" && "text-destructive"
            )}>
              {rgd.status || "Unknown"}
            </dd>
          </div>
          {rgd.labels?.["knodex.io/project"] && (
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Project</dt>
              <dd className="text-foreground font-mono">{rgd.labels["knodex.io/project"]}</dd>
            </div>
          )}
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Category</dt>
            <dd className="text-foreground">{rgd.category || "Uncategorized"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Version</dt>
            <dd className="text-foreground font-mono">{rgd.version || "N/A"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">API Version</dt>
            <dd className="text-foreground font-mono">{rgd.apiVersion || "N/A"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Kind</dt>
            <dd className="text-foreground font-mono">{rgd.kind || "N/A"}</dd>
          </div>
          {rgd.extendsKinds && rgd.extendsKinds.length > 0 && (
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Extends</dt>
              <dd className="text-foreground flex flex-wrap gap-1.5 justify-end">
                {rgd.extendsKinds.map((kind) => (
                  <ExtendsKindLink key={kind} kind={kind} rgd={kindToRGD.get(kind)} />
                ))}
              </dd>
            </div>
          )}
          {rgd.dependsOnKinds && rgd.dependsOnKinds.length > 0 && (
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Depends On</dt>
              <dd className="flex flex-wrap gap-1.5 justify-end">
                {rgd.dependsOnKinds.map((kind) => (
                  <DependsOnKindLink key={kind} kind={kind} />
                ))}
              </dd>
            </div>
          )}
        </dl>
      </div>

      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="text-sm font-medium text-foreground mb-3">Timestamps</h3>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Created</dt>
            <dd className="text-foreground">{formatDateTime(rgd.createdAt)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Updated</dt>
            <dd className="text-foreground">{formatDateTime(rgd.updatedAt)}</dd>
          </div>
        </dl>
      </div>

      {Object.keys(rgd.labels || {}).length > 0 && (
        <div className="rounded-lg border border-border bg-card p-4 md:col-span-2">
          <h3 className="text-sm font-medium text-foreground mb-3">Labels</h3>
          <div className="flex flex-wrap gap-2">
            {Object.entries(rgd.labels).map(([key, value]) => (
              <span
                key={key}
                className="px-2 py-1 rounded text-xs font-mono bg-secondary text-muted-foreground"
              >
                {key}: {value}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// Renders a single extends-kind as a link to the parent RGD detail page.
// Pure display component — Kind resolution is done by the parent via useKindToRGDMap.
// Falls back to plain text when the parent RGD is not in the catalog.
function ExtendsKindLink({ kind, rgd: parentRGD }: { kind: string; rgd?: CatalogRGD }) {
  if (!parentRGD) {
    return <span className="font-mono text-muted-foreground">{kind}</span>;
  }

  return (
    <Link
      to={`/catalog/${encodeURIComponent(parentRGD.name)}`}
      className="font-mono text-primary hover:underline"
    >
      {kind}
    </Link>
  );
}

function DependsOnKindLink({ kind }: { kind: string }) {
  const { kindToRGD } = useKindToRGDMap();
  const parentRGD = kindToRGD.get(kind);

  if (parentRGD) {
    return (
      <Link
        to={`/catalog/${encodeURIComponent(parentRGD.name)}`}
        className="text-primary hover:underline font-mono text-xs"
      >
        {kind}
      </Link>
    );
  }

  return <span className="text-foreground font-mono text-xs">{kind}</span>;
}

function ResourcesTab({ rgd }: { rgd: CatalogRGD }) {
  const { data: resourceGraph, isLoading, error } = useRGDResourceGraph(rgd.name, rgd.namespace);

  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <Box className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        </div>
        <div className="h-[400px] flex items-center justify-center">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            <span className="text-sm">Loading resources...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <Box className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        </div>
        <div className="h-[200px] flex flex-col items-center justify-center gap-2">
          <AlertCircle className="h-6 w-6 text-destructive" />
          <p className="text-sm text-destructive">Failed to load resources</p>
          <p className="text-xs text-muted-foreground">
            {error instanceof Error ? error.message : "Unknown error"}
          </p>
        </div>
      </div>
    );
  }

  if (!resourceGraph) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Box className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        <span className="text-xs text-muted-foreground">
          K8s resources defined in this RGD
        </span>
      </div>
      <Suspense
        fallback={
          <div className="h-[500px] rounded-lg border border-border bg-card flex items-center justify-center">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" />
              <span className="text-sm">Loading graph visualization...</span>
            </div>
          </div>
        }
      >
        <ResourceGraphView resourceGraph={resourceGraph} />
      </Suspense>
    </div>
  );
}
