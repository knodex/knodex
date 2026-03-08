// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Repository section for project settings
 * Manages repository configurations with React Query
 */
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, GitBranch, ShieldAlert } from "lucide-react";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AxiosError } from "axios";
import { Button } from "@/components/ui/button";
import { RepositoryList } from "./RepositoryList";
import { RepositoryForm } from "./RepositoryForm";
import { DeleteRepositoryDialog } from "./DeleteRepositoryDialog";
import {
  listRepositories,
  createRepository,
  updateRepository,
  deleteRepository,
  testConnection,
} from "@/api/repository";
import type {
  RepositoryConfig,
  CreateRepositoryRequest,
  TestConnectionRequest,
  TestConnectionResponse,
  UpdateRepositoryRequest,
} from "@/types/repository";
import { useProjects } from "@/hooks/useProjects";

interface RepositorySectionProps {
  canManage?: boolean;
  isLoadingPermission?: boolean;
  isErrorPermission?: boolean;
}

export function RepositorySection({
  canManage = false,
  isLoadingPermission = false,
  isErrorPermission = false,
}: RepositorySectionProps) {
  const [showForm, setShowForm] = useState(false);
  const [editingRepo, setEditingRepo] = useState<RepositoryConfig | null>(null);
  const [deletingRepo, setDeletingRepo] = useState<RepositoryConfig | null>(null);

  const queryClient = useQueryClient();

  // Fetch repositories
  const {
    data: repositoriesData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["repositories"],
    queryFn: () => listRepositories(),
  });

  // Fetch projects for the form
  const { data: projectsData } = useProjects();
  const projects = projectsData?.items ?? [];

  // Create repository mutation
  const createMutation = useMutation({
    mutationFn: (data: CreateRepositoryRequest) => createRepository(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      setShowForm(false);
      setEditingRepo(null);
    },
  });

  // Update repository mutation
  const updateMutation = useMutation({
    mutationFn: ({ repoId, data }: { repoId: string; data: UpdateRepositoryRequest }) =>
      updateRepository(repoId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      setShowForm(false);
      setEditingRepo(null);
    },
  });

  // Delete repository mutation
  const deleteMutation = useMutation({
    mutationFn: (repoId: string) => deleteRepository(repoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      setDeletingRepo(null);
    },
  });

  // Handle form submit
  const handleFormSubmit = async (data: CreateRepositoryRequest) => {
    if (editingRepo) {
      // For updates, use the update endpoint with limited fields
      await updateMutation.mutateAsync({
        repoId: editingRepo.id,
        data: {
          name: data.name,
          defaultBranch: data.defaultBranch,
        },
      });
    } else {
      await createMutation.mutateAsync(data);
    }
  };

  // Handle test connection with inline credentials
  const handleTestConnection = async (data: TestConnectionRequest): Promise<TestConnectionResponse> => {
    return await testConnection(data);
  };

  // Handle edit
  const handleEdit = (repo: RepositoryConfig) => {
    setEditingRepo(repo);
    setShowForm(true);
  };

  // Handle delete
  const handleDelete = (repoId: string) => {
    const repo = repositoriesData?.items.find((r) => r.id === repoId);
    if (repo) {
      setDeletingRepo(repo);
    }
  };

  // Handle cancel
  const handleCancel = () => {
    setShowForm(false);
    setEditingRepo(null);
  };

  const repositories = repositoriesData?.items || [];

  // Check if error is a 403 Forbidden
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  // If user doesn't have permission, show access denied message
  if (is403Error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            Repositories
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12 text-muted-foreground">
            <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
            <p className="text-sm font-medium">Access Denied</p>
            <p className="text-xs mt-2">
              You do not have permission to view repository configurations.
              Contact an administrator if you need access.
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (showForm) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            {editingRepo ? "Edit Repository" : "Add Repository"}
          </CardTitle>
          <CardDescription>
            {editingRepo
              ? "Update repository configuration for GitOps deployments"
              : "Configure a repository with SSH, HTTPS, or GitHub App authentication"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <RepositoryForm
            initialData={editingRepo || undefined}
            projects={projects}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            onTestConnection={handleTestConnection}
            isLoading={createMutation.isPending || updateMutation.isPending}
          />
        </CardContent>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <GitBranch className="h-5 w-5" />
                Repositories
              </CardTitle>
              <CardDescription className="mt-1">
                {repositories.length} repository configuration{repositories.length !== 1 ? "s" : ""}
              </CardDescription>
            </div>
            {isLoadingPermission ? (
              <Skeleton className="h-9 w-36" />
            ) : (isErrorPermission || canManage) ? (
              <Button onClick={() => setShowForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Add Repository
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {error && !is403Error && (
            <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load repositories: {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          )}

          <RepositoryList
            repositories={repositories}
            onEdit={(isLoadingPermission || isErrorPermission || canManage) ? handleEdit : undefined}
            onDelete={(isLoadingPermission || isErrorPermission || canManage) ? handleDelete : undefined}
            canManage={isLoadingPermission || isErrorPermission || canManage}
            isLoading={isLoading}
          />
        </CardContent>
      </Card>

      {deletingRepo && (
        <DeleteRepositoryDialog
          repository={deletingRepo}
          onConfirm={() => deleteMutation.mutateAsync(deletingRepo.id)}
          onCancel={() => setDeletingRepo(null)}
          isOpen={!!deletingRepo}
          isDeleting={deleteMutation.isPending}
          error={deleteMutation.error}
        />
      )}
    </>
  );
}
