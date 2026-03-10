// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { FolderGit2, Users, KeyRound, ScrollText } from "lucide-react";
import { SettingsCard } from "@/components/settings/SettingsCard";
import { isEnterprise } from "@/hooks/useCompliance";

/**
 * Settings page - Hub for platform administration
 * Following ArgoCD's Settings pattern with card-based navigation
 *
 * Access control: Settings is visible to all users. Each sub-section
 * (repositories, projects) handles its own authorization via API calls.
 * If the API returns 403, the sub-section displays an Access Denied message.
 * This follows the ArgoCD pattern of pure Casbin permission checks at the API layer.
 */
export function Settings() {
  return (
    <div className="py-6">
      {/* Header */}
      <div className="mb-8">
        <h2 className="text-sm font-medium text-foreground">Settings</h2>
        <p className="text-muted-foreground">
          Manage platform configuration, repositories, and access control
        </p>
      </div>

      {/* Settings cards grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <SettingsCard
          title="Repositories"
          description="Configure Git repositories for ResourceGraphDefinitions. Manage credentials and sync settings."
          icon={FolderGit2}
          to="/settings/repositories"
        />

        <SettingsCard
          title="Projects"
          description="Manage RBAC projects, roles, and policies. Configure source and destination restrictions."
          icon={Users}
          to="/settings/projects"
        />

        <SettingsCard
          title="SSO Providers"
          description="Manage OIDC authentication providers. Configure issuer URLs, client credentials, and scopes."
          icon={KeyRound}
          to="/settings/sso"
        />

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
