// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useEffect, useRef, Suspense } from "react";
import {
  Package,
  Box,
  Link2,
  Loader2,
  AlertCircle,
  ExternalLink,
  FolderKanban,
  Puzzle,
  KeyRound,
  GitBranch,
} from "@/lib/icons";
import { useRGD, useRGDDefinitionGraph, useRGDInstances, useRGDList, useRGDRevisions } from "@/hooks/useRGDs";
import { useKindToRGDMap } from "@/hooks/useKindToRGDMap";
import { StatusCard } from "@/components/instances/StatusCard";
import type { CatalogRGD } from "@/types/rgd";
import { Button } from "@/components/ui/button";
import { AddOnsTab } from "./AddOnsTab";
import { DependsOnTab } from "./DependsOnTab";
import { ScopeIndicator } from "@/components/shared/ScopeIndicator";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab, ConditionalTab } from "@/hooks/useDynamicTabs";
import { TabBar } from "@/components/shared/TabBar";
import { RGDIcon } from "@/components/ui/rgd-icon";
import { lazyWithPreload } from "@/lib/lazy-preload";

// Lazy load ResourceGraphView to code-split @xyflow/react (~200KB)
// This ensures ReactFlow is only loaded when user views the Resources tab
const ResourceGraphView = lazyWithPreload(() =>
  import("@/components/graph").then((m) => ({ default: m.ResourceGraphView }))
);

// Lazy load CatalogSecretsTab to code-split it from the main bundle.
const CatalogSecretsTab = lazyWithPreload(() =>
  import("./CatalogSecretsTab").then(m => ({ default: m.CatalogSecretsTab }))
);

// Lazy load RevisionsTab
const RevisionsTab = lazyWithPreload(() =>
  import("./RevisionsTab").then(m => ({ default: m.RevisionsTab }))
);

type TabId = "instances" | "resources" | "addons" | "depends-on" | "secrets" | "revisions";

const BASE_TABS: Tab<TabId>[] = [
  { id: "instances", label: "Instances", icon: <Package className="h-4 w-4" /> },
  { id: "resources", label: "Resources", icon: <Box className="h-4 w-4" /> },
];

interface RGDDetailViewProps {
  rgd: CatalogRGD;
  /** @deprecated Breadcrumbs handle navigation — this prop is unused */
  onBack?: () => void;
  onDeploy?: () => void;
  /** Optional tab to activate on mount (e.g., "revisions" from revision badge link) */
  initialTab?: TabId;
}

export function RGDDetailView({ rgd, onDeploy, initialTab }: RGDDetailViewProps) {
  // Fetch full RGD details
  const { data: fullRGD } = useRGD(rgd.name, rgd.namespace);
  const displayRGD = fullRGD || rgd;
  // Fetch add-ons (RGDs that extend this RGD's Kind)
  const { data: addOnsData } = useRGDList(
    displayRGD.kind ? { extendsKind: displayRGD.kind, pageSize: 100 } : undefined
  );
  const addOnsCount = addOnsData?.totalCount ?? 0;

  // Only count dependencies that exist in the catalog (hides tab when all deps are external)
  const { kindToRGD, isLoading: kindMapLoading } = useKindToRGDMap();
  const dependsOnCount = kindMapLoading
    ? (displayRGD.dependsOnKinds?.length || 0)
    : (displayRGD.dependsOnKinds?.filter((k) => kindToRGD.has(k)).length || 0);
  const secretRefsCount = displayRGD.secretRefs?.length ?? 0;

  // Fetch revision count (React Query deduplicates with RevisionsTab)
  const { data: revisionsData } = useRGDRevisions(displayRGD.name);
  const revisionsCount = revisionsData?.totalCount ?? 0;
  // Use lastIssuedRevision from the catalog API (populated from KRO RGD status).
  // Falls back to max revision number if field not yet available.
  const currentRevision = useMemo(() => {
    if (displayRGD?.lastIssuedRevision) return displayRGD.lastIssuedRevision;
    const items = revisionsData?.items;
    if (!items || items.length === 0) return undefined;
    return Math.max(...items.map(r => r.revisionNumber));
  }, [displayRGD?.lastIssuedRevision, revisionsData]);

  // Build conditional tabs
  const conditionalTabs = useMemo<ConditionalTab<TabId>[]>(() => [
    {
      condition: secretRefsCount > 0,
      tab: { id: "secrets", label: `Secrets (${secretRefsCount})`, icon: <KeyRound className="h-4 w-4" /> },
    },
    {
      condition: dependsOnCount > 0,
      tab: { id: "depends-on", label: `Depends On (${dependsOnCount})`, icon: <Link2 className="h-4 w-4" /> },
    },
    {
      condition: addOnsCount > 0,
      tab: { id: "addons", label: `Add-ons (${addOnsCount})`, icon: <Puzzle className="h-4 w-4" /> },
    },
    {
      condition: revisionsCount > 0,
      tab: { id: "revisions", label: `Revisions (${revisionsCount})`, icon: <GitBranch className="h-4 w-4" /> },
    },
  ], [addOnsCount, dependsOnCount, secretRefsCount, revisionsCount]);

  const { tabs, activeTab, setActiveTab } = useDynamicTabs(BASE_TABS, conditionalTabs, "instances" as TabId);

  // Switch to initialTab once the target tab appears in the dynamic list (e.g., after data loads)
  const appliedInitialTab = useRef(false);
  useEffect(() => {
    if (initialTab && !appliedInitialTab.current && tabs.some(t => t.id === initialTab)) {
      setActiveTab(initialTab);
      appliedInitialTab.current = true;
    }
  }, [initialTab, tabs, setActiveTab]);

  // Reset to instances when navigating between RGDs; leave appliedInitialTab guard
  // unchanged so initialTab can still activate on the new RGD via the useEffect above.
  const [prevRGDName, setPrevRGDName] = useState(rgd.name);
  if (prevRGDName !== rgd.name) {
    setPrevRGDName(rgd.name);
    setActiveTab("instances");
  }

  const normalizedTags = useMemo(() => {
    const category = (displayRGD.category || "uncategorized").toLowerCase();
    const uniqueTags = [...new Set(displayRGD.tags?.map((t) => t.toLowerCase()) || [])];
    return uniqueTags.filter((tag) => tag !== category);
  }, [displayRGD.tags, displayRGD.category]);

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="h-12 w-12 rounded-lg bg-secondary text-muted-foreground flex items-center justify-center shrink-0">
              <RGDIcon icon={displayRGD.icon} category={displayRGD.category || "uncategorized"} className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold tracking-tight text-foreground">
                  {displayRGD.title || displayRGD.name}
                </h1>
                {displayRGD.isClusterScoped && (
                  <ScopeIndicator isClusterScoped variant="badge" />
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

      </div>

      {/* Tabs */}
      <TabBar tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      {/* Tab content */}
      <div id={`panel-${activeTab}`} className="min-h-[300px]" role="tabpanel" aria-labelledby={`tab-${activeTab}`}>
        {activeTab === "instances" && (
          <InstancesTab rgd={displayRGD} onDeploy={onDeploy} />
        )}
        {activeTab === "resources" && (
          <ResourcesTab rgd={displayRGD} />
        )}
        {activeTab === "secrets" && displayRGD.secretRefs && CatalogSecretsTab && (
          <Suspense fallback={<div className="flex items-center justify-center min-h-[300px]"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>}>
            <CatalogSecretsTab secretRefs={displayRGD.secretRefs} />
          </Suspense>
        )}
        {activeTab === "depends-on" && (
          <DependsOnTab rgd={displayRGD} />
        )}
        {activeTab === "addons" && displayRGD.kind && (
          <AddOnsTab kind={displayRGD.kind} />
        )}
        {activeTab === "revisions" && (
          <Suspense fallback={<div className="flex items-center justify-center min-h-[300px]"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>}>
            <RevisionsTab rgdName={displayRGD.name} currentRevision={currentRevision} />
          </Suspense>
        )}
      </div>
    </div>
  );
}



function InstancesTab({ rgd, onDeploy }: { rgd: CatalogRGD; onDeploy?: () => void }) {
  const { data, isLoading } = useRGDInstances(rgd.name, rgd.namespace);
  const instances = data?.items ?? [];

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[300px]">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin" />
          <span className="text-sm">Loading instances...</span>
        </div>
      </div>
    );
  }

  if (instances.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[300px] text-center gap-3">
        <Package className="h-10 w-10 text-muted-foreground/40" />
        <p className="text-sm font-medium text-muted-foreground">No instances deployed yet</p>
        <p className="text-xs text-muted-foreground/70">Deploy an instance to see it here.</p>
        {onDeploy && (
          <Button onClick={onDeploy} className="gap-2 mt-2" data-testid="empty-state-deploy">
            <ExternalLink className="h-4 w-4" />
            Deploy
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))" }}>
      {instances.map((inst, index) => (
        <div
          key={inst.uid || `${inst.namespace}/${inst.name}`}
          className="animate-card-enter"
          style={{ animationDelay: `${Math.min(index * 40, 400)}ms` }}
        >
          <StatusCard instance={inst} hideKind />
        </div>
      ))}
    </div>
  );
}


function ResourcesTab({ rgd }: { rgd: CatalogRGD }) {
  const { data: resourceGraph, isLoading, error } = useRGDDefinitionGraph(rgd.name, rgd.namespace);

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
