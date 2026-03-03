import {
  ComplianceSummaryCards,
  ViolationsByEnforcement,
  RecentViolations,
  EnterpriseRequired,
  GatekeeperUnavailable,
} from "@/components/compliance";
import { useComplianceSummary, isEnterprise } from "@/hooks/useCompliance";
import { isEnterpriseRequired, isGatekeeperUnavailable } from "@/api/compliance";
import { useIsFeatureEnabled } from "@/hooks/useLicense";

export function ComplianceDashboard() {
  const { data: summary, isLoading, error } = useComplianceSummary();
  const complianceEnabled = useIsFeatureEnabled("compliance");

  // Show enterprise gate for non-enterprise builds
  if (!isEnterprise()) {
    return (
      <EnterpriseRequired
        feature="Policy Compliance Dashboard"
        description="Monitor OPA Gatekeeper policy compliance across your Kubernetes clusters with real-time violation tracking and detailed analytics."
      />
    );
  }

  // Show license gate when enterprise build but feature not licensed
  if (!complianceEnabled && !isLoading && !error) {
    return (
      <EnterpriseRequired
        feature="Policy Compliance Dashboard"
        description="Your current license does not include the Policy Compliance feature. Upgrade to Enterprise to unlock full compliance monitoring capabilities."
      />
    );
  }

  // Handle 402 Payment Required from backend (enterprise not licensed)
  if (error && isEnterpriseRequired(error)) {
    return (
      <EnterpriseRequired
        feature="Policy Compliance Dashboard"
        description="Your current license does not include the Policy Compliance feature. Upgrade to Enterprise to unlock full compliance monitoring capabilities."
      />
    );
  }

  // Handle 503 Service Unavailable (Gatekeeper not installed or syncing)
  if (error && isGatekeeperUnavailable(error)) {
    return (
      <GatekeeperUnavailable
        message="OPA Gatekeeper is not available in your cluster. Please verify Gatekeeper is installed and the controller pods are running."
      />
    );
  }

  // Show generic error for other errors
  if (error && !isEnterpriseRequired(error) && !isGatekeeperUnavailable(error)) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <div className="rounded-full bg-destructive/10 p-4">
          <svg
            className="h-8 w-8 text-destructive"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
        </div>
        <div className="text-center">
          <h2 className="text-lg font-semibold">Failed to Load Compliance Data</h2>
          <p className="text-sm text-muted-foreground mt-1">
            {error instanceof Error ? error.message : "An unexpected error occurred"}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      {/* Page Header */}
      <div>
        <h2 className="text-sm font-medium text-foreground">Policy Compliance</h2>
        <p className="text-muted-foreground">
          Monitor OPA Gatekeeper policy compliance across your clusters
        </p>
      </div>

      {/* Summary Cards */}
      <ComplianceSummaryCards summary={summary} isLoading={isLoading} />

      {/* Main Content Grid */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Violations by Enforcement - Takes 1 column */}
        <div className="lg:col-span-1">
          <ViolationsByEnforcement summary={summary} isLoading={isLoading} />
        </div>

        {/* Recent Violations Table - Takes 2 columns */}
        <div className="lg:col-span-2">
          <RecentViolations limit={10} />
        </div>
      </div>
    </div>
  );
}

export default ComplianceDashboard;
