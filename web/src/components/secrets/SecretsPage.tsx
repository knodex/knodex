// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useEffect, useMemo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { KeyRound, Plus, ShieldAlert, Trash2 } from "lucide-react";
import { useSecretList } from "@/hooks/useSecrets";
import { useProjects } from "@/hooks/useProjects";
import { useCanI } from "@/hooks/useCanI";
import { formatDistanceToNow } from "@/lib/date";
import { getSafeErrorMessage } from "@/lib/errors";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { CreateSecretDialog } from "./CreateSecretDialog";
import { DeleteSecretDialog } from "./DeleteSecretDialog";

export function SecretsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedProject = searchParams.get("project") ?? "";
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ name: string; namespace: string } | null>(null);

  // Update project selection via URL search params (single source of truth)
  const setSelectedProject = (project: string) => {
    setSearchParams(project ? { project } : {}, { replace: true });
  };

  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData?.items]);

  const { allowed: canGet, isLoading: permLoading } = useCanI("secrets", "get", selectedProject || "-");
  const { allowed: canCreate } = useCanI("secrets", "create", selectedProject || "-");
  const { allowed: canDelete } = useCanI("secrets", "delete", selectedProject || "-");

  const { data, isLoading, isError, error, refetch } = useSecretList(selectedProject);

  // Auto-select first project if only one available and no URL param
  useEffect(() => {
    if (!selectedProject && projects.length === 1) {
      setSearchParams({ project: projects[0].name }, { replace: true });
    }
  }, [projects, selectedProject, setSearchParams]);

  // Access denied state
  if (!permLoading && canGet === false) {
    return (
      <section className="flex flex-col items-center justify-center py-16 text-center">
        <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-semibold">Access Denied</h2>
        <p className="text-sm text-muted-foreground mt-1">
          You don't have permission to view secrets.
        </p>
      </section>
    );
  }

  return (
    <section className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Secrets</h1>
          <p className="text-sm text-muted-foreground">
            Manage Kubernetes secrets for your projects.
          </p>
        </div>
        <div className="flex items-center gap-3">
          {/* Project selector */}
          <Select value={selectedProject} onValueChange={setSelectedProject}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Select project" />
            </SelectTrigger>
            <SelectContent>
              {projectsLoading ? (
                selectedProject ? (
                  <SelectItem value={selectedProject}>{selectedProject}</SelectItem>
                ) : (
                  <SelectItem value="_loading" disabled>Loading...</SelectItem>
                )
              ) : projects.length === 0 ? (
                <SelectItem value="_none" disabled>No projects</SelectItem>
              ) : (
                projects.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.name}
                  </SelectItem>
                ))
              )}
            </SelectContent>
          </Select>

          {canCreate && selectedProject && (
            <Button onClick={() => setIsCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Create Secret
            </Button>
          )}
        </div>
      </div>

      {/* Content */}
      {!selectedProject ? (
        <EmptyState message="Select a project to view its secrets." />
      ) : isLoading || permLoading ? (
        <TableSkeleton />
      ) : isError ? (
        <div className="flex flex-col items-center justify-center py-12">
          <Alert variant="destructive" showIcon onRetry={() => refetch()} className="max-w-md">
            <AlertTitle>Failed to load secrets</AlertTitle>
            <AlertDescription>{getSafeErrorMessage(error)}</AlertDescription>
          </Alert>
        </div>
      ) : !data || data.items.length === 0 ? (
        <EmptyState message="No secrets found in this project." />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Namespace</TableHead>
              <TableHead>Keys</TableHead>
              <TableHead>Created At</TableHead>
              {canDelete && <TableHead className="w-10" />}
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.items.map((secret) => (
              <TableRow
                key={`${secret.namespace}/${secret.name}`}
                className="cursor-pointer"
                onClick={() =>
                  navigate(
                    `/secrets/${encodeURIComponent(secret.namespace)}/${encodeURIComponent(secret.name)}?project=${encodeURIComponent(selectedProject)}`
                  )
                }
              >
                <TableCell className="font-medium">{secret.name}</TableCell>
                <TableCell>{secret.namespace}</TableCell>
                <TableCell>
                  <span className="text-muted-foreground">
                    {secret.keys.length > 0 ? secret.keys.join(", ") : "—"}
                  </span>
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {formatDistanceToNow(secret.createdAt)}
                </TableCell>
                {canDelete && (
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeleteTarget({ name: secret.name, namespace: secret.namespace });
                      }}
                      aria-label={`Delete ${secret.name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {/* Create Secret Dialog */}
      {selectedProject && (
        <CreateSecretDialog
          open={isCreateOpen}
          onOpenChange={setIsCreateOpen}
          project={selectedProject}
        />
      )}

      {/* Delete Secret Dialog */}
      {deleteTarget && selectedProject && (
        <DeleteSecretDialog
          open={!!deleteTarget}
          onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
          secretName={deleteTarget.name}
          secretNamespace={deleteTarget.namespace}
          project={selectedProject}
          navigateOnDelete={false}
        />
      )}
    </section>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <KeyRound className="h-12 w-12 text-muted-foreground mb-4" />
      <p className="text-sm text-muted-foreground">{message}</p>
    </div>
  );
}

function TableSkeleton() {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Namespace</TableHead>
          <TableHead>Keys</TableHead>
          <TableHead>Created At</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {Array.from({ length: 5 }).map((_, i) => (
          <TableRow key={i}>
            <TableCell><Skeleton className="h-4 w-32" /></TableCell>
            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
            <TableCell><Skeleton className="h-4 w-40" /></TableCell>
            <TableCell><Skeleton className="h-4 w-20" /></TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
