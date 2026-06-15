// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * TeamsSettings — OSS Teams management page (Story 10.4).
 *
 * Lists Teams, and lets operators create/edit/delete them and manage each
 * Team's OIDC groups via the GroupTypeahead (observed-groups suggestions with a
 * raw free-text fallback). Mutations are gated in the UI by
 * `useCanI("settings","update")` so non-operators see disabled controls — the
 * server enforces regardless (single Casbin layer, NFR-T1).
 *
 * It ships in every build and owns `/settings/teams` — the self-hosted,
 * federated Teams model (a team references existing external-IdP groups).
 */
import { useState, useCallback, useMemo } from "react";
import {
  Plus,
  Trash2,
  Loader2,
  Users,
  Search,
  Settings2,
} from "@/lib/icons";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { useCanI } from "@/hooks/useCanI";
import {
  useTeams,
  useCreateTeam,
  useUpdateTeam,
  useDeleteTeam,
  useObservedGroups,
} from "@/hooks/useTeams";
import { TeamDrawer } from "@/components/teams/TeamDrawer";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { Team, TeamFormValues } from "@/types/team";

// --- Group chip visual identity ---
//
// A team binds opaque OIDC group strings. Many real-world groups follow a
// `provider/group` convention (e.g. `okta/platform-eng`) and Keycloak federated
// groups bring their own (`keycloak/<realm-group>`); the rest are bare names
// (`alpha-developers`, an Azure AD object UUID, …). We don't ship vendor logos
// — instead each chip gets a small colored dot whose hue is pinned for
// well-known providers (so an Okta group always reads as Okta) and otherwise
// hashed off the string so the same group renders the same color across
// sessions. Loosely mirrors the SSO providers list (`SSOSettings.tsx`).
const GROUP_PALETTE: ReadonlyArray<{ dot: string; label: string }> = [
  { dot: "bg-teal-400", label: "text-teal-300" },
  { dot: "bg-violet-400", label: "text-violet-300" },
  { dot: "bg-blue-400", label: "text-blue-300" },
  { dot: "bg-amber-400", label: "text-amber-300" },
  { dot: "bg-rose-400", label: "text-rose-300" },
  { dot: "bg-emerald-400", label: "text-emerald-300" },
];

const WELL_KNOWN_GROUP_TONES: Record<string, (typeof GROUP_PALETTE)[number]> = {
  google: GROUP_PALETTE[2],
  github: GROUP_PALETTE[0],
  gitlab: GROUP_PALETTE[3],
  okta: GROUP_PALETTE[1],
  azure: GROUP_PALETTE[2],
  microsoft: GROUP_PALETTE[2],
  entra: GROUP_PALETTE[2],
  auth0: GROUP_PALETTE[4],
  keycloak: GROUP_PALETTE[5],
  supabase: GROUP_PALETTE[5],
};

interface ParsedGroup {
  /** Optional provider/realm label (everything before the first `/`). */
  provider: string | null;
  /** The group identifier shown in mono — provider stripped. */
  name: string;
  tone: (typeof GROUP_PALETTE)[number];
}

function hashTone(key: string): (typeof GROUP_PALETTE)[number] {
  // FNV-1a-ish so the same input always picks the same color across sessions.
  let h = 2166136261;
  for (let i = 0; i < key.length; i++) {
    h ^= key.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return GROUP_PALETTE[(h >>> 0) % GROUP_PALETTE.length];
}

function parseGroup(group: string): ParsedGroup {
  const slash = group.indexOf("/");
  if (slash > 0 && slash < group.length - 1) {
    const provider = group.slice(0, slash);
    const name = group.slice(slash + 1);
    const tone =
      WELL_KNOWN_GROUP_TONES[provider.toLowerCase()] ?? hashTone(provider.toLowerCase());
    return { provider, name, tone };
  }
  return { provider: null, name: group, tone: hashTone(group.toLowerCase()) };
}

function GroupChip({ group }: { group: string }) {
  const { provider, name, tone } = parseGroup(group);
  return (
    <div className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-muted/30 px-2 py-1 text-xs">
      <span
        className={cn("h-1.5 w-1.5 rounded-full shrink-0", tone.dot)}
        aria-hidden
      />
      {provider && (
        <>
          <span className={cn("font-medium", tone.label)}>{provider}</span>
          <span className="text-muted-foreground/50">/</span>
        </>
      )}
      <span className="font-mono text-foreground">{name}</span>
    </div>
  );
}

// Case-insensitive substring match across team name, description, and any
// bound group string — operators commonly hunt by group name when looking for
// "which team owns this Okta group", so the group list is searchable too.
function matchesSearch(team: Team, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  if (team.name.toLowerCase().includes(needle)) return true;
  if (team.description?.toLowerCase().includes(needle)) return true;
  return team.oidcGroups.some((g) => g.toLowerCase().includes(needle));
}

export function TeamsSettings() {
  const { allowed: canUpdate } = useCanI("settings", "update");
  const editable = canUpdate === true;

  const { data, isLoading, error } = useTeams();
  const { data: observed } = useObservedGroups();
  const createMutation = useCreateTeam();
  const updateMutation = useUpdateTeam();
  const deleteMutation = useDeleteTeam();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerKey, setDrawerKey] = useState(0);
  const [editingTeam, setEditingTeam] = useState<Team | null>(null);
  const [deletingTeam, setDeletingTeam] = useState<Team | null>(null);
  const [query, setQuery] = useState("");

  const teams = useMemo(() => data?.items ?? [], [data]);
  const suggestions = useMemo(() => observed?.groups ?? [], [observed]);
  const filteredTeams = useMemo(
    () => teams.filter((t) => matchesSearch(t, query.trim())),
    [teams, query]
  );
  const saving = createMutation.isPending || updateMutation.isPending;

  const openCreate = useCallback(() => {
    setEditingTeam(null);
    setDrawerKey((k) => k + 1);
    setDrawerOpen(true);
  }, []);

  const openEdit = useCallback((team: Team) => {
    setEditingTeam(team);
    setDrawerKey((k) => k + 1);
    setDrawerOpen(true);
  }, []);

  const closeForm = useCallback(() => {
    setDrawerOpen(false);
    setEditingTeam(null);
  }, []);

  const handleSubmit = useCallback(
    async (values: TeamFormValues) => {
      try {
        if (!editingTeam) {
          await createMutation.mutateAsync({
            name: values.name,
            description: values.description || undefined,
            oidcGroups: values.oidcGroups,
          });
          toast.success(`Team "${values.name}" created`);
        } else {
          await updateMutation.mutateAsync({
            name: editingTeam.name,
            request: {
              description: values.description || undefined,
              oidcGroups: values.oidcGroups,
            },
          });
          toast.success(`Team "${editingTeam.name}" updated`);
        }
        closeForm();
      } catch (err) {
        const msg =
          err instanceof AxiosError
            ? err.response?.data?.message || err.message
            : "Failed to save team";
        toast.error(msg);
      }
    },
    [editingTeam, createMutation, updateMutation, closeForm]
  );

  const handleDelete = useCallback(async () => {
    if (!deletingTeam) return;
    try {
      await deleteMutation.mutateAsync(deletingTeam.name);
      toast.success(`Team "${deletingTeam.name}" deleted`);
      setDeletingTeam(null);
    } catch (err) {
      const msg =
        err instanceof AxiosError
          ? err.response?.data?.message || err.message
          : "Failed to delete team";
      toast.error(msg);
    }
  }, [deletingTeam, deleteMutation]);

  // --- List view ---
  //
  // Layout matches the redesigned mockup: search input + primary "New team"
  // action sit above the team-card stack (no per-row wrapper to nest into).
  // Each card opens with an avatar tile + name/description + Manage/delete
  // cluster, then a hairline divider, then a sectioned OIDC-groups list with
  // provider-colored chips. The bare pencil icon was promoted to a labeled
  // "Manage" button so a non-trivial action gets a non-trivial affordance.
  const hasTeams = !isLoading && !error && teams.length > 0;
  const showingFiltered = hasTeams && query.trim().length > 0;

  return (
    <div className="space-y-6" data-testid="teams-settings">
      {hasTeams && (
        <div className="flex items-center gap-3">
          <div className="relative flex-1">
            <Search
              className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none"
              aria-hidden
            />
            <Input
              type="search"
              placeholder="Search teams"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9 h-10"
              aria-label="Search teams"
              data-testid="teams-search-input"
            />
          </div>
          <Button
            onClick={openCreate}
            disabled={!editable}
            data-testid="create-team-button"
            className="h-10"
          >
            <Plus className="h-4 w-4 mr-1.5" />
            New team
          </Button>
        </div>
      )}

      {isLoading ? (
        // Skeleton mirrors the redesigned card: avatar tile + name/description
        // + action cluster up top, hairline divider, then the OIDC groups
        // section with a label row + chips. Two cards is enough hint of stack;
        // matches dimensions closely so there's no layout shift on resolve.
        <div className="space-y-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="p-0">
                <div className="flex items-start gap-4 px-6 py-5">
                  <Skeleton className="h-12 w-12 rounded-xl shrink-0" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-5 w-44" />
                    <Skeleton className="h-3.5 w-72" />
                  </div>
                  <div className="flex items-center gap-2">
                    <Skeleton className="h-9 w-24 rounded-md" />
                    <Skeleton className="h-9 w-9 rounded-md" />
                  </div>
                </div>
                <div className="border-t border-border/60 px-6 py-4 space-y-3">
                  <Skeleton className="h-3 w-32" />
                  <div className="flex gap-2">
                    <Skeleton className="h-7 w-40 rounded-md" />
                    <Skeleton className="h-7 w-32 rounded-md" />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : error ? (
        <Card>
          <CardContent className="pt-6 space-y-1" data-testid="teams-error">
            {(() => {
              // Distinguish operator-gate denials (401/403) from server / network
              // errors (500, CRD missing, network down). The previous copy
              // ("You may not have operator access") was misleading for the
              // common dev-cluster case where the Team CRD just isn't installed,
              // which surfaces as a 500 — not a 403.
              const status =
                error instanceof AxiosError ? error.response?.status : undefined;
              const serverMsg =
                error instanceof AxiosError
                  ? typeof error.response?.data === "object" &&
                    error.response?.data !== null &&
                    "message" in error.response.data &&
                    typeof (error.response.data as { message: unknown })
                      .message === "string"
                    ? (error.response.data as { message: string }).message
                    : error.message
                  : error instanceof Error
                  ? error.message
                  : "Unknown error";

              if (status === 401 || status === 403) {
                return (
                  <p className="text-sm text-destructive">
                    Failed to load teams: you don't have <code>settings:get</code>{" "}
                    permission. The Teams page is operator-gated.
                  </p>
                );
              }
              return (
                <>
                  <p className="text-sm text-destructive">
                    Failed to load teams
                    {status ? ` (HTTP ${status})` : ""}: {serverMsg}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    If this is a fresh cluster, verify the Team CRD is installed:{" "}
                    <code>kubectl apply -f deploy/crds/team.yaml</code>.
                  </p>
                </>
              );
            })()}
          </CardContent>
        </Card>
      ) : teams.length === 0 ? (
        <Card data-testid="teams-empty-state">
          <CardContent className="flex flex-col items-center justify-center gap-5 py-20 text-center">
            <div
              className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20"
              aria-hidden
            >
              <Users className="h-8 w-8" />
            </div>
            <div className="space-y-1.5 max-w-md">
              <h3 className="text-lg font-semibold">Create your first team</h3>
              <p className="text-sm text-muted-foreground">
                No teams yet. Teams bind a reusable set of OIDC groups to
                project roles, so you can grant the same group access across
                multiple projects.
              </p>
            </div>
            <Button
              onClick={openCreate}
              disabled={!editable}
              size="lg"
              data-testid="create-team-button"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              New team
            </Button>
          </CardContent>
        </Card>
      ) : filteredTeams.length === 0 ? (
        // Filter produced no hits — quiet inline state, not the big CTA card
        // (operator just needs to clear/adjust the query).
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            <Search className="h-6 w-6 mx-auto mb-2 opacity-50" aria-hidden />
            No teams match{" "}
            <span className="font-mono text-foreground">"{query}"</span>.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3" data-testid="teams-list">
          {filteredTeams.map((team) => (
            <Card key={team.name} data-testid={`team-card-${team.name}`}>
              <CardContent className="p-0">
                <div className="flex items-start gap-4 px-6 py-5">
                  {/* Avatar tile — primary-tinted Users glyph, matches the
                      empty-state hero so the visual identity is consistent. */}
                  <div
                    className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary ring-1 ring-primary/20"
                    aria-hidden
                  >
                    <Users className="h-6 w-6" />
                  </div>

                  <div className="flex-1 min-w-0">
                    <h3 className="text-base font-semibold text-foreground truncate">
                      {team.name}
                    </h3>
                    {team.description && (
                      <p className="text-sm text-muted-foreground mt-1 line-clamp-2">
                        {team.description}
                      </p>
                    )}
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => openEdit(team)}
                      disabled={!editable}
                      data-testid={`edit-team-${team.name}`}
                      aria-label={`Manage ${team.name}`}
                    >
                      <Settings2 className="h-4 w-4 mr-1.5" />
                      Manage
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-9 w-9 p-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      onClick={() => setDeletingTeam(team)}
                      disabled={!editable}
                      aria-label={`Delete ${team.name}`}
                      data-testid={`delete-team-${team.name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>

                {/* Hairline divider + sectioned groups list. Section label
                    follows the mockup's "OIDC GROUPS · N" treatment so the
                    count is part of the heading instead of a separate badge. */}
                <div className="border-t border-border/60 px-6 py-4">
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground mb-3">
                    OIDC groups
                    <span className="mx-1.5 text-muted-foreground/40">·</span>
                    <span
                      className="text-foreground"
                      data-testid={`team-group-count-${team.name}`}
                    >
                      {team.oidcGroups.length}
                    </span>
                  </p>
                  {team.oidcGroups.length > 0 ? (
                    <div className="flex flex-wrap gap-2">
                      {team.oidcGroups.map((g) => (
                        <GroupChip key={g} group={g} />
                      ))}
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground italic">
                      No groups bound — this team grants no access.
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
          {showingFiltered && (
            <p className="text-xs text-muted-foreground text-center pt-1">
              Showing {filteredTeams.length} of {teams.length} teams
            </p>
          )}
        </div>
      )}

      <TeamDrawer
        key={drawerKey}
        open={drawerOpen}
        onOpenChange={(open) => {
          if (!open) closeForm();
        }}
        mode={editingTeam ? "edit" : "create"}
        initialValues={
          editingTeam
            ? {
                name: editingTeam.name,
                description: editingTeam.description ?? "",
                oidcGroups: [...editingTeam.oidcGroups],
              }
            : undefined
        }
        onSubmit={handleSubmit}
        isSubmitting={saving}
        suggestions={suggestions}
        canEdit={editable}
      />

      <AlertDialog
        open={!!deletingTeam}
        onOpenChange={(open) => !open && setDeletingTeam(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete team?</AlertDialogTitle>
            <AlertDialogDescription>
              This deletes team{" "}
              <span className="font-mono">{deletingTeam?.name}</span>. Any
              project role bound to it will lose the team's groups. This cannot
              be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button variant="ghost" onClick={() => setDeletingTeam(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
              data-testid="confirm-delete-team"
            >
              {deleteMutation.isPending && (
                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              )}
              Delete
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

export default TeamsSettings;
