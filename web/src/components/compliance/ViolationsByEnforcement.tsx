// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { ComplianceSummary } from "@/types/compliance";
import { getEnforcementColors } from "@/types/compliance";

interface ViolationsByEnforcementProps {
  summary?: ComplianceSummary;
  isLoading?: boolean;
}

interface EnforcementBarProps {
  label: string;
  count: number;
  total: number;
  actionKey: string;
}

function EnforcementBar({ label, count, total, actionKey }: EnforcementBarProps) {
  const percentage = total > 0 ? Math.round((count / total) * 100) : 0;
  const colors = getEnforcementColors(actionKey);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <div className="flex items-center gap-2">
          <div
            className={cn(
              "h-3 w-3 rounded-full",
              actionKey === "deny" && "bg-red-500",
              actionKey === "warn" && "bg-amber-500",
              actionKey === "dryrun" && "bg-blue-500"
            )}
          />
          <span className="font-medium">{label}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className={cn("font-bold", colors.text)}>
            {count.toLocaleString()}
          </span>
          <span className="text-muted-foreground">({percentage}%)</span>
        </div>
      </div>
      <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-500",
            actionKey === "deny" && "bg-red-500",
            actionKey === "warn" && "bg-amber-500",
            actionKey === "dryrun" && "bg-blue-500"
          )}
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      {[1, 2, 3].map((i) => (
        <div key={i} className="space-y-2">
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-16" />
          </div>
          <Skeleton className="h-2 w-full" />
        </div>
      ))}
    </div>
  );
}

export function ViolationsByEnforcement({
  summary,
  isLoading,
}: ViolationsByEnforcementProps) {
  const byEnforcement = summary?.byEnforcement ?? {};
  const total = summary?.totalViolations ?? 0;

  const enforcements = [
    { key: "deny", label: "Deny", description: "Blocked resource creation" },
    { key: "warn", label: "Warn", description: "Logged warning but allowed" },
    { key: "dryrun", label: "Dry Run", description: "Audit mode only" },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Violations by Enforcement</CardTitle>
        <CardDescription>
          Distribution of violations by enforcement action
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <LoadingSkeleton />
        ) : total === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <div className="rounded-full bg-green-100 dark:bg-green-900/30 p-3 mb-4">
              <svg
                className="h-6 w-6 text-green-600 dark:text-green-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </div>
            <p className="text-sm font-medium text-green-600 dark:text-green-400">
              All Clear!
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              No policy violations detected
            </p>
          </div>
        ) : (
          <div className="space-y-6">
            {enforcements.map((e) => {
              const count = byEnforcement[e.key] ?? 0;
              return (
                <EnforcementBar
                  key={e.key}
                  label={e.label}
                  count={count}
                  total={total}
                  actionKey={e.key}
                />
              );
            })}
            <div className="pt-4 border-t">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Total Violations</span>
                <span className="font-bold text-lg">{total.toLocaleString()}</span>
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default ViolationsByEnforcement;
