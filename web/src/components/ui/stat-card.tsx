// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * StatCard — compact metric card (title + big number + optional icon).
 *
 * Shared primitive promoted out of the audit-local copy (AuditStats.tsx). The
 * `warning` variant colors the value via the `--status-warning` design token
 * (no literal hex, no Tailwind palette class) so callers like the project
 * Overview "Issues" card stay token-faithful (AC 8 / AC 17).
 */

import * as React from "react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "@/lib/icons";

export type StatCardVariant = "default" | "warning";

export interface StatCardProps {
  title: string;
  value: number | string;
  subtitle?: string;
  icon?: LucideIcon;
  variant?: StatCardVariant;
  isLoading?: boolean;
}

export function StatCard({
  title,
  value,
  subtitle,
  icon: Icon,
  variant = "default",
  isLoading,
}: StatCardProps) {
  const valueStyle =
    variant === "warning" ? { color: "var(--status-warning)" } : undefined;

  return (
    <Card className="relative overflow-hidden transition-all hover:shadow-md">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        {Icon && <Icon className="h-4 w-4 text-muted-foreground" />}
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-20" />
            {subtitle !== undefined && <Skeleton className="h-4 w-32" />}
          </div>
        ) : (
          <>
            <div
              className={cn(
                "text-3xl font-bold",
                variant === "default" && "text-foreground"
              )}
              style={valueStyle}
            >
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
}
