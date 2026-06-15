// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { isEnterprise } from "@/hooks/useCompliance";
import { EnterpriseRequired } from "@/components/compliance";
import { AuditConfigForm } from "@/components/settings/audit/AuditConfigForm";

/**
 * Audit Settings page — Config form only.
 *
 * The events table, filters, and stats have moved to the top-level /audit page.
 * This page retains only the audit configuration form under /settings/audit.
 *
 * Enterprise-only feature gated by __ENTERPRISE__ build-time constant.
 */
export function AuditSettings() {
  // Enterprise gate
  if (!isEnterprise()) {
    return (
      <EnterpriseRequired
        feature="Audit Trail"
        description="Monitor user actions and security events with a comprehensive audit trail. Track logins, permission changes, and resource modifications."
      />
    );
  }

  return (
    <div>
      {/* Configuration form — the real enabled + retentionDays fields. The
          prototype's Events-24h stat, Storage, and Forwarding sections have no
          production backend and are intentionally NOT built (flagged in the PR
          for design-owner confirmation). */}
      <AuditConfigForm />
    </div>
  );
}
