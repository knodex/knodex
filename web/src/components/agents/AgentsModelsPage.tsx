// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { Boxes, Plus, RefreshCw, Search, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
  filterSelectClasses,
} from "@/components/ui/filter-bar";
import { useModels } from "@/hooks/useAgents";
import { CreateModelDialog } from "@/components/agents/CreateModelDialog";
import { cn } from "@/lib/utils";

// Select requires non-empty option values; this sentinel stands in for "all".
const ALL_PROVIDERS = "__all_providers__";

/** Section-level retry card (does NOT blow away the rest of the tab). */
function SectionError({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-lg border border-border py-10"
      data-testid="agents-models-error"
    >
      <p className="text-sm text-muted-foreground">
        Models could not be loaded. This is usually transient.
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
 * Models tab (Story 53.4): the caller's Casbin-scoped kagent ModelConfigs from
 * GET /api/v1/agents/models, plus a Create Model dialog that orchestrates a
 * Secret + a KnodexAgentModelConfig instance behind one POST. A workspace with zero
 * models needs one created before an agent has a provider to run on — the empty
 * state says so.
 *
 * Toolbar mirrors the Agents page: a search/filter bar plus a provider filter.
 */
export function AgentsModelsPage() {
  const [createOpen, setCreateOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [provider, setProvider] = useState("");
  const { data, isLoading, isError, refetch } = useModels();

  // Hide the toolbar/header button on the empty state — the empty state owns its
  // own Create Model CTA, so a search bar over zero models would be noise.
  const hasModels = !!data && data.models.length > 0;

  // Distinct providers across the visible models drive the provider filter.
  const providers = useMemo(() => {
    const set = new Set<string>();
    for (const m of data?.models ?? []) {
      if (m.provider) set.add(m.provider);
    }
    return [...set].sort();
  }, [data?.models]);

  // Client-side filter over name/namespace/provider/model + provider select.
  const filtered = useMemo(() => {
    const models = data?.models ?? [];
    const q = search.trim().toLowerCase();
    return models.filter((m) => {
      if (provider && m.provider !== provider) return false;
      if (!q) return true;
      return (
        m.name.toLowerCase().includes(q) ||
        m.namespace.toLowerCase().includes(q) ||
        m.provider.toLowerCase().includes(q) ||
        m.model.toLowerCase().includes(q)
      );
    });
  }, [data?.models, search, provider]);

  return (
    <div className="space-y-8" data-testid="agents-models-page">
      {hasModels && (
        <div className="flex items-center gap-3">
          <div className="relative flex-1 min-w-0">
            <Search className={filterSearchIconClasses} />
            <Input
              type="text"
              placeholder="Filter models..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className={filterSearchClasses}
              aria-label="Search models"
              data-testid="agents-models-search"
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
          <Select
            value={provider || ALL_PROVIDERS}
            onValueChange={(v) => setProvider(v === ALL_PROVIDERS ? "" : v)}
          >
            <SelectTrigger
              className={cn(filterSelectClasses(!!provider), "w-[170px] shrink-0")}
              aria-label="Filter by provider"
              data-testid="agents-models-provider-filter"
            >
              <span className="truncate">
                {provider ? `Provider: ${provider}` : "All providers"}
              </span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_PROVIDERS}>All providers</SelectItem>
              {providers.map((p) => (
                <SelectItem key={p} value={p} className="text-xs">
                  {p}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <button
            type="button"
            onClick={() => setCreateOpen(true)}
            data-testid="create-model-button"
            className="inline-flex shrink-0 items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            <Plus className="h-4 w-4" />
            Create Model
          </button>
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-40 w-full" data-testid="agents-models-loading" />
      ) : isError || !data ? (
        <SectionError onRetry={() => void refetch()} />
      ) : data.models.length === 0 ? (
        <EmptyState
          icon={Boxes}
          title="No models yet"
          description="Create a model so your agents have a provider to run on. You'll need one before you can deploy an agent."
          className="py-10 rounded-lg border border-dashed border-border"
          action={
            <button
              type="button"
              onClick={() => setCreateOpen(true)}
              data-testid="create-model-button-empty"
              className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            >
              <Plus className="h-4 w-4" />
              Create Model
            </button>
          }
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Search}
          title="No matching models"
          description="Try adjusting your search or provider filter."
          className="py-10 rounded-lg border border-dashed border-border"
          action={
            <button
              type="button"
              onClick={() => {
                setSearch("");
                setProvider("");
              }}
              className="text-[13px] underline underline-offset-2"
              style={{ color: "var(--text-secondary)" }}
            >
              Clear filters
            </button>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm" data-testid="agents-models-table">
            <thead className="bg-muted/50 text-left text-xs uppercase text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Namespace</th>
                <th className="px-4 py-3 font-medium">Provider</th>
                <th className="px-4 py-3 font-medium">Model</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((m) => (
                <tr
                  key={`${m.namespace}/${m.name}`}
                  className="border-t border-border"
                  data-testid="agents-models-row"
                >
                  <td className="px-4 py-3 font-medium">{m.name}</td>
                  <td className="px-4 py-3 text-muted-foreground">{m.namespace}</td>
                  <td className="px-4 py-3">{m.provider}</td>
                  <td className="px-4 py-3">{m.model}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CreateModelDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  );
}

export default AgentsModelsPage;
