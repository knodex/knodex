// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  LayoutGrid,
  Package,
  FolderKanban,
  Settings,
  Shield,
  Clock,
  Rocket,
} from "@/lib/icons";

import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from "@/components/ui/command";
import { searchAll } from "@/api/search";
import { STALE_TIME } from "@/lib/query-client";

// --- Types ---

interface RecentItem {
  id: string;
  label: string;
  href: string;
  timestamp: number;
}

// --- Constants ---

const RECENT_STORAGE_KEY = "command-palette-recent";
const MAX_RECENT_ITEMS = 10;

const NAVIGATE_ITEMS = [
  { id: "nav-instances", label: "Instances", href: "/instances", icon: Package },
  { id: "nav-catalog", label: "Catalog", href: "/catalog", icon: LayoutGrid },
  { id: "nav-projects", label: "Projects", href: "/projects", icon: FolderKanban },
  { id: "nav-secrets", label: "Secrets", href: "/secrets", icon: Shield },
  { id: "nav-settings", label: "Settings", href: "/settings", icon: Settings },
];

// --- Recent items helpers ---

function loadRecentItems(): RecentItem[] {
  try {
    const raw = localStorage.getItem(RECENT_STORAGE_KEY);
    if (!raw) return [];
    const items = JSON.parse(raw) as RecentItem[];
    return Array.isArray(items) ? items.slice(0, MAX_RECENT_ITEMS) : [];
  } catch {
    return [];
  }
}

function saveRecentItem(item: Omit<RecentItem, "timestamp">) {
  const items = loadRecentItems().filter((r) => r.id !== item.id);
  items.unshift({ ...item, timestamp: Date.now() });
  localStorage.setItem(
    RECENT_STORAGE_KEY,
    JSON.stringify(items.slice(0, MAX_RECENT_ITEMS))
  );
}

// --- Highlight helper ---

function highlightMatch(text: string, query: string): React.ReactNode {
  if (!query) return text;
  const idx = text.toLowerCase().indexOf(query.toLowerCase());
  if (idx === -1) return text;
  return (
    <>
      {text.slice(0, idx)}
      <strong className="text-foreground">{text.slice(idx, idx + query.length)}</strong>
      {text.slice(idx + query.length)}
    </>
  );
}

// --- Debounce hook ---

function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}

// --- Component ---

interface CommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CommandPalette({ open, onOpenChange }: CommandPaletteProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(query, 150);

  // Fetch search results
  const { data: searchData } = useQuery({
    queryKey: ["search", debouncedQuery],
    queryFn: () => searchAll(debouncedQuery),
    enabled: open && debouncedQuery.length > 0,
    staleTime: STALE_TIME.FREQUENT,
  });

  // Load recent items when opening with empty query
  const recentItems = useMemo(() => (open ? loadRecentItems() : []), [open]);

  // Filter navigate items by query
  const filteredNavItems = useMemo(() => {
    if (!query) return NAVIGATE_ITEMS;
    const lower = query.toLowerCase();
    return NAVIGATE_ITEMS.filter((item) =>
      item.label.toLowerCase().includes(lower)
    );
  }, [query]);

  // Map API results
  const rgdResults = searchData?.results.rgds ?? [];
  const instanceResults = searchData?.results.instances ?? [];
  const projectResults = searchData?.results.projects ?? [];

  const handleSelect = useCallback(
    (href: string, label: string, id: string) => {
      saveRecentItem({ id, label, href });
      onOpenChange(false);
      navigate(href);
    },
    [navigate, onOpenChange]
  );

  // Reset query when closing
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional: reset transient UI state when dialog closes
    if (!open) setQuery("");
  }, [open]);

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <div aria-live="polite" className="sr-only">
        {searchData
          ? `${searchData.totalCount} results found`
          : query
            ? "Searching..."
            : ""}
      </div>
      <CommandInput
        placeholder="Search RGDs, instances, projects..."
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>

        {/* Recent */}
        {!query && recentItems.length > 0 && (
          <CommandGroup heading="Recent">
            {recentItems.map((item) => (
              <CommandItem
                key={item.id}
                value={item.id}
                onSelect={() => handleSelect(item.href, item.label, item.id)}
              >
                <Clock className="mr-2 h-4 w-4 text-muted-foreground" />
                {item.label}
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {/* Navigate */}
        {filteredNavItems.length > 0 && (
          <CommandGroup heading="Navigate">
            {filteredNavItems.map((item) => (
              <CommandItem
                key={item.id}
                value={item.id}
                onSelect={() => handleSelect(item.href, item.label, item.id)}
              >
                <item.icon className="mr-2 h-4 w-4 text-muted-foreground" />
                {highlightMatch(item.label, query)}
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {/* Deploy (RGDs) */}
        {rgdResults.length > 0 && (
          <CommandGroup heading="Deploy">
            {rgdResults.map((rgd) => (
              <CommandItem
                key={`rgd-${rgd.name}`}
                value={`rgd-${rgd.name}`}
                onSelect={() =>
                  handleSelect(
                    `/catalog/${encodeURIComponent(rgd.name)}`,
                    rgd.displayName || rgd.name,
                    `rgd-${rgd.name}`
                  )
                }
              >
                <Rocket className="mr-2 h-4 w-4 text-muted-foreground" />
                <div className="flex flex-col">
                  <span>{highlightMatch(rgd.displayName || rgd.name, query)}</span>
                  {rgd.description && (
                    <span className="text-xs text-muted-foreground line-clamp-1">
                      {rgd.description}
                    </span>
                  )}
                </div>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {/* Instances */}
        {instanceResults.length > 0 && (
          <CommandGroup heading="Instances">
            {instanceResults.map((inst) => (
              <CommandItem
                key={`inst-${inst.namespace}-${inst.kind}-${inst.name}`}
                value={`inst-${inst.namespace}-${inst.kind}-${inst.name}`}
                onSelect={() =>
                  handleSelect(
                    `/instances/${encodeURIComponent(inst.namespace)}/${encodeURIComponent(inst.kind)}/${encodeURIComponent(inst.name)}`,
                    inst.name,
                    `inst-${inst.namespace}-${inst.kind}-${inst.name}`
                  )
                }
              >
                <Package className="mr-2 h-4 w-4 text-muted-foreground" />
                <div className="flex flex-col">
                  <span>{highlightMatch(inst.name, query)}</span>
                  <span className="text-xs text-muted-foreground">
                    {inst.namespace} · {inst.kind} · {inst.status}
                  </span>
                </div>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {/* Projects */}
        {projectResults.length > 0 && (
          <CommandGroup heading="Projects">
            {projectResults.map((proj) => (
              <CommandItem
                key={`proj-${proj.name}`}
                value={`proj-${proj.name}`}
                onSelect={() =>
                  handleSelect(
                    `/projects/${encodeURIComponent(proj.name)}`,
                    proj.name,
                    `proj-${proj.name}`
                  )
                }
              >
                <FolderKanban className="mr-2 h-4 w-4 text-muted-foreground" />
                <div className="flex flex-col">
                  <span>{highlightMatch(proj.name, query)}</span>
                  {proj.description && (
                    <span className="text-xs text-muted-foreground line-clamp-1">
                      {proj.description}
                    </span>
                  )}
                </div>
              </CommandItem>
            ))}
          </CommandGroup>
        )}
      </CommandList>
    </CommandDialog>
  );
}

