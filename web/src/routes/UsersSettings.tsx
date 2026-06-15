// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * UsersSettings — Settings → Users roster page (Story 16.1 / UM-1, extended by
 * Story 16.3 / UM-3 with an inactive badge + client-side search/filters).
 *
 * Lists the canonical `identity.users` roster (who is consuming seats) via the
 * frozen 15.8 Users API (`GET /api/v1/users`, keyset-paginated). This is a
 * CONSUMER-only page: no mutations here. The reclaim action + seat-usage widget
 * land in 16.2.
 *
 * Search and filters are CLIENT-SIDE over the already-loaded pages
 * (`data.pages.flatMap(p => p.users)`) — `useUsers` is a keyset
 * `useInfiniteQuery`, so the page only holds the pages the operator has loaded
 * via "Load more", NOT the whole server roster. The API is frozen and serves no
 * search/state query params (same consumer discipline as 16.1). The `ListFooter`
 * therefore counts the FILTERED set, not the raw loaded set.
 *
 * The `isInactive` flag is display-only: it is derived server-side from
 * last-seen activity (`IDENTITY_INACTIVE_THRESHOLD_DAYS`, default 30) and does
 * NOT affect billing/seat count — entitlement is `state`-based (FR-U7). The
 * inactive badge tooltip says exactly that.
 *
 * OSS/EE parity: the roster API exists on every edition (Postgres mandatory,
 * R5-5), so nothing on this page is gated on `isEnterprise()`.
 *
 * Operator-gating is enforced by the server (`settings/* get`). A non-operator
 * gets a 403 on the list call, which drives the Access Denied state (Pattern B,
 * copied from SSOSettings.tsx / LicenseSettings.tsx).
 */

import { useMemo, useState } from "react";
import { AxiosError } from "axios";
import { toast } from "sonner";
import {
  Users,
  ShieldAlert,
  Loader2,
  Search,
  X,
  AlertTriangle,
  RotateCcw,
} from "@/lib/icons";
import { ApiError } from "@/api/client";
import { useUsers, useReclaimUser } from "@/hooks/useUsers";
import { useLicenseStatus } from "@/hooks/useLicense";
import { useCanI } from "@/hooks/useCanI";
import { formatDate } from "@/lib/date";
import { SeatUsageWidget } from "@/components/settings/seat-usage";
import { PageShell } from "@/components/layout/PageShell";
import { UserAvatar } from "@/components/ui/user-avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ListFooter } from "@/components/ui/list-footer";
import { FiltersDropdown } from "@/components/ui/filters-dropdown";
import { FilterChip } from "@/components/ui/filter-chip";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
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
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { User } from "@/types/user";

type StateFilter = "all" | "active" | "removed";

/** Distinct issuer strings for a user (a user may have multiple identities). */
function distinctIssuers(user: User): string[] {
  return Array.from(
    new Set(user.federatedIdentities.map((fi) => fi.issuer).filter(Boolean)),
  );
}

/**
 * StateBadge — a calm dot+label status (mirrors the cloud roster treatment):
 * an active user gets a green dot + "Active"; a removed user gets a muted dot +
 * "Removed". Keeps the `user-state-badge` test id + verbatim label text.
 */
function StateBadge({ state }: { state: User["state"] }) {
  if (state === "removed") {
    return (
      <span
        data-testid="user-state-badge"
        className="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium text-muted-foreground"
      >
        <span
          className="h-1.5 w-1.5 rounded-full bg-muted-foreground/50"
          aria-hidden="true"
        />
        Removed
      </span>
    );
  }
  return (
    <span
      data-testid="user-state-badge"
      className="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-400"
    >
      <span
        className="h-1.5 w-1.5 rounded-full bg-emerald-500"
        aria-hidden="true"
      />
      Active
    </span>
  );
}

/**
 * ApplicationRoleBadge — read-only display of a user's effective application
 * role (Story 17.3 / Epic 17). Path A: there is deliberately NO control to
 * change a user's application role here — it is set at the IdP / group-mapping.
 * `serveradmin` gets a non-destructive accent (default) variant; everything
 * else (`member`, a future `auditor`) renders muted (outline).
 */
function ApplicationRoleBadge({ role }: { role: User["applicationRole"] }) {
  if (role === "serveradmin") {
    return (
      <Badge variant="default" data-testid="user-app-role-badge">
        Server admin
      </Badge>
    );
  }
  return (
    <Badge
      variant="outline"
      className="text-muted-foreground"
      data-testid="user-app-role-badge"
    >
      {role === "member" ? "Member" : role}
    </Badge>
  );
}

/**
 * InactiveBadge — display-only "Inactive" flag for an idle account. The tooltip
 * states plainly that this does NOT affect billing/seat count (entitlement is
 * `state`-based, not activity-based). A `TooltipProvider` is mounted app-wide
 * (App.tsx), so we only use Tooltip/Trigger/Content here. The trigger is a
 * native <button> (interactive + focusable, no jsx-a11y violation, forwards
 * refs cleanly for Radix); the Badge renders inside it.
 */
function InactiveBadge() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className="inline-flex cursor-default rounded-md outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/40"
          data-testid="user-inactive-badge"
        >
          <Badge
            variant="outline"
            className="border-status-inactive/40 text-status-inactive"
          >
            Inactive
          </Badge>
        </button>
      </TooltipTrigger>
      <TooltipContent data-testid="user-inactive-tooltip" className="max-w-xs">
        Inactive: not seen in 30+ days. Display-only — does not affect billing or
        seat count.
      </TooltipContent>
    </Tooltip>
  );
}

function IssuerChips({ user }: { user: User }) {
  const issuers = distinctIssuers(user);
  if (issuers.length === 0) {
    return <span className="text-xs text-muted-foreground italic">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {issuers.map((issuer) => (
        <span
          key={issuer}
          className="inline-flex items-center rounded-md border border-border/60 bg-muted/30 px-2 py-0.5 text-xs font-mono text-foreground"
          data-testid="user-issuer-chip"
        >
          {issuer}
        </span>
      ))}
    </div>
  );
}

interface ReclaimSeatDialogProps {
  user: User;
  isOpen: boolean;
  isPending: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * ReclaimSeatDialog — confirm a seat reclaim (Story 16.2 / UM-2 AC #3).
 *
 * This is a REVERSIBLE seat reclaim, NOT a destructive delete. The body carries
 * the verbatim reclaim + IdP-revocation note ("Permanent exclusion requires
 * IdP-side revocation.") and states the account reappears on the next SSO login.
 * Deliberately, to avoid over-warning / mis-framing it as a hard delete:
 *  - a plain confirm (no type-to-confirm Input, unlike DeleteRepositoryDialog);
 *  - an amber AlertTriangle puck, NOT destructive-red;
 *  - the action button is `variant="outline"`, NOT `variant="destructive"`;
 *  - no "delete" / "permanently" / "cannot be undone" wording anywhere.
 */
function ReclaimSeatDialog({
  user,
  isOpen,
  isPending,
  onConfirm,
  onCancel,
}: ReclaimSeatDialogProps) {
  const handleOpenChange = (open: boolean) => {
    if (!open && !isPending) onCancel();
  };

  return (
    <AlertDialog open={isOpen} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="max-w-md" data-testid="reclaim-seat-dialog">
        <AlertDialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-amber-500/10">
              <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
            </div>
            <AlertDialogTitle>Reclaim seat</AlertDialogTitle>
          </div>
          <AlertDialogDescription className="sr-only">
            Reclaim the licensed seat held by {user.email}.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div
          className="space-y-3 text-sm text-muted-foreground"
          data-testid="reclaim-seat-body"
        >
          <p>
            Reclaim the seat held by{" "}
            <strong className="text-foreground">{user.email}</strong>? This frees
            the licensed seat now, but the account reappears on their next SSO
            login. Permanent exclusion requires IdP-side revocation.
          </p>
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
          <Button
            variant="outline"
            onClick={onConfirm}
            disabled={isPending}
            data-testid="reclaim-seat-confirm"
          >
            {isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <RotateCcw className="mr-2 h-4 w-4" />
            )}
            Reclaim seat
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function UsersSettings() {
  const {
    data,
    isLoading,
    error,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useUsers();

  // 16.3 client-side search/filter state.
  const [searchQuery, setSearchQuery] = useState("");
  const [stateFilter, setStateFilter] = useState<StateFilter>("all");
  const [inactiveOnly, setInactiveOnly] = useState(false);

  // 16.2 seat widget + reclaim. ALL hooks sit above the first conditional return
  // (rules-of-hooks — 16.1/16.3 both hit this). The license query is
  // `enabled: isEnterprise()`, so on OSS it never fetches and `seats` stays
  // undefined → the widget self-hides. A license error/403 must NOT blank the
  // roster, so we deliberately do NOT gate the page on this query.
  const { data: licenseStatus } = useLicenseStatus();
  // UX-only gate (the server still enforces settings/* update). Show the action
  // when explicitly allowed OR the can-i check errored; hide only on explicit
  // false (mirror LicenseSettings.tsx).
  const { allowed: canReclaimRaw, isError: canReclaimError } = useCanI(
    "settings",
    "update",
  );
  const canReclaim = canReclaimRaw === true || canReclaimError;
  const reclaimMutation = useReclaimUser();
  // Track the dialog target by user object (NOT by index — rows re-sort/filter).
  const [reclaimTarget, setReclaimTarget] = useState<User | null>(null);

  // All hooks must run before any conditional return (rules-of-hooks).
  // apiClient's interceptor rejects with an ApiError (exposes `.status`); fall
  // back to the raw axios shape (`.response.status`) so unit tests that mock the
  // axios error also resolve. Checking only `.response.status` misses the real
  // runtime error and the Access-Denied state never renders (caught by E2E).
  const is403Error = useMemo(() => {
    if (!error) return false;
    const status =
      (error as ApiError)?.status ?? (error as AxiosError)?.response?.status;
    return status === 403;
  }, [error]);
  const users = useMemo(
    () => data?.pages.flatMap((page) => page.users) ?? [],
    [data],
  );

  // Client-side filter over the already-loaded pages (NOT a server query).
  // Order: search (email|displayName substring) → state → inactive-only.
  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return users.filter((u) => {
      if (q) {
        // AC #2: substring match on email OR displayName (each field tested
        // independently — never the concatenation, which would let a query
        // straddling the field boundary produce a spurious match).
        const email = u.email.toLowerCase();
        const name = (u.displayName ?? "").toLowerCase();
        if (!email.includes(q) && !name.includes(q)) return false;
      }
      if (stateFilter !== "all" && u.state !== stateFilter) return false;
      if (inactiveOnly && !u.isInactive) return false;
      return true;
    });
  }, [users, searchQuery, stateFilter, inactiveOnly]);

  // Footer counts reflect the FILTERED set (AC #4), not the raw loaded set.
  const filteredActiveCount = useMemo(
    () => filtered.filter((u) => u.state === "active").length,
    [filtered],
  );

  // Active non-search filters drive the FiltersDropdown count badge.
  const activeFilterCount =
    (stateFilter !== "all" ? 1 : 0) + (inactiveOnly ? 1 : 0);

  // Confirm → reclaim the seat. A 404 (already removed) is resolved as success
  // inside useReclaimUser, so it never reaches this catch (no error-toast storm).
  // Success copy uses reclaim framing — never "deleted".
  const handleReclaim = async () => {
    if (!reclaimTarget) return;
    const email = reclaimTarget.email;
    try {
      await reclaimMutation.mutateAsync(reclaimTarget.id);
      toast.success(`Seat reclaimed — ${email} reappears on next SSO login.`);
      setReclaimTarget(null);
    } catch (err) {
      const message =
        err instanceof AxiosError
          ? err.response?.data?.message || err.message
          : err instanceof Error
            ? err.message
            : "Failed to reclaim seat";
      toast.error(message);
    }
  };

  // --- 403 Access Denied (Pattern B — copied from SSOSettings.tsx) ---
  if (is403Error) {
    return (
      <div data-testid="users-access-denied">
        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to view the user roster.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // --- Loading skeleton ---
  if (isLoading) {
    return (
      <div className="space-y-3" data-testid="users-loading">
        <Skeleton className="h-10 w-full rounded-md" />
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-md" />
        ))}
      </div>
    );
  }

  // --- Non-403 error ---
  if (error) {
    const status =
      error instanceof AxiosError ? error.response?.status : undefined;
    const message =
      error instanceof AxiosError
        ? error.response?.data?.message || error.message
        : error instanceof Error
          ? error.message
          : "Unknown error";
    return (
      <Card>
        <CardContent className="pt-6" data-testid="users-error">
          <p className="text-sm text-destructive">
            Failed to load users{status ? ` (HTTP ${status})` : ""}: {message}
          </p>
        </CardContent>
      </Card>
    );
  }

  // --- Empty roster (genuinely no users — distinct from the "no matches"
  //     filtered-empty state below) ---
  if (users.length === 0) {
    return (
      <Card data-testid="users-empty-state">
        <CardContent className="flex flex-col items-center justify-center gap-4 py-20 text-center">
          <div
            className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20"
            aria-hidden
          >
            <Users className="h-8 w-8" />
          </div>
          <div className="space-y-1.5 max-w-md">
            <h3 className="text-lg font-semibold">No users yet</h3>
            <p className="text-sm text-muted-foreground">
              The roster is populated as users sign in via SSO. Once someone logs
              in, they'll appear here.
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  // --- Search + filters toolbar (the slot 16.1 reserved for this story) ---
  const toolbar = (
    <div className="flex items-center gap-2">
      <div className="relative flex-1 min-w-[260px]">
        <Search className={filterSearchIconClasses} />
        <Input
          type="text"
          placeholder="Filter users…"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className={filterSearchClasses}
          aria-label="Search users"
          data-testid="users-search"
        />
        {searchQuery && (
          <button
            type="button"
            onClick={() => setSearchQuery("")}
            className={filterClearButtonClasses}
            aria-label="Clear search"
            data-testid="users-search-clear"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      <FiltersDropdown activeCount={activeFilterCount}>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label
              className="text-xs font-medium text-muted-foreground"
              htmlFor="users-state-filter"
            >
              State
            </label>
            <Select
              value={stateFilter}
              onValueChange={(v) => setStateFilter(v as StateFilter)}
            >
              <SelectTrigger
                id="users-state-filter"
                className="h-8 w-full text-xs"
                aria-label="Filter by state"
                data-testid="users-state-filter"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="removed">Removed</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <FilterChip
            state={inactiveOnly ? "active" : "idle"}
            showDot={inactiveOnly}
            aria-pressed={inactiveOnly}
            onClick={() => setInactiveOnly((v) => !v)}
            data-testid="users-inactive-filter"
          >
            Inactive only
          </FilterChip>
        </div>
      </FiltersDropdown>

      {/* Seat usage (EE only — `seats` is absent on OSS, so this self-hides).
          Sits at the toolbar's right edge as "{used} / {allowed} seats used",
          NOT inside the cloud roster (control-plane-enforced there). */}
      {licenseStatus?.seats && (
        <div
          className="ml-auto flex shrink-0 items-center pl-2"
          data-testid="users-seat-usage-header"
        >
          <SeatUsageWidget
            seats={licenseStatus.seats}
            maxUsers={licenseStatus.license?.maxUsers}
          />
        </div>
      )}
    </div>
  );

  // --- Roster table (filtered) ---
  // Client-side filtering only narrows the already-loaded keyset pages, so the
  // Load-more affordance is offered in BOTH the populated and no-matches states
  // (a search can exclude every loaded row while more server pages remain).
  const loadMore = hasNextPage ? (
    <div className="flex flex-col items-center gap-1.5">
      <Button
        variant="outline"
        onClick={() => fetchNextPage()}
        disabled={isFetchingNextPage}
        data-testid="users-load-more"
      >
        {isFetchingNextPage && (
          <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
        )}
        Load more
      </Button>
      <p className="text-xs text-muted-foreground">
        Filtering loaded users — Load more to search the full roster.
      </p>
    </div>
  ) : null;

  return (
    <div className="space-y-4" data-testid="users-settings">
      <PageShell toolbar={toolbar} />

      {reclaimTarget && (
        <ReclaimSeatDialog
          user={reclaimTarget}
          isOpen
          isPending={reclaimMutation.isPending}
          onConfirm={handleReclaim}
          onCancel={() => setReclaimTarget(null)}
        />
      )}

      {filtered.length === 0 ? (
        <>
          <Card data-testid="users-no-matches">
            <CardContent className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Users className="h-10 w-10 text-muted-foreground opacity-50" />
              <p className="text-sm text-muted-foreground">
                No users match your search or filters.
              </p>
            </CardContent>
          </Card>
          {loadMore}
        </>
      ) : (
        <>
          <div
            className="animate-fade-in-up divide-y divide-border overflow-hidden rounded-lg border border-border"
            data-testid="users-list"
          >
            {filtered.map((user) => {
              const primary = user.displayName || user.email;
              return (
                <div
                  key={user.id}
                  data-testid={`user-row-${user.id}`}
                  className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/30"
                >
                  <UserAvatar
                    name={user.displayName}
                    email={user.email}
                    className="h-10 w-10 text-sm"
                  />

                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <span className="truncate text-sm font-medium text-foreground">
                        {primary}
                      </span>
                      <StateBadge state={user.state} />
                      {user.isInactive && <InactiveBadge />}
                      <ApplicationRoleBadge role={user.applicationRole} />
                    </div>
                    <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1">
                      {/* Only show the email subline when the primary line is a
                          name (otherwise the primary line already IS the email). */}
                      {user.displayName && (
                        <span className="truncate text-xs text-muted-foreground">
                          {user.email}
                        </span>
                      )}
                      <IssuerChips user={user} />
                    </div>
                  </div>

                  <div className="hidden shrink-0 flex-col items-end gap-0.5 whitespace-nowrap text-xs text-muted-foreground sm:flex">
                    <span>Joined {formatDate(user.firstSeenAt)}</span>
                    <span>Last seen {formatDate(user.lastSeenAt)}</span>
                  </div>

                  <div className="flex w-9 justify-end">
                    {/* Reclaim is shown on ACTIVE rows only (removed rows have
                        already reclaimed the seat) and only when the operator
                        holds settings/* update (UX gate; server still enforces).
                        Reversible seat reclaim — NOT a destructive delete. */}
                    {user.state === "active" && canReclaim ? (
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                            onClick={() => setReclaimTarget(user)}
                            aria-label={`Reclaim seat for ${user.email}`}
                            data-testid={`reclaim-seat-${user.id}`}
                          >
                            <RotateCcw className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Reclaim seat</TooltipContent>
                      </Tooltip>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>

          {loadMore}

          <ListFooter
            total={filtered.length}
            totalLabel="users"
            breakdown={[["active", filteredActiveCount]]}
            data-testid="users-list-footer"
          />
        </>
      )}
    </div>
  );
}

export default UsersSettings;
