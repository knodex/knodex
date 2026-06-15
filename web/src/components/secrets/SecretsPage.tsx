// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ExternalLink, KeyRound, Plus, RefreshCw, Search, ShieldAlert, Trash2, X } from "@/lib/icons";
import { useSecretList } from "@/hooks/useSecrets";
import { useCanI } from "@/hooks/useCanI";
import { formatDistanceToNow } from "@/lib/date";
import { getSafeErrorMessage } from "@/lib/errors";
import { expiryHint, statusBadgeClasses, statusLabel } from "@/lib/secret-metadata";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { ListTableHeader, ListTableShell } from "@/components/ui/list-table";
import { ListFooter } from "@/components/ui/list-footer";
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
import type { Secret, SecretRotation, SecretStatus } from "@/types/secret";

type SortField = "name" | "namespace" | "keys" | "rotation" | "status" | "updatedAt";
type SortDir = "asc" | "desc";

/**
 * Rank used to sort the Status column. We sort by *expiry* rather than
 * by status enum so within a category (e.g. expiring-soon) rows order by
 * how close they are to expiring. Secrets without an expiration date
 * sort last in both directions — they are absence, not a value.
 */
function statusSortKey(secret: Secret): number {
  const ts = secret.metadata?.expiresAt;
  if (!ts) return Number.POSITIVE_INFINITY;
  const t = Date.parse(ts);
  return Number.isNaN(t) ? Number.POSITIVE_INFINITY : t;
}

/**
 * Updated timestamp falls back to createdAt when the wire payload omits
 * updatedAt — the production Secret type marks `updatedAt` optional, and
 * a brand-new secret has no separate update.
 */
function getUpdatedTimestamp(secret: Secret): string {
  return secret.updatedAt && secret.updatedAt.length > 0 ? secret.updatedAt : secret.createdAt;
}

export function SecretsPage() {
  const navigate = useNavigate();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ name: string; namespace: string } | null>(null);
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [searchQuery, setSearchQuery] = useState("");

  // Secrets are namespace-keyed under the unified Casbin model — the
  // currently-selected project is only an audit lens, not an access input.
  // Page-level can-i checks use the wildcard "-" object slot (existence
  // probe); per-row delete checks pass each row's namespace.
  const { allowed: canGet, isLoading: permLoading } = useCanI("secrets", "get", "-");
  const { allowed: canCreate } = useCanI("secrets", "create", "-");
  const { allowed: canDelete } = useCanI("secrets", "delete", "-");

  // Single namespace-agnostic list. The server filters to the user's
  // accessible namespaces; the project lens (top-right selector) no
  // longer participates in the request.
  const { data, isLoading, isError, error, refetch } = useSecretList();

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
        case "rotation":
          // Empty rotation sorts after "auto"/"manual" in asc, before in desc.
          // Using the raw string is enough since the enum is alphabetically
          // ordered the way operators expect (auto, manual).
          aVal = a.metadata?.rotation ?? "￿";
          bVal = b.metadata?.rotation ?? "￿";
          break;
        case "status":
          aVal = statusSortKey(a);
          bVal = statusSortKey(b);
          break;
        case "updatedAt":
          aVal = getUpdatedTimestamp(a);
          bVal = getUpdatedTimestamp(b);
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortDir === "asc" ? -1 : 1;
      if (aVal > bVal) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
  }, [data?.items, sortField, sortDir, searchQuery]);

  // Footer summary computed over the visible (search-filtered) `sorted` array so it
  // matches the rows on screen (mirrors the 48.2/48.3 ListFooter precedent).
  // NOTE: the prototype's `vault-backed` / `auto-rotated` breakdown fields don't
  // exist in the production Secret model (no provider/rotation metadata) and we
  // do NOT fabricate them. We also don't count "namespaces" — every secret
  // lives in exactly one namespace so a distinct-namespace count is a
  // misleading aggregate. `keys` (sum of secret.keys.length) is the only
  // honest per-secret rollup the wire data carries.
  const totalKeys = useMemo(
    () => sorted.reduce((sum, s) => sum + s.keys.length, 0),
    [sorted]
  );

  // Access denied state — user has zero accessible namespaces for secrets.
  if (!permLoading && canGet === false) {
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
      {/* Loading */}
      {!doneLoading ? (
        <ListSkeleton canDelete={canDelete === true} />
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
            <>
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
              <div data-testid="secrets-list-footer">
                <ListFooter
                  total={sorted.length}
                  totalLabel="secrets"
                  breakdown={[
                    ["keys", totalKeys],
                  ]}
                />
              </div>
            </>
          )}
        </>
      )}

      {/* Create Secret Dialog — namespace is picked inside the dialog now */}
      <CreateSecretDialog
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
      />

      {/* Delete Secret Dialog */}
      {deleteTarget && (
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
    <ListTableShell>
      <Table className="table-fixed">
        <ListTableHeader>
          <TableRow>
            {/* Leading icon column — visual key glyph next to every row.
                Header has no label (decorative), but a hidden one is wired
                via aria for screen readers. */}
            <th className="w-12" aria-hidden="true" />
            <SortableHead field="name" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[22%]">Name</SortableHead>
            <SortableHead field="namespace" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[15%]">Namespace</SortableHead>
            <SortableHead field="keys" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[22%]">Keys</SortableHead>
            <SortableHead field="rotation" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[10%]">Rotation</SortableHead>
            <SortableHead field="status" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[16%]">Status</SortableHead>
            <SortableHead field="updatedAt" sortField={sortField} sortDir={sortDir} onSort={onSort} className="w-[10%]">Updated</SortableHead>
            {canDelete && <th className="w-[5%]" />}
          </TableRow>
        </ListTableHeader>
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
              <TableCell className="text-[var(--brand-primary)]">
                <KeyRound className="h-4 w-4" aria-hidden="true" />
              </TableCell>
              <TableCell className="font-medium text-foreground truncate">
                <span className="inline-flex items-center gap-1.5">
                  <span className="truncate">{secret.name}</span>
                  {secret.metadata?.docsUrl && (
                    <a
                      href={secret.metadata.docsUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="text-muted-foreground hover:text-foreground shrink-0"
                      aria-label={`Documentation for ${secret.name}`}
                      title="Open documentation"
                      data-testid={`secret-docs-link-${secret.name}`}
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                    </a>
                  )}
                </span>
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground truncate">{secret.namespace}</TableCell>
              <TableCell className="text-sm text-muted-foreground truncate">
                {secret.keys.length > 0 ? secret.keys.join(", ") : "—"}
              </TableCell>
              <TableCell data-testid={`secret-rotation-${secret.name}`}>
                <RotationChip rotation={secret.metadata?.rotation} />
              </TableCell>
              <TableCell data-testid={`secret-status-${secret.name}`}>
                <StatusBadge
                  status={secret.status}
                  expiresAt={secret.metadata?.expiresAt}
                />
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {formatDistanceToNow(getUpdatedTimestamp(secret))}
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
    </ListTableShell>
  );
}

/* ---------- Shared sub-components ---------- */

/**
 * Compact chip showing the rotation policy. Renders an em-dash when no
 * policy has been declared so the column reads consistently.
 */
function RotationChip({ rotation }: { rotation: SecretRotation | undefined }) {
  if (!rotation) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-foreground/80">
      <RefreshCw className="h-3 w-3" aria-hidden="true" />
      {rotation === "auto" ? "Auto" : "Manual"}
    </span>
  );
}

/**
 * Color-coded status badge driven by the server-computed `status` field.
 * Falls back to an em-dash when no expiration date is set. The expiry
 * date (when present) is shown as a hint below the badge so operators
 * see how close to expiry without having to open the detail view.
 */
function StatusBadge({ status, expiresAt }: { status: SecretStatus | undefined; expiresAt: string | undefined }) {
  if (!status) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  return (
    <div className="flex flex-col items-start gap-0.5">
      <span
        className={cn(
          "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
          statusBadgeClasses(status),
        )}
        title={expiryHint(expiresAt)}
      >
        {statusLabel(status)}
      </span>
      {expiresAt && (
        <span className="text-[10px] text-muted-foreground">
          {expiryHint(expiresAt)}
        </span>
      )}
    </div>
  );
}

function ListSkeleton({ canDelete }: { canDelete: boolean }) {
  // Column widths + count mirror the real header in SecretsListView (see line
  // ~360 in this file): leading icon col + Name/Namespace/Keys/Rotation/
  // Status/Updated, then an optional Actions col gated on canDelete. Keeping
  // them aligned avoids the layout shift (column-width jump + missing actions
  // pop-in) the previous version had when the viewer had delete perms.
  return (
    <ListTableShell noAnimation>
      <Table>
        <ListTableHeader>
          <TableRow>
            <th className="w-12 p-3" aria-hidden="true" />
            <th className="w-[22%] p-3"><Skeleton className="h-4 w-12" /></th>
            <th className="w-[15%] p-3"><Skeleton className="h-4 w-16" /></th>
            <th className="w-[22%] p-3"><Skeleton className="h-4 w-10" /></th>
            <th className="w-[10%] p-3"><Skeleton className="h-4 w-12" /></th>
            <th className="w-[16%] p-3"><Skeleton className="h-4 w-16" /></th>
            <th className="w-[10%] p-3"><Skeleton className="h-4 w-14" /></th>
            {canDelete && <th className="w-[5%] p-3" aria-hidden="true" />}
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {Array.from({ length: 5 }).map((_, i) => (
            <TableRow key={i}>
              <TableCell>
                <Skeleton className="h-4 w-4 rounded" />
              </TableCell>
              <TableCell><Skeleton className="h-4 w-32" /></TableCell>
              <TableCell><Skeleton className="h-4 w-24" /></TableCell>
              <TableCell><Skeleton className="h-4 w-40" /></TableCell>
              <TableCell><Skeleton className="h-4 w-14 rounded-full" /></TableCell>
              <TableCell><Skeleton className="h-4 w-20 rounded-full" /></TableCell>
              <TableCell><Skeleton className="h-4 w-16" /></TableCell>
              {canDelete && (
                <TableCell>
                  <Skeleton className="h-4 w-4 rounded ml-auto" />
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </ListTableShell>
  );
}
