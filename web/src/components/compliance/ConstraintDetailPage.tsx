// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useParams, Link } from "react-router-dom";
import { toast } from "sonner";
import {
  Shield,
  ExternalLink,
  AlertTriangle,
  FileText,
  Layers,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useConstraint, useUpdateConstraintEnforcement } from "@/hooks/useCompliance";
import { PageHeader } from "./PageHeader";
import { EnforcementBadge } from "./EnforcementBadge";
import { EnforcementSelector } from "./EnforcementSelector";
import { MatchRulesDisplay } from "./MatchRulesDisplay";
import { ErrorState } from "./ErrorState";
import { formatDate } from "@/lib/date";
import type { EnforcementAction } from "@/types/compliance";

/**
 * Constraint detail page
 * AC-CON-07: Full constraint info with parameters, violations
 * AC-CON-08: Match scope visible (which kinds, namespaces, etc.)
 */
export function ConstraintDetailPage() {
  const { kind, name } = useParams<{ kind: string; name: string }>();

  const {
    data: constraint,
    isLoading,
    isError,
    error,
    refetch,
    isRefetching,
  } = useConstraint(kind || "", name || "");

  const updateEnforcement = useUpdateConstraintEnforcement();

  const handleEnforcementChange = async (newAction: EnforcementAction) => {
    if (!kind || !name) return;

    await updateEnforcement.mutateAsync(
      { kind, name, enforcementAction: newAction },
      {
        onSuccess: () => {
          toast.success(`Enforcement action updated to "${newAction}"`);
        },
        onError: (err) => {
          const message = err instanceof Error ? err.message : "Failed to update enforcement action";
          toast.error(message);
          throw err; // Re-throw to keep dialog open
        },
      }
    );
  };

  if (!kind || !name) {
    return (
      <ErrorState
        message="Invalid Constraint"
        details="Kind and name are required in the URL"
      />
    );
  }

  if (isLoading) {
    return <ConstraintDetailSkeleton />;
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Constraint"
          breadcrumbs={[
            { label: "Compliance", href: "/compliance" },
            { label: "Constraints", href: "/compliance/constraints" },
            { label: name },
          ]}
        />
        <ErrorState
          message="Failed to load constraint"
          details={error instanceof Error ? error.message : "Unknown error"}
          onRetry={() => refetch()}
          isRetrying={isRefetching}
        />
      </div>
    );
  }

  if (!constraint) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Constraint Not Found"
          breadcrumbs={[
            { label: "Compliance", href: "/compliance" },
            { label: "Constraints", href: "/compliance/constraints" },
            { label: name },
          ]}
        />
        <ErrorState
          message="Constraint Not Found"
          details={`Constraint "${kind}/${name}" does not exist`}
        />
      </div>
    );
  }

  const violations = constraint.violations || [];

  return (
    <div className="space-y-6">
      <PageHeader
        title={constraint.name}
        subtitle={`${constraint.kind} constraint`}
        breadcrumbs={[
          { label: "Compliance", href: "/compliance" },
          { label: "Constraints", href: "/compliance/constraints" },
          { label: constraint.name },
        ]}
        actions={
          <Button variant="outline" asChild>
            <Link to="/compliance/constraints">Back to Constraints</Link>
          </Button>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Constraint Info */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Shield className="h-5 w-5 text-muted-foreground" />
                Constraint Details
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Kind
                  </p>
                  <Link
                    to={`/compliance/constraints?kind=${constraint.kind}`}
                    className="text-lg font-semibold text-primary hover:underline flex items-center gap-1"
                  >
                    {constraint.kind}
                    <ExternalLink className="h-4 w-4" />
                  </Link>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Template
                  </p>
                  <Link
                    to={`/compliance/templates/${constraint.templateName}`}
                    className="text-lg font-semibold text-primary hover:underline flex items-center gap-1"
                  >
                    {constraint.templateName}
                    <ExternalLink className="h-4 w-4" />
                  </Link>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Created
                  </p>
                  <p className="text-lg font-semibold">
                    {formatDate(constraint.createdAt)}
                  </p>
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-2">
                    Enforcement Action
                  </p>
                  <EnforcementSelector
                    currentAction={constraint.enforcementAction}
                    constraintKind={constraint.kind}
                    constraintName={constraint.name}
                    onEnforcementChange={handleEnforcementChange}
                    isUpdating={updateEnforcement.isPending}
                    canUpdate={true}
                  />
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-2">
                    Violations
                  </p>
                  {constraint.violationCount > 0 ? (
                    <Badge variant="destructive" className="font-mono text-sm">
                      {constraint.violationCount} violation
                      {constraint.violationCount !== 1 ? "s" : ""}
                    </Badge>
                  ) : (
                    <Badge
                      variant="outline"
                      className="text-green-600 border-green-200 bg-green-50 dark:text-green-400 dark:border-green-900 dark:bg-green-950/30 text-sm"
                    >
                      No violations
                    </Badge>
                  )}
                </div>
              </div>

              {constraint.labels &&
                Object.keys(constraint.labels).length > 0 && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-2">
                      Labels
                    </p>
                    <div className="flex flex-wrap gap-1.5">
                      {Object.entries(constraint.labels).map(([key, value]) => (
                        <Badge
                          key={key}
                          variant="secondary"
                          className="text-xs"
                        >
                          {key}: {value}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}
            </CardContent>
          </Card>

          {/* Match Rules */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Layers className="h-5 w-5 text-muted-foreground" />
                Match Rules
              </CardTitle>
            </CardHeader>
            <CardContent>
              <MatchRulesDisplay match={constraint.match} variant="full" />
            </CardContent>
          </Card>

          {/* Parameters */}
          {constraint.parameters &&
            Object.keys(constraint.parameters).length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <FileText className="h-5 w-5 text-muted-foreground" />
                    Parameters
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <pre className="p-4 bg-muted rounded-lg overflow-x-auto text-sm">
                    <code>
                      {JSON.stringify(constraint.parameters, null, 2)}
                    </code>
                  </pre>
                </CardContent>
              </Card>
            )}

          {/* Violations Table */}
          {violations.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <AlertTriangle className="h-5 w-5 text-destructive" />
                  Violations
                  <Badge variant="destructive" className="ml-2">
                    {violations.length}
                  </Badge>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead style={{ width: "30%" }}>Resource</TableHead>
                      <TableHead style={{ width: "15%" }}>Namespace</TableHead>
                      <TableHead style={{ width: "55%" }}>Message</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {violations.map((violation, index) => (
                      <TableRow key={index}>
                        <TableCell className="font-medium">
                          <div className="flex flex-col">
                            <span className="text-sm">
                              {violation.resource.kind}/{violation.resource.name}
                            </span>
                            {violation.resource.apiGroup && (
                              <span className="text-xs text-muted-foreground">
                                {violation.resource.apiGroup}
                              </span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {violation.resource.namespace || (
                            <span className="italic">cluster-scoped</span>
                          )}
                        </TableCell>
                        <TableCell className="text-sm">
                          {violation.message}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>

                <div className="mt-4">
                  <Button variant="outline" asChild>
                    <Link
                      to={`/compliance/violations?constraint=${constraint.kind}/${constraint.name}`}
                    >
                      View All Violations
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Quick Stats</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                <span className="text-sm text-muted-foreground">
                  Enforcement
                </span>
                <EnforcementBadge action={constraint.enforcementAction} />
              </div>
              <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                <span className="text-sm text-muted-foreground">
                  Change Action
                </span>
                <EnforcementSelector
                  currentAction={constraint.enforcementAction}
                  constraintKind={constraint.kind}
                  constraintName={constraint.name}
                  onEnforcementChange={handleEnforcementChange}
                  isUpdating={updateEnforcement.isPending}
                  canUpdate={true}
                  className="w-[130px]"
                />
              </div>
              <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                <span className="text-sm text-muted-foreground">
                  Violations
                </span>
                <span
                  className={`font-semibold ${
                    constraint.violationCount > 0
                      ? "text-destructive"
                      : "text-green-600 dark:text-green-400"
                  }`}
                >
                  {constraint.violationCount}
                </span>
              </div>
              <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                <span className="text-sm text-muted-foreground">
                  Target Kinds
                </span>
                <span className="font-semibold">
                  {constraint.match.kinds?.flatMap((k) => k.kinds).length || 0}
                </span>
              </div>
              <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
                <span className="text-sm text-muted-foreground">
                  Namespaces
                </span>
                <span className="font-semibold">
                  {constraint.match.namespaces?.length || "All"}
                </span>
              </div>
            </CardContent>
          </Card>

          {/* Quick Links */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Related</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Button variant="outline" className="w-full justify-start" asChild>
                <Link to={`/compliance/templates/${constraint.templateName}`}>
                  <FileText className="h-4 w-4 mr-2" />
                  View Template
                </Link>
              </Button>
              <Button variant="outline" className="w-full justify-start" asChild>
                <Link to={`/compliance/constraints?kind=${constraint.kind}`}>
                  <Shield className="h-4 w-4 mr-2" />
                  Similar Constraints
                </Link>
              </Button>
              {constraint.violationCount > 0 && (
                <Button
                  variant="outline"
                  className="w-full justify-start"
                  asChild
                >
                  <Link
                    to={`/compliance/violations?constraint=${constraint.kind}/${constraint.name}`}
                  >
                    <AlertTriangle className="h-4 w-4 mr-2" />
                    View Violations
                  </Link>
                </Button>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function ConstraintDetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96" />
      </div>
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <Skeleton className="h-6 w-40" />
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-3">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="space-y-2">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-6 w-32" />
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <Skeleton className="h-6 w-32" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-32 w-full" />
            </CardContent>
          </Card>
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-24" />
          </CardHeader>
          <CardContent className="space-y-4">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export default ConstraintDetailPage;
