// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import { FileText, ExternalLink, Layers, AlertCircle, Plus } from "@/lib/icons";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useConstraintTemplate, useConstraints } from "@/hooks/useCompliance";
import { PageHeader } from "@/components/layout/PageHeader";
import { RegoCodeViewer } from "./RegoCodeViewer";
import { ErrorState } from "./ErrorState";
import { CreateConstraintDialog } from "./CreateConstraintDialog";
import { formatDate } from "@/lib/date";
import { getEnforcementClassName } from "@/types/compliance";

/**
 * ConstraintTemplate detail page
 * AC-TPL-04: Shows full template info including description, kind, parameters
 * AC-TPL-05: List of constraints using this template with links
 * AC-TPL-06: Rego code displayed in syntax-highlighted code block
 * AC-TPL-07: Back to templates list
 */
export function ConstraintTemplateDetailPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);

  const {
    data: template,
    isLoading,
    isError,
    error,
    refetch,
    isRefetching,
  } = useConstraintTemplate(name || "");

  // Fetch constraints using this template's kind
  const { data: constraintsData } = useConstraints(
    template ? { kind: template.kind, pageSize: 100 } : undefined
  );

  // All hooks must be before any early returns
  const handleConstraintCreated = useCallback((constraintName: string) => {
    if (template) {
      navigate(`/compliance/constraints/${template.kind}/${constraintName}`);
    }
  }, [template, navigate]);

  const constraints = useMemo(() => constraintsData?.items || [], [constraintsData?.items]);

  const handleOpenCreateDialog = useCallback(() => setIsCreateDialogOpen(true), []);

  const breadcrumbs = useMemo(() => [
    { label: "Compliance", href: "/compliance" },
    { label: "Templates", href: "/compliance/templates" },
    { label: template?.name || name || "" },
  ], [template?.name, name]);

  const parametersJson = useMemo(
    () => template?.parameters ? JSON.stringify(template.parameters, null, 2) : null,
    [template]
  );

  const labelEntries = useMemo(
    () => template?.labels ? Object.entries(template.labels) : [],
    [template]
  );

  if (!name) {
    return (
      <ErrorState
        message="Invalid Template"
        details="No template name provided in the URL"
      />
    );
  }

  if (isLoading) {
    return <TemplateDetailSkeleton />;
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Constraint Template"
          breadcrumbs={[
            { label: "Compliance", href: "/compliance" },
            { label: "Templates", href: "/compliance/templates" },
            { label: name },
          ]}
        />
        <ErrorState
          message="Failed to load template"
          details={error instanceof Error ? error.message : "Unknown error"}
          onRetry={() => refetch()}
          isRetrying={isRefetching}
        />
      </div>
    );
  }

  if (!template) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Template Not Found"
          breadcrumbs={[
            { label: "Compliance", href: "/compliance" },
            { label: "Templates", href: "/compliance/templates" },
            { label: name },
          ]}
        />
        <ErrorState
          message="Template Not Found"
          details={`ConstraintTemplate "${name}" does not exist`}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={template.name}
        subtitle={template.description || "No description available"}
        breadcrumbs={breadcrumbs}
        actions={
          <div className="flex items-center gap-2">
            <Button
              onClick={handleOpenCreateDialog}
              data-testid="create-constraint-btn"
            >
              <Plus className="h-4 w-4 mr-2" />
              Create Constraint
            </Button>
            <Button variant="outline" asChild>
              <Link to="/compliance/templates">Back to Templates</Link>
            </Button>
          </div>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Template Info */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileText className="h-5 w-5 text-muted-foreground" />
                Template Details
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Constraint Kind
                  </p>
                  <Link
                    to={`/compliance/constraints?kind=${template.kind}`}
                    className="text-lg font-semibold text-primary hover:underline flex items-center gap-1"
                  >
                    {template.kind}
                    <ExternalLink className="h-4 w-4" />
                  </Link>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Created
                  </p>
                  <p className="text-lg font-semibold">
                    {formatDate(template.createdAt)}
                  </p>
                </div>
              </div>

              {template.description && (
                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-1">
                    Description
                  </p>
                  <p className="text-sm">{template.description}</p>
                </div>
              )}

              {template.labels && Object.keys(template.labels).length > 0 && (
                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-2">
                    Labels
                  </p>
                  <div className="flex flex-wrap gap-1.5">
                    {labelEntries.map(([key, value]) => (
                      <Badge key={key} variant="secondary" className="text-xs">
                        {key}: {value}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Rego Policy */}
          {template.rego && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <FileText className="h-5 w-5 text-muted-foreground" />
                  Rego Policy
                </CardTitle>
              </CardHeader>
              <CardContent>
                <RegoCodeViewer
                  code={template.rego}
                  title={`${template.name}.rego`}
                  maxHeight="500px"
                />
              </CardContent>
            </Card>
          )}

          {/* Parameters Schema */}
          {template.parameters &&
            Object.keys(template.parameters).length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Layers className="h-5 w-5 text-muted-foreground" />
                    Parameters Schema
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <pre className="p-4 bg-muted rounded-lg overflow-x-auto text-sm">
                    <code>{parametersJson}</code>
                  </pre>
                </CardContent>
              </Card>
            )}
        </div>

        {/* Sidebar - Constraints using this template */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <AlertCircle className="h-5 w-5 text-muted-foreground" />
                Constraints Using This Template
                <Badge variant="secondary" className="ml-auto">
                  {constraints.length}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {constraints.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-4">
                  No constraints are using this template yet.
                </p>
              ) : (
                <div className="space-y-2">
                  {constraints.map((constraint) => (
                    <Link
                      key={`${constraint.kind}-${constraint.name}`}
                      to={`/compliance/constraints/${constraint.kind}/${constraint.name}`}
                      className="block p-3 rounded-lg border hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm truncate">
                          {constraint.name}
                        </span>
                        <Badge
                          variant="outline"
                          className={getEnforcementClassName(constraint.enforcementAction)}
                        >
                          {constraint.enforcementAction}
                        </Badge>
                      </div>
                      {constraint.violationCount > 0 && (
                        <p className="text-xs text-destructive mt-1">
                          {constraint.violationCount} violation
                          {constraint.violationCount !== 1 ? "s" : ""}
                        </p>
                      )}
                    </Link>
                  ))}
                </div>
              )}

              {constraints.length > 0 && (
                <Button variant="outline" className="w-full mt-4" asChild>
                  <Link to={`/compliance/constraints?kind=${template.kind}`}>
                    View All Constraints
                  </Link>
                </Button>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Create Constraint Dialog */}
      <CreateConstraintDialog
        template={template}
        open={isCreateDialogOpen}
        onOpenChange={setIsCreateDialogOpen}
        onSuccess={handleConstraintCreated}
      />
    </div>
  );
}

function TemplateDetailSkeleton() {
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
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-6 w-32" />
                </div>
                <div className="space-y-2">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-6 w-32" />
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <Skeleton className="h-6 w-32" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-64 w-full" />
            </CardContent>
          </Card>
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-48" />
          </CardHeader>
          <CardContent className="space-y-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export default ConstraintTemplateDetailPage;
