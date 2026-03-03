import { Link } from "react-router-dom";
import { ArrowLeft, ScrollText } from "lucide-react";
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
    <div className="container mx-auto py-6 px-4 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="mb-8">
        <Link
          to="/settings"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Settings
        </Link>
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <ScrollText className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">Audit Configuration</h2>
            <p className="text-muted-foreground">
              Configure audit trail settings
            </p>
          </div>
        </div>
      </div>

      {/* Configuration form */}
      <AuditConfigForm />
    </div>
  );
}
