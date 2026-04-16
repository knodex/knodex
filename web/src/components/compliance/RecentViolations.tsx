// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import { ExternalLink, AlertTriangle } from "@/lib/icons";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useRecentViolations } from "@/hooks/useCompliance";
import type { Violation } from "@/types/compliance";
import { getEnforcementColors } from "@/types/compliance";

interface RecentViolationsProps {
  limit?: number;
}

function EnforcementBadge({ action }: { action: string }) {
  const colors = getEnforcementColors(action);
  return (
    <Badge
      variant="outline"
      className={cn(colors.bg, colors.text, colors.border, "font-medium")}
    >
      {action}
    </Badge>
  );
}

function ResourceLink({ violation }: { violation: Violation }) {
  const { resource } = violation;

  return (
    <div className="flex flex-col">
      <span className="font-medium text-sm">{resource.name}</span>
      <span className="text-xs text-muted-foreground">
        {resource.kind}
        {resource.namespace && ` in ${resource.namespace}`}
      </span>
    </div>
  );
}

function ConstraintLink({ violation }: { violation: Violation }) {
  return (
    <Link
      to={`/compliance/constraints/${violation.constraintKind}/${violation.constraintName}`}
      className="text-sm font-medium text-primary hover:underline"
    >
      {violation.constraintName}
    </Link>
  );
}

function LoadingSkeleton() {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Resource</TableHead>
          <TableHead>Constraint</TableHead>
          <TableHead>Enforcement</TableHead>
          <TableHead className="hidden md:table-cell">Message</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {[1, 2, 3, 4, 5].map((i) => (
          <TableRow key={i}>
            <TableCell>
              <div className="space-y-1">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-3 w-32" />
              </div>
            </TableCell>
            <TableCell>
              <Skeleton className="h-4 w-28" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-6 w-16" />
            </TableCell>
            <TableCell className="hidden md:table-cell">
              <Skeleton className="h-4 w-48" />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="rounded-full bg-green-100 dark:bg-green-900/30 p-4 mb-4">
        <svg
          className="h-8 w-8 text-green-600 dark:text-green-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      <h3 className="text-lg font-semibold text-green-600 dark:text-green-400">
        No Violations
      </h3>
      <p className="text-sm text-muted-foreground mt-1 max-w-[240px]">
        All resources are compliant with your Gatekeeper policies
      </p>
    </div>
  );
}

export function RecentViolations({ limit = 10 }: RecentViolationsProps) {
  const { data, isLoading, error } = useRecentViolations(limit);

  const violations = data?.items ?? [];

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-muted-foreground" />
            Recent Violations
          </CardTitle>
          <CardDescription>
            Latest policy violations detected by Gatekeeper
          </CardDescription>
        </div>
        {violations.length > 0 && (
          <Button variant="ghost" size="sm" asChild>
            <Link to="/compliance/violations" className="flex items-center gap-1">
              View All
              <ExternalLink className="h-3 w-3" />
            </Link>
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {error ? (
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <AlertTriangle className="h-8 w-8 text-destructive mb-2" />
            <p className="text-sm text-destructive">
              Failed to load violations
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {error instanceof Error ? error.message : "Unknown error"}
            </p>
          </div>
        ) : isLoading ? (
          <LoadingSkeleton />
        ) : violations.length === 0 ? (
          <EmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Resource</TableHead>
                <TableHead>Constraint</TableHead>
                <TableHead>Enforcement</TableHead>
                <TableHead className="hidden md:table-cell max-w-[300px]">
                  Message
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {violations.map((violation, index) => (
                <TableRow key={`${violation.constraintKind}-${violation.constraintName}-${violation.resource.name}-${index}`}>
                  <TableCell>
                    <ResourceLink violation={violation} />
                  </TableCell>
                  <TableCell>
                    <ConstraintLink violation={violation} />
                  </TableCell>
                  <TableCell>
                    <EnforcementBadge action={violation.enforcementAction} />
                  </TableCell>
                  <TableCell className="hidden md:table-cell max-w-[300px]">
                    <p className="text-sm text-muted-foreground truncate">
                      {violation.message}
                    </p>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

export default RecentViolations;
