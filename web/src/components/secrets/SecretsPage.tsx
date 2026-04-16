// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useQueries } from "@tanstack/react-query";
import { KeyRound, Plus, Search, ShieldAlert, Trash2, X } from "@/lib/icons";
import { useSecretList } from "@/hooks/useSecrets";
import { useProjects } from "@/hooks/useProjects";
import { listSecrets } from "@/api/secrets";
import { useCanI } from "@/hooks/useCanI";
import { STALE_TIME } from "@/lib/query-client";
import { useCurrentProject } from "@/hooks/useAuth";
import { formatDistanceToNow } from "@/lib/date";
import { getSafeErrorMessage } from "@/lib/errors";
import {
  Table,
  TableHeader,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { SortableHead } from "@/components/ui/sortable-table";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import { CreateSecretDialog } from "./CreateSecretDialog";
import { DeleteSecretDialog } from "./DeleteSecretDialog";
import { PageHeader } from "@/components/layout/PageHeader";
import type { Secret } from "@/types/secret";

type SortField = "name" | "namespace" | "keys" | "createdAt";
type SortDir = "asc" | "desc";

export function SecretsPage() {
  const navigate = useNavigate();
  const selectedProject = useCurrentProject() ?? "";
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ name: string; namespace: string } | null>(null);
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [searchQuery, setSearchQuery] = useState("");

  const { allowed: canGet, isLoading: permLoading } = useCanI("secrets", "get", selectedProject || "-");
  // Only check create/delete permissions when a project is selected — on "All Projects",
  // the Create button is always shown since the dialog has its own project selector.
  const { allowed: canCreateInProject } = useCanI("secrets", "create", selectedProject || "-");
  const canCreate = selectedProject ? canCreateInProject : true;
  const { allowed: canDelete } = useCanI("secrets", "delete", selectedProject || "-");

  // Single-project fetch (when a project is selected)
  const { data: singleData, isLoading: singleLoading, isError: singleIsError, error: singleError, refetch: singleRefetch } = useSecretList(selectedProject);

  // All-projects fetch (when "All Projects" is selected)
  const { data: projectsData } = useProjects();
  const allProjectNames = useMemo(() => projectsData?.items?.map((p) => p.name) ?? [], [projectsData]);

  const allProjectQueries = useQueries({
    queries: !selectedProject
      ? allProjectNames.map((proj) => ({
          queryKey: ["secrets", proj],
          queryFn: () => listSecrets(proj),
          staleTime: STALE_TIME.FREQUENT,
        }))
      : [],
  });

  // Merge results
  const { data, isLoading, isError, error, refetch } = useMemo(() => {
    if (selectedProject) {
      return { data: singleData, isLoading: singleLoading, isError: singleIsError, error: singleError, refetch: singleRefetch };
    }
    // All projects mode
    const allLoading = allProjectQueries.some((q) => q.isLoading);
    const anyError = allProjectQueries.find((q) => q.isError);
    const mergedItems = allProjectQueries.flatMap((q) => q.data?.items ?? []);
    return {
      data: allProjectNames.length > 0 ? { items: mergedItems, pageCount: 1 } : undefined,
      isLoading: allLoading,
      isError: !!anyError,
      error: anyError?.error ?? null,
      refetch: () => { allProjectQueries.forEach((q) => q.refetch()); },
    };
  }, [selectedProject, singleData, singleLoading, singleIsError, singleError, singleRefetch, allProjectQueries, allProjectNames]);

  const handleSort = useCallback((field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  }, [sortField]);

  const sorted = useMemo(() => {
    let items = data?.items ?? [];
    if (items.length === 0) return [];

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      items = items.filter(
        (s) =>
          s.name.toLowerCase().includes(q) ||
          s.namespace.toLowerCase().includes(q) ||
          s.keys.some((k) => k.toLowerCase().includes(q))
      );
    }

    return [...items].sort((a, b) => {
      let aVal: string | number;
      let bVal: string | number;

      switch (sortField) {
        case "name":
          aVal = a.name.toLowerCase();
          bVal = b.name.toLowerCase();
          break;
        case "namespace":
          aVal = a.namespace.toLowerCase();
          bVal = b.namespace.toLowerCase();
          break;
        case "keys":
          aVal = a.keys.length;
          bVal = b.keys.length;
          break;
        case "createdAt":
          aVal = a.createdAt;
          bVal = b.createdAt;
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [data?.items, sortField, sortDir, searchQuery]);

  // Access denied state (only when a specific project is selected)
  if (selectedProject && !permLoading && canGet === false) {
    return (
      <section className="flex flex-col items-center justify-center py-16 text-center">
        <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-semibold">Access Denied</h2>
        <p className="text-sm text-muted-foreground mt-1">
          You don't have permission to view secrets.
        </p>
      </section>
    );
  }

  const totalItems = data?.items ?? [];
  const hasSecrets = totalItems.length > 0;
  const doneLoading = !isLoading && !permLoading;

  return (
    <section className="space-y-6">
      {/* Header */}
      <PageHeader title="Secrets" />

      {/* Loading */}
      {!doneLoading ? (
        <ListSkeleton />
      ) : isError ? (
        <div className="flex flex-col items-center justify-center py-12">
          <Alert variant="destructive" showIcon onRetry={() => refetch()} className="max-w-md">
            <AlertTitle>Failed to load secrets</AlertTitle>
            <AlertDescription>{getSafeErrorMessage(error)}</AlertDescription>
          </Alert>
        </div>
      ) : !hasSecrets ? (
        /* Empty state — no secrets at all */
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted mb-5">
            <KeyRound className="h-8 w-8 text-muted-foreground" />
          </div>
          <h3 className="text-base font-semibold mb-1">No secrets yet</h3>
          <p className="text-sm text-muted-foreground mb-6 max-w-sm">
            Start adding secrets to store credentials, tokens, and sensitive configuration for your deployments.
          </p>
          {canCreate && (
            <button
              onClick={() => setIsCreateOpen(true)}
              className="inline-flex items-center h-9 gap-2 rounded-[var(--radius-token-md)] px-4 text-sm font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97]"
            >
              <Plus className="h-4 w-4" />
              Create Secret
            </button>
          )}
        </div>
      ) : (
        <>
          {/* Search + Create on same row */}
          <div className="flex items-center gap-2">
            <div className="relative flex-1 min-w-[280px]">
              <Search className={filterSearchIconClasses} />
              <Input
                type="text"
                placeholder="Filter secrets..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className={filterSearchClasses}
                aria-label="Search secrets"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery("")}
                  className={filterClearButtonClasses}
                  aria-label="Clear search"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
            {canCreate && (
              <button
                onClick={() => setIsCreateOpen(true)}
                className="inline-flex items-center h-8 gap-1.5 rounded-[var(--radius-token-md)] px-2.5 text-xs font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97] shrink-0"
              >
                <Plus className="h-3 w-3" />
                Create
              </button>
            )}
          </div>

          {sorted.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <KeyRound className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-sm text-muted-foreground">
                No secrets match &ldquo;{searchQuery}&rdquo;
              </p>
            </div>
          ) : (
            <SecretsListView
              items={sorted}
              sortField={sortField}
              sortDir={sortDir}
              onSort={handleSort}
              canDelete={canDelete === true}
              onSecretClick={(s) =>
                navigate(`/secrets/${encodeURIComponent(s.namespace)}/${encodeURIComponent(s.name)}`)
              }
              onDeleteClick={(s) => setDeleteTarget({ name: s.name, namespace: s.namespace })}
            />
          )}
        </>
      )}

      {/* Create Secret Dialog — key forces remount when global project changes */}
      <CreateSecretDialog
        key={selectedProject || "__all__"}
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
      />

      {/* Delete Secret Dialog */}
      {deleteTarget && selectedProject && (
        <DeleteSecretDialog
          open={!!deleteTarget}
          onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
          secretName={deleteTarget.name}
          secretNamespace={deleteTarget.namespace}
          navigateOnDelete={false}
        />
      )}
    </section>
  );
}

/* ---------- List view (matches CatalogListView / InstancesListView pattern) ---------- */

interface SecretsListViewProps {
  items: Secret[];
  sortField: SortField;
  sortDir: SortDir;
  onSort: (field: SortField) => void;
  canDelete: boolean;
  onSecretClick: (secret: Secret) => void;
  onDeleteClick: (secret: Secret) => void;
}

function SecretsListView({
  items,
  sortField,
  sortDir,
  onSort,
  canDelete,
  onSecretClick,
  onDeleteClick,
}: SecretsListViewProps) {
  return (
    <div className="rounded-lg border border-border overflow-hidden animate-fade-in-up">
      <Table className="table-fixed">
        <TableHeader>
          <TableRow>
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[30%]">Name</SortableHead>
            <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[20%]">Namespace</SortableHead>
            <SortableHead field="keys" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[30%]">Keys</SortableHead>
            <SortableHead field="createdAt" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[15%]">Created</SortableHead>
            {canDelete && <th className="w-[5%]" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((secret) => (
            <TableRow
              key={`${secret.namespace}/${secret.name}`}
              className="cursor-pointer"
              onClick={() => onSecretClick(secret)}
              role="button"
              tabIndex={0}
              aria-label={`View details for ${secret.name}`}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onSecretClick(secret);
                }
              }}
            >
              <TableCell className="font-medium text-foreground truncate">{secret.name}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground truncate">{secret.namespace}</TableCell>
              <TableCell className="text-sm text-muted-foreground truncate">
                {secret.keys.length > 0 ? secret.keys.join(", ") : "—"}
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {formatDistanceToNow(secret.createdAt)}
              </TableCell>
              {canDelete && (
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon"
                    className={cn("h-7 w-7 text-muted-foreground hover:text-destructive")}
                    aria-label={`Delete ${secret.name}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      onDeleteClick(secret);
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

/* ---------- Shared sub-components ---------- */

function ListSkeleton() {
  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <th className="pl-4 w-[30%] p-3"><Skeleton className="h-4 w-12" /></th>
            <th className="w-[20%] p-3"><Skeleton className="h-4 w-16" /></th>
            <th className="w-[25%] p-3"><Skeleton className="h-4 w-10" /></th>
            <th className="w-[20%] p-3 text-right"><Skeleton className="h-4 w-14 ml-auto" /></th>
          </TableRow>
        </TableHeader>
        <TableBody>
          {Array.from({ length: 5 }).map((_, i) => (
            <TableRow key={i}>
              <TableCell className="pl-4">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-8 w-8 rounded-md" />
                  <Skeleton className="h-4 w-32" />
                </div>
              </TableCell>
              <TableCell><Skeleton className="h-4 w-24" /></TableCell>
              <TableCell><Skeleton className="h-4 w-40" /></TableCell>
              <TableCell className="text-right"><Skeleton className="h-4 w-20 ml-auto" /></TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
