// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { User, Shield, Users, KeyRound } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useUserStore } from "@/stores/userStore";
import { getAccountInfo } from "@/api/auth";

/**
 * Formats a Unix timestamp as a human-readable date/time string.
 */
function formatTimestamp(ts: number | null): string {
  if (!ts) return "N/A";
  return new Date(ts * 1000).toLocaleString();
}

export function UserInfoPage() {
  const user = useUserStore((s) => s.user);
  const storeGroups = useUserStore((s) => s.groups);
  const casbinRoles = useUserStore((s) => s.casbinRoles);
  const roles = useUserStore((s) => s.roles);
  const issuer = useUserStore((s) => s.issuer);

  // Fetch server-authoritative account info for filtered groups.
  // The backend filters groups to only those with Casbin policy mappings,
  // while the JWT contains all IdP groups.
  const { data: accountInfo, isLoading: isAccountLoading } = useQuery({
    queryKey: ["account", "info"],
    queryFn: getAccountInfo,
    enabled: !!user,
    staleTime: 60 * 1000,
  });

  // Use server-filtered groups when available, fall back to JWT groups from store.
  // While the API is loading, keep groups empty to avoid flashing unfiltered JWT groups.
  const groups = accountInfo?.groups ?? (isAccountLoading ? [] : storeGroups);

  const isLocal = useMemo(() => user?.email?.endsWith("@local") ?? false, [user]);

  if (!user) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <p className="text-muted-foreground">Not authenticated</p>
      </div>
    );
  }

  return (
    <div className="py-6">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-sm font-medium text-foreground">Account Info</h2>
        <p className="text-muted-foreground">
          Your identity, access level, and session details
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Identity Card */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
              <User className="h-5 w-5" />
            </div>
            <CardTitle className="text-lg">Identity</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Display Name" value={user.name || user.email?.split("@")[0] || "—"} />
            <InfoRow label="Email" value={user.email || "—"} />
            <InfoRow label="User ID" value={user.id} mono />
          </CardContent>
        </Card>

        {/* Authentication Card */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
              <KeyRound className="h-5 w-5" />
            </div>
            <CardTitle className="text-lg">Authentication</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Issuer" value={isLocal ? "Local" : (issuer || "OIDC")} mono={!isLocal} />
            <InfoRow label="Issued At" value={isAccountLoading ? "Loading..." : formatTimestamp(accountInfo?.tokenIssuedAt ?? null)} />
          </CardContent>
        </Card>

        {/* Groups Card */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
              <Users className="h-5 w-5" />
            </div>
            <CardTitle className="text-lg">Groups</CardTitle>
          </CardHeader>
          <CardContent>
            {isAccountLoading ? (
              <div className="flex flex-wrap gap-2">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="h-6 w-24 animate-pulse rounded-full bg-muted" />
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
          <CardHeader className="flex flex-row items-center gap-3 pb-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary border border-primary/20">
              <Shield className="h-5 w-5" />
            </div>
            <CardTitle className="text-lg">Roles & Access</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
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
            {/* Project-Scoped Roles */}
            {Object.keys(roles).length > 0 && (
              <div>
                <h4 className="text-sm font-medium mb-2">Project Roles</h4>
                <div className="flex flex-wrap gap-2">
                  {Object.entries(roles).map(([project, role]) => (
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
    </div>
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
