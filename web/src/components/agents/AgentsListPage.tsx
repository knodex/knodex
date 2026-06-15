// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Bot, LayoutGrid, List, RefreshCw, Search, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { FilterChip, FilterChipDot, filterChipClasses } from "@/components/ui/filter-chip";
import { FiltersDropdown } from "@/components/ui/filters-dropdown";
import { useAgents } from "@/hooks/useAgents";
import { useAgentRuns } from "@/hooks/useAgentRuns";
import { useWebSocket } from "@/hooks/useWebSocket";
import { useIsMobile } from "@/hooks/useIsMobile";
import { AgentCard } from "@/components/agents/AgentCard";
import { AgentsListView } from "@/components/agents/AgentsListView";
import { AgentModelEditModal, type EditableAgent } from "@/components/agents/AgentModelEditModal";
import { PastConversationsSection } from "@/components/agents/PastConversationsSection";
import type { AgentModel, InstalledAgent } from "@/api/agents";
import { cn } from "@/lib/utils";

const AGENTS_VIEW_KEY = "knodex.agents.view";
type ViewMode = "grid" | "list";

// Select requires non-empty values, so "all" needs a sentinel.
const ALL_VALUE = "__all__";

/** Resolved model label, identical to AgentModelBadge's "provider · name". "" when unresolved. */
function modelLabel(model?: AgentModel | null): string {
  if (!model || (!model.provider && !model.name)) return "";
  return [model.provider, model.name].filter(Boolean).join(" · ");
}

/** Resolve initial view mode: persisted choice wins, otherwise default to `list` (mirrors Instances). */
function readInitialViewMode(): ViewMode {
  const stored = localStorage.getItem(AGENTS_VIEW_KEY);
  return stored === "grid" || stored === "list" ? stored : "list";
}

/** Section-level retry card (does NOT blow away the rest of the tab). */
function SectionError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-lg border border-border py-10"
      data-testid="agents-list-error"
    >
      <p className="text-sm text-muted-foreground">
        Agents could not be loaded. This is usually transient.
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
 * Agents tab (Story 53.2): one Casbin-scoped list of the caller's agents from
 * GET /api/v1/agents. Every card/row is namespaced and chat-enabled (the single
 * namespaced invoke path) — no hub/installed split.
 *
 * Toolbar mirrors the Instances page: a search/filter bar + a list/grid view
 * toggle. Agents have no deploy/create action here (they arrive via the
 * Catalog), so the primary-action slot is intentionally empty.
 */
export function AgentsListPage() {
  // Live run updates (Story 49.4, UX-DR6): the central useWebSocket handleMessage
  // invalidates ["agents","runs"] for the in-flight indicator below.
  useWebSocket({ subscriptions: ["agent_runs"] });

  const isMobile = useIsMobile();
  const navigate = useNavigate();

  // Model-edit modal target: null when closed. Lifted here so a single modal
  // instance serves every card/row.
  const [editing, setEditing] = useState<EditableAgent | null>(null);
  const [search, setSearch] = useState("");
  const [namespace, setNamespace] = useState("");
  const [model, setModel] = useState("");

  // Mobile renders the card grid only, so skip the localStorage read on mobile.
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    isMobile ? "grid" : readInitialViewMode()
  );

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem(AGENTS_VIEW_KEY, mode);
  }, []);

  const { data, isLoading, isError, isFetching, refetch } = useAgents();

  // Live in-flight indicator (Story 49.4): one query for all running runs, under
  // the same ["agents","runs"] prefix as the history so a single WebSocket
  // invalidation refreshes both.
  const { data: runningRuns } = useAgentRuns({ status: "running", pageSize: 100 });
  const isAgentRunning = useCallback(
    (name: string, namespace: string) =>
      runningRuns?.items.some(
        (run) => run.agentType === name && run.agentNamespace === namespace
      ) ?? false,
    [runningRuns]
  );

  // Filter-option sources: distinct namespaces + resolved model labels across
  // the (Casbin-scoped) agent list.
  const namespaceOptions = useMemo(() => {
    const set = new Set<string>();
    for (const a of data?.agents ?? []) set.add(a.namespace);
    return [...set].sort();
  }, [data?.agents]);

  const modelOptions = useMemo(() => {
    const set = new Set<string>();
    for (const a of data?.agents ?? []) {
      const label = modelLabel(a.model);
      if (label) set.add(label);
    }
    return [...set].sort();
  }, [data?.agents]);

  // Client-side filter: search (name/namespace/description) + namespace + model.
  const filtered = useMemo(() => {
    const agents = data?.agents ?? [];
    const q = search.trim().toLowerCase();
    return agents.filter((a) => {
      if (namespace && a.namespace !== namespace) return false;
      if (model && modelLabel(a.model) !== model) return false;
      if (
        q &&
        !a.name.toLowerCase().includes(q) &&
        !a.namespace.toLowerCase().includes(q) &&
        !a.description.toLowerCase().includes(q)
      ) {
        return false;
      }
      return true;
    });
  }, [data?.agents, search, namespace, model]);

  const activeFilterCount = (namespace ? 1 : 0) + (model ? 1 : 0);
  const hasActiveFilters = Boolean(search || namespace || model);
  const clearFilters = useCallback(() => {
    setSearch("");
    setNamespace("");
    setModel("");
  }, []);

  const chatHref = useCallback(
    (agent: InstalledAgent) =>
      `/agents/list/${encodeURIComponent(agent.namespace)}/${encodeURIComponent(agent.name)}`,
    []
  );

  const openChat = useCallback(
    (agent: InstalledAgent) => navigate(chatHref(agent)),
    [navigate, chatHref]
  );

  const openEditor = useCallback(
    (agent: InstalledAgent) =>
      setEditing({
        name: agent.name,
        namespace: agent.namespace,
        model: agent.model,
        modelConfig: agent.modelConfig,
      }),
    []
  );

  const showList = viewMode === "list" && !isMobile;

  return (
    <div className="space-y-8" data-testid="agents-list">
      {/* Toolbar: search + view toggle (mirrors InstancesPage). No deploy/create. */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 min-w-0">
          <Search className={filterSearchIconClasses} />
          <Input
            type="text"
            placeholder="Filter agents..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className={filterSearchClasses}
            aria-label="Search agents"
            data-testid="agents-search"
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

        {/* Namespace + Model filters, collapsed into a single Filters popover. */}
        <FiltersDropdown activeCount={activeFilterCount} className="shrink-0">
          <div className="flex flex-col items-start gap-2">
            <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70">
              Filter by
            </span>

            {/* Namespace */}
            <Select
              value={namespace || ALL_VALUE}
              onValueChange={(v) => setNamespace(v === ALL_VALUE ? "" : v)}
            >
              <SelectTrigger
                className={cn(filterChipClasses(namespace ? "active" : "idle"), "max-w-[220px]")}
                aria-label="Filter by namespace"
                data-testid="agents-namespace-filter"
              >
                <span className="inline-flex items-center gap-1.5 truncate">
                  {namespace && <FilterChipDot />}
                  <span className="truncate">
                    {namespace ? `Namespace: ${namespace}` : "Namespace"}
                  </span>
                </span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_VALUE}>All namespaces</SelectItem>
                {namespaceOptions.map((ns) => (
                  <SelectItem key={ns} value={ns} className="text-xs">
                    {ns}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {/* Model */}
            <Select
              value={model || ALL_VALUE}
              onValueChange={(v) => setModel(v === ALL_VALUE ? "" : v)}
            >
              <SelectTrigger
                className={cn(filterChipClasses(model ? "active" : "idle"), "max-w-[220px]")}
                aria-label="Filter by model"
                data-testid="agents-model-filter"
              >
                <span className="inline-flex items-center gap-1.5 truncate">
                  {model && <FilterChipDot />}
                  <span className="truncate">{model ? `Model: ${model}` : "Model"}</span>
                </span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_VALUE}>All models</SelectItem>
                {modelOptions.map((m) => (
                  <SelectItem key={m} value={m} className="text-xs">
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </FiltersDropdown>

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

      {/* Active filter pills (namespace/model) with a Clear-all affordance. */}
      {(namespace || model) && (
        <div className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-2 text-muted-foreground/70">
            <span>Filters:</span>
            <div className="flex flex-wrap gap-1.5">
              {namespace && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => setNamespace("")}
                  aria-label="Remove namespace filter"
                  data-testid="remove-namespace-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{namespace}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
              )}
              {model && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => setModel("")}
                  aria-label="Remove model filter"
                  data-testid="remove-model-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{model}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={clearFilters}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-muted-foreground/70 hover:text-foreground hover:bg-muted/30 transition-all duration-200 font-medium"
            aria-label="Clear all filters"
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-40 w-full" data-testid="agents-list-loading" />
      ) : isError || !data ? (
        <SectionError onRetry={() => void refetch()} />
      ) : data.agents.length === 0 ? (
        <EmptyState
          icon={Bot}
          title="No agents available"
          description="No agents are deployed in the namespaces your access can reach. Deploy one from a template and it will appear here."
          className="py-10 rounded-lg border border-dashed border-border"
          action={
            <div className="flex flex-col items-center gap-2">
              <Link
                to="/agents/templates"
                className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                Deploy from a template
              </Link>
              <Link
                to="/catalog"
                className="text-[13px] underline underline-offset-2"
                style={{ color: "var(--text-secondary)" }}
              >
                or browse the full Catalog
              </Link>
            </div>
          }
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Search}
          title="No matching agents"
          description="Try adjusting your search or filters."
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
        <AgentsListView
          items={filtered}
          isRunning={isAgentRunning}
          hrefForAgent={chatHref}
          onAgentClick={openChat}
          onEdit={openEditor}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((agent) => (
            <AgentCard
              key={`${agent.namespace}/${agent.name}`}
              name={agent.name}
              namespace={agent.namespace}
              description={agent.description}
              model={agent.model}
              running={isAgentRunning(agent.name, agent.namespace)}
              to={chatHref(agent)}
              onEdit={() => openEditor(agent)}
            />
          ))}
        </div>
      )}

      <PastConversationsSection />

      {/* Mounted only while editing — no idle Dialog in the ready-state tree. */}
      {editing && (
        <AgentModelEditModal
          open
          onOpenChange={(o) => {
            if (!o) setEditing(null);
          }}
          agent={editing}
        />
      )}
    </div>
  );
}

export default AgentsListPage;
