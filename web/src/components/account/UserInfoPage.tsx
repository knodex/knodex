// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { User, Shield, Users, KeyRound, Globe, type LucideIcon } from "@/lib/icons";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { useUserStore } from "@/stores/userStore";
import { getAccountInfo } from "@/api/auth";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Formats a Unix timestamp as a human-readable date/time string.
 */
function formatTimestamp(ts: number | null): string {
  if (!ts) return "N/A";
  return new Date(ts * 1000).toLocaleString();
}

/**
 * UserInfoPage renders the self-scoped "My Access" view (story 17.1) — the
 * coherent lobby every authenticated user lands in, including a `member` with
 * zero project bindings. It honestly surfaces identity, OIDC groups, the
 * effective application role, bound projects + their project roles, and a
 * plain-language effective-access summary (never raw Casbin tuples). With no
 * bindings it shows an explicit, honest empty-state instead of a 403 wall.
 *
 * Route/file/testid wiring is preserved (still `/user-info`) so existing E2E +
 * unit coverage carries over; only the visible header label moved to "My Access".
 */
export function UserInfoPage() {
  const user = useUserStore((s) => s.user);
  const storeGroups = useUserStore((s) => s.groups);
  const casbinRoles = useUserStore((s) => s.casbinRoles);
  const storeRoles = useUserStore((s) => s.roles);
  const storeProjects = useUserStore((s) => s.projects);
  const issuer = useUserStore((s) => s.issuer);

  // Fetch server-authoritative account info. The backend filters groups to only
  // those with Casbin policy mappings and computes the effective applicationRole
  // (story 17.1) from existing Casbin state, while the JWT contains all IdP groups.
  const { data: accountInfo, isLoading: isAccountLoading } = useQuery({
    queryKey: ["account", "info"],
    queryFn: getAccountInfo,
    enabled: !!user,
    staleTime: STALE_TIME.STANDARD,
  });

  // Use server-filtered groups when available, fall back to JWT groups from store.
  // While the API is loading, keep groups empty to avoid flashing unfiltered JWT groups.
  const groups = accountInfo?.groups ?? (isAccountLoading ? [] : storeGroups);

  // Effective application role badge value. Prefer the server-authoritative
  // applicationRole; before the API resolves, derive the identical predicate from
  // the store's casbinRoles so the badge never flashes empty. Both paths are pure
  // display — `member` is never a Casbin subject (NFR-T1).
  const applicationRole =
    accountInfo?.applicationRole ??
    (casbinRoles.includes("role:serveradmin") ? "serveradmin" : "member");
  const isServerAdmin = applicationRole === "serveradmin";

  // Bound projects + their project roles. Prefer server-authoritative data; fall
  // back to the store when the API has not resolved (keeps existing coverage green).
  const roles = accountInfo?.roles ?? storeRoles;
  const projects = accountInfo?.projects ?? storeProjects;
  const roleEntries = Object.entries(roles);
  const hasBindings = roleEntries.length > 0 || projects.length > 0;

  const isLocal = useMemo(() => user?.email?.endsWith("@local") ?? false, [user]);

  if (!user) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <p className="text-muted-foreground">Not authenticated</p>
      </div>
    );
  }

  return (
    <div className="py-6" data-testid="my-access-view">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-sm font-medium text-foreground">My Access</h2>
        <p className="text-muted-foreground">
          Your identity, access level, and session details
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Identity Card */}
        <Card>
          <IconedCardHeader icon={User} title="Identity" />
          <CardContent className="space-y-3">
            <InfoRow label="Display Name" value={user.name || user.email?.split("@")[0] || "—"} />
            <InfoRow label="Email" value={user.email || "—"} />
            <InfoRow label="User ID" value={user.id} mono />
          </CardContent>
        </Card>

        {/* Authentication Card */}
        <Card>
          <IconedCardHeader icon={KeyRound} title="Authentication" />
          <CardContent className="space-y-3">
            <InfoRow label="Issuer" value={isLocal ? "Local" : (issuer || "OIDC")} mono={!isLocal} />
            <InfoRow label="Issued At" value={isAccountLoading ? "Loading..." : formatTimestamp(accountInfo?.tokenIssuedAt ?? null)} />
          </CardContent>
        </Card>

        {/* Groups Card */}
        <Card>
          <IconedCardHeader icon={Users} title="Groups" />
          <CardContent>
            {isAccountLoading ? (
              <div className="flex flex-wrap gap-2">
                {[1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-6 w-24 rounded-full" />
                ))}
              </div>
            ) : groups.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {isLocal ? "Local admin users have no OIDC groups" : "No groups assigned"}
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {groups.map((group) => (
                  <Badge key={group} variant="secondary">{group}</Badge>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Roles & Access Card */}
        <Card>
          <IconedCardHeader icon={Shield} title="Roles & Access" />
          <CardContent className="space-y-4">
            {/* Application role (story 17.1) — derived display value, not a subject */}
            <div>
              <h4 className="text-sm font-medium mb-2">Application Role</h4>
              <Badge
                variant={isServerAdmin ? "default" : "secondary"}
                data-testid="application-role"
              >
                {applicationRole}
              </Badge>
            </div>
            {/* Global Roles */}
            <div>
              <h4 className="text-sm font-medium mb-2">Global Roles</h4>
              {casbinRoles.length === 0 ? (
                <p className="text-sm text-muted-foreground">No global roles</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {casbinRoles.filter((r) => r.startsWith("role:")).map((role) => (
                    <Badge key={role} variant="default">{role}</Badge>
                  ))}
                </div>
              )}
            </div>
            {/* Project-Scoped Roles (bound projects + their project roles) */}
            {roleEntries.length > 0 && (
              <div data-testid="bound-projects">
                <h4 className="text-sm font-medium mb-2">Project Roles</h4>
                <div className="flex flex-wrap gap-2">
                  {roleEntries.map(([project, role]) => (
                    <Badge key={project} variant="outline">
                      {role} on {project}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Effective-access summary — plain language, never raw Casbin tuples */}
      <Card className="mt-6" data-testid="effective-access-summary">
        <IconedCardHeader icon={Globe} title="Effective Access" />
        <CardContent>
          {isServerAdmin ? (
            <p className="text-sm text-muted-foreground">
              You're a server administrator with full access across all projects.
            </p>
          ) : hasBindings ? (
            <ul className="space-y-1 text-sm text-muted-foreground">
              {roleEntries.length > 0
                ? roleEntries.map(([project, role]) => (
                    <li key={project}>
                      You can act as <span className="font-medium text-foreground">{role}</span> on{" "}
                      <span className="font-medium text-foreground">{project}</span>.
                    </li>
                  ))
                : projects.map((project) => (
                    <li key={project}>
                      You're a member of <span className="font-medium text-foreground">{project}</span>.
                    </li>
                  ))}
            </ul>
          ) : (
            <div data-testid="my-access-empty" className="space-y-1">
              <p className="text-sm font-medium text-foreground">
                You're signed in but not yet a member of any project
              </p>
              <p className="text-sm text-muted-foreground">
                Ask a project administrator to add you to a project, or an
                administrator to grant you access. Once you're a member, your
                projects and what you can do will appear here.
              </p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

/** Card header with a circular accent icon and title */
function IconedCardHeader({ icon: Icon, title }: { icon: LucideIcon; title: string }) {
  return (
    <CardHeader className="flex flex-row items-center gap-3 pb-3">
      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
        <Icon className="h-5 w-5" />
      </div>
      <CardTitle className="text-lg">{title}</CardTitle>
    </CardHeader>
  );
}

/** Small helper for label-value rows */
function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={`text-sm font-medium ${mono ? "font-mono text-xs" : ""}`}>{value}</span>
    </div>
  );
}

export default UserInfoPage;
