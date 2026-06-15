// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { isEnterprise } from "@/hooks/useCompliance";
import { useSettings } from "@/hooks/useSettings";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Settings (General) — default pane of the Settings master-detail shell.
 *
 * Layout mirrors the design-prototype General page: stacked Cards with a
 * Section title and a list of label/value rows. Strict policy: every row
 * here is sourced from a real backend field. Anything the prototype shows
 * that we don't have a wire format for is deliberately omitted rather than
 * fabricated.
 *
 * Fields we DO have:
 *  - organization name (`/api/v1/settings.organization`)
 *  - edition (build-time `isEnterprise()`)
 *  - app version (build literal `__APP_VERSION__`)
 *  - authentication issuer (server-set on session restore, e.g. "Local",
 *    "Keycloak"; surfaced via userStore.issuer)
 *
 * Prototype rows DELIBERATELY OMITTED — no backend:
 *  - Region, Plan, Created, Default project
 *  - API version, Cluster, GitOps engine version, Gatekeeper version
 *
 * The Settings title, menu, and content framing live in `SettingsLayout`,
 * which wraps this component at the `/settings` index route. Access control:
 * visible to all users; each sub-section handles its own authorization via
 * API calls (403 → Access Denied surface).
 */
export function Settings() {
  const { data: settings, isLoading: settingsLoading } = useSettings();

  const edition = isEnterprise() ? "Enterprise" : "OSS";
  const version = typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "dev";

  return (
    <div className="space-y-5">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">Organisation</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 pt-0">
          <DataRow
            label="Name"
            value={
              settingsLoading ? (
                <Skeleton className="h-4 w-24 inline-block" />
              ) : (
                settings?.organization ?? "—"
              )
            }
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">Platform</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 pt-0">
          <DataRow label="Version" value={`v${version}`} mono />
          <DataRow label="Edition" value={edition} />
        </CardContent>
      </Card>
    </div>
  );
}

interface DataRowProps {
  label: string;
  value: ReactNode;
  mono?: boolean;
}

/**
 * One label/value row inside a Settings Card. Visual mirror of the prototype's
 * `<DataRow>` (label left, value right, subtle bottom border within the card).
 * The last row in a Card carries no border thanks to `last:border-b-0`.
 */
function DataRow({ label, value, mono = false }: DataRowProps) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-[var(--border-subtle)] pb-3 text-sm last:border-b-0 last:pb-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd
        className={
          mono
            ? "font-mono text-xs text-foreground"
            : "font-medium text-foreground"
        }
      >
        {value}
      </dd>
    </div>
  );
}

export default Settings;
