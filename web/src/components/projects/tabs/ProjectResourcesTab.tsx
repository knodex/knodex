// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Copy, Check } from "@/lib/icons";
import { useProjectResources } from "@/hooks/useProjects";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { TableLoadingSkeleton } from "@/components/compliance/TableLoadingSkeleton";
import { StatusIndicator } from "@/components/ui/status-indicator";
import type { StatusIndicatorStatus } from "@/components/ui/status-indicator";
import { cn } from "@/lib/utils";
import type { Project, AggregatedResource } from "@/types/project";

interface ProjectResourcesTabProps {
  project: Project;
  active?: boolean;
}

const RESOURCE_COLUMNS = [
  { header: "Name" },
  { header: "Kind", width: "120px" },
  { header: "Cluster", width: "160px" },
  { header: "Namespace", width: "160px" },
  { header: "Status", width: "120px" },
  { header: "Age", width: "80px" },
];

function mapStatusToIndicator(status: string): StatusIndicatorStatus {
  switch (status) {
    case "Ready":
    case "Active":
      return "healthy";
    case "NotReady":
    case "False":
      return "error";
    default:
      return "unknown";
  }
}

function resourceToYaml(resource: AggregatedResource): string {
  return [
    `name: ${resource.name}`,
    `kind: ${resource.kind}`,
    `cluster: ${resource.cluster}`,
    `namespace: ${resource.namespace}`,
    `status: ${resource.status}`,
    `age: ${resource.age}`,
  ].join("\n");
}

export function ProjectResourcesTab({ project, active = true }: ProjectResourcesTabProps) {
  const [activeKind, setActiveKind] = useState<"Certificate" | "Ingress">(
    "Certificate"
  );
  const [selectedResource, setSelectedResource] =
    useState<AggregatedResource | null>(null);
  const [copied, setCopied] = useState(false);

  const { data, isLoading, error } = useProjectResources(
    project.name,
    activeKind,
    active
  );

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access may be blocked in non-HTTPS or restricted environments
    }
  };

  const unreachableClusters = Object.entries(data?.clusterStatus ?? {}).filter(
    ([, status]) => status.phase === "unreachable"
  );

  return (
    <div className="space-y-4">
      {/* Kind selector */}
      <div className="flex gap-2">
        {(["Certificate", "Ingress"] as const).map((k) => (
          <Button
            key={k}
            size="sm"
            variant={activeKind === k ? "default" : "outline"}
            onClick={() => {
              setActiveKind(k);
              setSelectedResource(null);
            }}
          >
            {k}
          </Button>
        ))}
      </div>

      {/* Unreachable cluster banners */}
      {unreachableClusters.map(([cluster, status]) => (
        <Alert key={cluster} variant="warning" showIcon>
          <AlertDescription>
            <strong>{cluster}</strong>: unreachable — showing last known data
            {status.message && ` (${status.message})`}
          </AlertDescription>
        </Alert>
      ))}

      {/* Loading state */}
      {isLoading && (
        <Card>
          <CardContent className="p-0">
            <TableLoadingSkeleton columns={RESOURCE_COLUMNS} rows={5} />
          </CardContent>
        </Card>
      )}

      {/* Error state */}
      {error && !isLoading && (
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <p className="text-lg font-medium text-destructive">
                Failed to load resources
              </p>
              <p className="text-sm text-muted-foreground mt-2">
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty state */}
      {data && data.items.length === 0 && !isLoading && (
        <Card>
          <CardContent className="py-12">
            <div className="text-center text-muted-foreground">
              No {activeKind} resources found across your clusters
            </div>
          </CardContent>
        </Card>
      )}

      {/* Data table */}
      {data && data.items.length > 0 && !isLoading && (
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  {RESOURCE_COLUMNS.map((col) => (
                    <TableHead
                      key={col.header}
                      style={col.width ? { width: col.width } : undefined}
                    >
                      {col.header}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((resource) => (
                  <TableRow
                    key={`${resource.cluster}/${resource.namespace}/${resource.name}`}
                    className={cn(
                      "cursor-pointer",
                      selectedResource?.name === resource.name &&
                        selectedResource?.cluster === resource.cluster &&
                        selectedResource?.namespace === resource.namespace
                        ? "bg-muted"
                        : "hover:bg-muted/50"
                    )}
                    onClick={() => setSelectedResource(resource)}
                  >
                    <TableCell className="font-medium">
                      {resource.name}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{resource.kind}</Badge>
                    </TableCell>
                    <TableCell>{resource.cluster}</TableCell>
                    <TableCell>{resource.namespace}</TableCell>
                    <TableCell>
                      <span className="flex items-center gap-2">
                        <StatusIndicator
                          status={mapStatusToIndicator(resource.status)}
                        />
                        {resource.status}
                      </span>
                    </TableCell>
                    <TableCell>{resource.age}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Resource detail side panel */}
      <Sheet
        open={selectedResource !== null}
        onOpenChange={(open) => !open && setSelectedResource(null)}
      >
        <SheetContent className="w-3/4 max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle>
              {selectedResource?.name} ({selectedResource?.kind})
            </SheetTitle>
            <SheetDescription>
              Resource details from cluster {selectedResource?.cluster}
            </SheetDescription>
          </SheetHeader>
          <div className="mt-4 space-y-4">
            <div className="flex justify-end">
              <Button
                size="sm"
                variant="outline"
                onClick={() =>
                  selectedResource &&
                  handleCopy(resourceToYaml(selectedResource))
                }
              >
                {copied ? (
                  <Check className="h-4 w-4 mr-1" />
                ) : (
                  <Copy className="h-4 w-4 mr-1" />
                )}
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <pre className="text-xs font-mono overflow-auto rounded-md bg-muted p-4">
              {selectedResource && resourceToYaml(selectedResource)}
            </pre>
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
