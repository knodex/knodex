// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { FileCode, LayoutGrid, List, RefreshCw, Search, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import { MultiSelect } from "@/components/ui/multi-select";
import { useAgentTemplates } from "@/hooks/useAgents";
import { useIsMobile } from "@/hooks/useIsMobile";
import { AgentsTemplatesListView } from "@/components/agents/AgentsTemplatesListView";
import { TemplateCard } from "@/components/agents/TemplateCard";
import { cn } from "@/lib/utils";

const TEMPLATES_VIEW_KEY = "knodex.agent-templates.view";
type ViewMode = "grid" | "list";

/** Resolve initial view mode: persisted choice wins, otherwise default to `list` (mirrors Agents). */
function readInitialViewMode(): ViewMode {
  const stored = localStorage.getItem(TEMPLATES_VIEW_KEY);
  return stored === "grid" || stored === "list" ? stored : "list";
}

/** Section-level retry card (does NOT blow away the rest of the tab). */
function SectionError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-lg border border-border py-10"
      data-testid="agents-templates-error"
    >
      <p className="text-sm text-muted-foreground">
        Templates could not be loaded. This is usually transient.
      </p>
      <button
        onClick={onRetry}
        className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
      >
        <RefreshCw className="h-4 w-4" />
        Retry
      </button>
    </div>
  );
}

/**
 * Templates tab: agent-template RGDs discovered by schema.kind
 * (KnodexAgentTemplate) from GET /api/v1/agents/templates — independent of the
 * catalog annotation. Each row deploys via the standard Deploy flow
 * (/deploy/{name}), so a template is just an RGD the operator can instantiate.
 *
 * Toolbar mirrors the Agents page: a search/filter bar + a list/grid view
 * toggle.
 */
export function AgentsTemplatesPage() {
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const { data, isLoading, isError, isFetching, refetch } = useAgentTemplates();

  const [search, setSearch] = useState("");
  // Tag filter (reuses the catalog's knodex.io/tags model, already on CatalogRGD).
  const [selectedTags, setSelectedTags] = useState<string[]>([]);

  // Mobile renders the card grid only, so skip the localStorage read on mobile.
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    isMobile ? "grid" : readInitialViewMode()
  );

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem(TEMPLATES_VIEW_KEY, mode);
  }, []);

  const deploy = useCallback(
    (name: string) => navigate(`/deploy/${encodeURIComponent(name)}`),
    [navigate]
  );

  // Distinct tags across the loaded templates (normalized lowercase), source for
  // the tag filter — mirrors the catalog's availableTags.
  const availableTags = useMemo(() => {
    const set = new Set<string>();
    for (const t of data?.items ?? []) {
      for (const tag of t.tags ?? []) {
        const norm = tag.trim().toLowerCase();
        if (norm) set.add(norm);
      }
    }
    return [...set].sort();
  }, [data?.items]);

  // Client-side filter: search (name/title/description/tags) + tag AND-match.
  const filtered = useMemo(() => {
    const items = data?.items ?? [];
    const q = search.trim().toLowerCase();
    return items.filter((t) => {
      if (selectedTags.length > 0) {
        const itemTags = new Set((t.tags ?? []).map((x) => x.toLowerCase()));
        if (!selectedTags.every((tag) => itemTags.has(tag))) return false;
      }
      if (
        q &&
        !t.name.toLowerCase().includes(q) &&
        !(t.title ?? "").toLowerCase().includes(q) &&
        !t.description.toLowerCase().includes(q) &&
        !(t.tags ?? []).some((x) => x.toLowerCase().includes(q))
      ) {
        return false;
      }
      return true;
    });
  }, [data?.items, search, selectedTags]);

  const clearFilters = useCallback(() => {
    setSearch("");
    setSelectedTags([]);
  }, []);
  const hasActiveFilters = Boolean(search || selectedTags.length > 0);

  const showList = viewMode === "list" && !isMobile;
  const hasTemplates = !!data && data.items.length > 0;

  return (
    <div className="space-y-8" data-testid="agents-templates-page">
      {/* Toolbar: search + view toggle (mirrors AgentsListPage). */}
      {hasTemplates && (
        <div className="flex items-center gap-3">
          <div className="relative flex-1 min-w-0">
            <Search className={filterSearchIconClasses} />
            <Input
              type="text"
              placeholder="Filter templates..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className={filterSearchClasses}
              aria-label="Search templates"
              data-testid="agents-templates-search"
            />
            {search && (
              <button
                type="button"
                onClick={() => setSearch("")}
                className={filterClearButtonClasses}
                aria-label="Clear search input"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
          {availableTags.length > 0 && (
            <MultiSelect
              options={availableTags.map((tag) => ({ label: tag, value: tag }))}
              selected={selectedTags}
              onChange={setSelectedTags}
              placeholder="Filter by tag..."
              className="min-w-[180px] shrink-0"
            />
          )}
          {!isMobile && (
            <div className="flex items-center gap-2 shrink-0">
              {isFetching && !isLoading && (
                <span className="flex items-center gap-2 text-xs text-muted-foreground">
                  <RefreshCw className="h-3 w-3 animate-spin" />
                </span>
              )}
              <div
                className="flex items-center h-9 border border-[var(--border-default)] rounded-[var(--radius-token-md)] p-0.5"
                role="group"
                aria-label="View mode"
              >
                <button
                  onClick={() => handleViewModeChange("list")}
                  className={cn(
                    "h-full px-2 rounded-[var(--radius-token-sm)] transition-colors",
                    viewMode === "list"
                      ? "bg-[var(--brand-primary)] text-black"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                  aria-label="Table view"
                  aria-pressed={viewMode === "list"}
                >
                  <List className="h-3.5 w-3.5" />
                </button>
                <button
                  onClick={() => handleViewModeChange("grid")}
                  className={cn(
                    "h-full px-2 rounded-[var(--radius-token-sm)] transition-colors",
                    viewMode === "grid"
                      ? "bg-[var(--brand-primary)] text-black"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                  aria-label="Grid view"
                  aria-pressed={viewMode === "grid"}
                >
                  <LayoutGrid className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Active tag-filter pills with a Clear-all affordance. */}
      {hasTemplates && selectedTags.length > 0 && (
        <div className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-2 text-muted-foreground/70">
            <span>Tags:</span>
            <div className="flex flex-wrap gap-1.5">
              {selectedTags.map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => setSelectedTags((prev) => prev.filter((t) => t !== tag))}
                  className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80 hover:bg-muted/60 transition-colors"
                  aria-label={`Remove ${tag} tag filter`}
                  data-testid="remove-template-tag-filter"
                >
                  <span>{tag}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </button>
              ))}
            </div>
          </div>
          <button
            type="button"
            onClick={() => setSelectedTags([])}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-muted-foreground/70 hover:text-foreground hover:bg-muted/30 transition-all duration-200 font-medium"
            aria-label="Clear all tag filters"
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-40 w-full" data-testid="agents-templates-loading" />
      ) : isError || !data ? (
        <SectionError onRetry={() => void refetch()} />
      ) : data.items.length === 0 ? (
        <EmptyState
          icon={FileCode}
          title="No agent templates"
          description="Agent templates are catalog-annotated RGDs with kind KnodexAgentTemplate. Apply one to the cluster (e.g. the RGD Builder agent) and it appears here, ready to deploy."
          className="py-10 rounded-lg border border-dashed border-border"
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Search}
          title="No matching templates"
          description="Try adjusting your search or tag filters."
          className="py-10 rounded-lg border border-dashed border-border"
          action={
            hasActiveFilters ? (
              <button
                type="button"
                onClick={clearFilters}
                className="text-[13px] underline underline-offset-2"
                style={{ color: "var(--text-secondary)" }}
              >
                Clear filters
              </button>
            ) : undefined
          }
        />
      ) : showList ? (
        <AgentsTemplatesListView items={filtered} onDeploy={deploy} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((t) => (
            <TemplateCard
              key={t.name}
              name={t.name}
              title={t.title}
              description={t.description}
              instances={t.instances}
              tags={t.tags}
              onDeploy={() => deploy(t.name)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default AgentsTemplatesPage;
