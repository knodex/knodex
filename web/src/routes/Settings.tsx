// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { KeyRound, ScrollText, ShieldCheck } from "@/lib/icons";
import { SettingsCard } from "@/components/settings/SettingsCard";
import { PageHeader } from "@/components/layout/PageHeader";
import { isEnterprise } from "@/hooks/useCompliance";

/**
 * Settings page - Hub for platform administration
 * Following ArgoCD's Settings pattern with card-based navigation
 *
 * Access control: Settings is visible to all users. Each sub-section
 * handles its own authorization via API calls.
 * If the API returns 403, the sub-section displays an Access Denied message.
 * This follows the ArgoCD pattern of pure Casbin permission checks at the API layer.
 *
 * Note: Projects and Repositories have been promoted to top-level routes
 * (/projects, /repositories) and are no longer part of Settings.
 */
export function Settings() {
  return (
    <div className="py-6">
      {/* Header */}
      <PageHeader
        title="Settings"
        description="Manage platform configuration and access control"
        className="mb-8"
      />

      {/* Settings cards grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <SettingsCard
          title="SSO Providers"
          description="Manage OIDC authentication providers. Configure issuer URLs, client credentials, and scopes."
          icon={KeyRound}
          to="/settings/sso"
        />

        {isEnterprise() && (
          <SettingsCard
            title="License"
            description="View license status, expiry date, and activate or renew your enterprise license."
            icon={ShieldCheck}
            to="/settings/license"
          />
        )}

        {isEnterprise() && (
          <SettingsCard
            title="Audit Configuration"
            description="Configure audit trail settings. Enable or disable audit recording and set retention policies."
            icon={ScrollText}
            to="/settings/audit"
          />
        )}
      </div>

      {/* Version info */}
      <div className="mt-8 text-xs text-muted-foreground">
        Knodex v{typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "dev"} &middot; {isEnterprise() ? "Enterprise" : "OSS"} Edition
      </div>
    </div>
  );
}

export default Settings;
