// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { FileText, ShieldCheck, AlertTriangle } from "@/lib/icons";
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
    warning: "text-amber-700 dark:text-amber-400",
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

export function ComplianceSummaryCards({
  summary,
  isLoading,
}: ComplianceSummaryCardsProps) {
  const totalViolations = summary?.totalViolations ?? 0;

  return (
    <div className="grid gap-4 md:grid-cols-3">
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
      <Link to="/compliance/violations">
        <Card className="relative overflow-hidden transition-all hover:shadow-sm cursor-pointer hover:border-primary/50">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Violations
            </CardTitle>
            <AlertTriangle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-8 w-20" />
                <Skeleton className="h-4 w-32" />
              </div>
            ) : (
              <>
                <div className={cn("text-3xl font-bold", totalViolations > 0 ? "text-red-600 dark:text-red-400" : "text-foreground")}>
                  {totalViolations.toLocaleString()}
                </div>
                <div className="flex items-center gap-2 mt-2">
                  {[
                    { key: "deny", label: "Deny" },
                    { key: "warn", label: "Warn" },
                    { key: "dryrun", label: "Dry Run" },
                  ].map((e) => {
                    const count = summary?.byEnforcement?.[e.key] ?? 0;
                    const colors = getEnforcementColors(e.key);
                    return (
                      <span
                        key={e.key}
                        className={cn("rounded px-1.5 py-0.5 text-xs font-medium", colors.bg, colors.text)}
                      >
                        {e.label} {count}
                      </span>
                    );
                  })}
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </Link>
    </div>
  );
}

export default ComplianceSummaryCards;
