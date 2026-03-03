import { Link } from "react-router-dom";
import { FileText, ShieldCheck, AlertTriangle, Activity } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { ComplianceSummary } from "@/types/compliance";
import { getEnforcementColors } from "@/types/compliance";

interface ComplianceSummaryCardsProps {
  summary?: ComplianceSummary;
  isLoading?: boolean;
}

interface SummaryCardProps {
  title: string;
  value: number | string;
  subtitle?: string;
  icon: typeof FileText;
  to?: string;
  variant?: "default" | "warning" | "danger";
  isLoading?: boolean;
}

function SummaryCard({
  title,
  value,
  subtitle,
  icon: Icon,
  to,
  variant = "default",
  isLoading,
}: SummaryCardProps) {
  const variantClasses = {
    default: "text-foreground",
    warning: "text-yellow-600 dark:text-yellow-400",
    danger: "text-red-600 dark:text-red-400",
  };

  const content = (
    <Card
      className={cn(
        "relative overflow-hidden transition-all hover:shadow-sm",
        to && "cursor-pointer hover:border-primary/50"
      )}
    >
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-20" />
            {subtitle && <Skeleton className="h-4 w-32" />}
          </div>
        ) : (
          <>
            <div className={cn("text-3xl font-bold", variantClasses[variant])}>
              {typeof value === "number" ? value.toLocaleString() : value}
            </div>
            {subtitle && (
              <p className="text-xs text-muted-foreground mt-1">{subtitle}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );

  if (to) {
    return <Link to={to}>{content}</Link>;
  }

  return content;
}

function EnforcementBreakdownCard({
  byEnforcement,
  isLoading,
}: {
  byEnforcement?: Record<string, number>;
  isLoading?: boolean;
}) {
  const enforcements = [
    { key: "deny", label: "Deny" },
    { key: "warn", label: "Warn" },
    { key: "dryrun", label: "Dry Run" },
  ];

  return (
    <Card className="relative overflow-hidden">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          By Enforcement
        </CardTitle>
        <Activity className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            {enforcements.map((e) => (
              <Skeleton key={e.key} className="h-6 w-full" />
            ))}
          </div>
        ) : (
          <div className="space-y-2">
            {enforcements.map((e) => {
              const count = byEnforcement?.[e.key] ?? 0;
              const colors = getEnforcementColors(e.key);

              return (
                <div
                  key={e.key}
                  className={cn(
                    "flex items-center justify-between rounded-md px-3 py-1.5",
                    colors.bg
                  )}
                >
                  <span className={cn("text-sm font-medium", colors.text)}>
                    {e.label}
                  </span>
                  <span className={cn("text-sm font-bold", colors.text)}>
                    {count.toLocaleString()}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function ComplianceSummaryCards({
  summary,
  isLoading,
}: ComplianceSummaryCardsProps) {
  const totalViolations = summary?.totalViolations ?? 0;
  const violationVariant = totalViolations > 0 ? "danger" : "default";

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <SummaryCard
        title="Policy Templates"
        value={summary?.totalTemplates ?? 0}
        subtitle="ConstraintTemplates"
        icon={FileText}
        to="/compliance/templates"
        isLoading={isLoading}
      />
      <SummaryCard
        title="Active Constraints"
        value={summary?.totalConstraints ?? 0}
        subtitle="Enforcing policies"
        icon={ShieldCheck}
        to="/compliance/constraints"
        isLoading={isLoading}
      />
      <SummaryCard
        title="Total Violations"
        value={totalViolations}
        subtitle="Resources non-compliant"
        icon={AlertTriangle}
        to="/compliance/violations"
        variant={violationVariant}
        isLoading={isLoading}
      />
      <EnforcementBreakdownCard
        byEnforcement={summary?.byEnforcement}
        isLoading={isLoading}
      />
    </div>
  );
}

export default ComplianceSummaryCards;
