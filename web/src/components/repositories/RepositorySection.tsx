// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ShieldAlert } from "@/lib/icons";
import { AxiosError } from "axios";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
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
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingRepo, setEditingRepo] = useState<RepositoryConfig | null>(null);
  const [deletingRepo, setDeletingRepo] = useState<RepositoryConfig | null>(null);

  const queryClient = useQueryClient();

  const {
    data: repositoriesData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["repositories"],
    queryFn: () => listRepositories(),
  });

  const { data: projectsData } = useProjects();
  const projects = projectsData?.items ?? [];

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setEditingRepo(null);
  }, []);

  const createMutation = useMutation({
    mutationFn: (data: CreateRepositoryRequest) => createRepository(data),
    onSuccess: (_, data) => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      closeDialog();
      toast.success(`Repository "${data.name}" added successfully`);
    },
    onError: (err: Error) => {
      toast.error(err.message || "Failed to add repository");
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ repoId, data }: { repoId: string; data: UpdateRepositoryRequest }) =>
      updateRepository(repoId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      closeDialog();
      toast.success("Repository updated successfully");
    },
    onError: (err: Error) => {
      toast.error(err.message || "Failed to update repository");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (repoId: string) => deleteRepository(repoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
      toast.success(`Repository "${deletingRepo?.name}" deleted`);
      setDeletingRepo(null);
    },
    onError: (err: Error) => {
      toast.error(err.message || "Failed to delete repository");
    },
  });

  const handleFormSubmit = async (data: CreateRepositoryRequest) => {
    if (editingRepo) {
      await updateMutation.mutateAsync({
        repoId: editingRepo.id,
        data: { name: data.name, defaultBranch: data.defaultBranch },
      });
    } else {
      await createMutation.mutateAsync(data);
    }
  };

  const handleTestConnection = async (data: TestConnectionRequest): Promise<TestConnectionResponse> => {
    return await testConnection(data);
  };

  const handleEdit = (repo: RepositoryConfig) => {
    setEditingRepo(repo);
    setDialogOpen(true);
  };

  const handleDelete = (repoId: string) => {
    const repo = repositoriesData?.items.find((r) => r.id === repoId);
    if (repo) setDeletingRepo(repo);
  };

  const handleDialogOpenChange = useCallback((open: boolean) => {
    if (!open) {
      closeDialog();
    } else {
      setDialogOpen(true);
    }
  }, [closeDialog]);

  const repositories = repositoriesData?.items || [];
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  if (is403Error) {
    return (
      <section className="flex flex-col items-center justify-center py-16 text-center">
        <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-semibold">Access Denied</h2>
        <p className="text-sm text-muted-foreground mt-1">
          You do not have permission to view repositories.
        </p>
      </section>
    );
  }

  return (
    <>
      {error && !is403Error && (
        <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
          <p className="text-sm text-destructive">
            Failed to load repositories: {error instanceof Error ? error.message : "Unknown error"}
          </p>
        </div>
      )}

      <RepositoryList
        repositories={repositories}
        onEdit={(isLoadingPermission || isErrorPermission || canManage) ? handleEdit : undefined}
        onDelete={(isLoadingPermission || isErrorPermission || canManage) ? handleDelete : undefined}
        onCreate={() => setDialogOpen(true)}
        canManage={isLoadingPermission || isErrorPermission || canManage}
        isLoadingPermission={isLoadingPermission}
        isLoading={isLoading}
      />

      <Dialog open={dialogOpen} onOpenChange={handleDialogOpenChange}>
        <DialogContent className="sm:max-w-[700px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingRepo ? "Edit Repository" : "Add Repository"}</DialogTitle>
            <DialogDescription>
              {editingRepo
                ? "Update the repository configuration."
                : "Connect a Git repository for deployment tracking."}
            </DialogDescription>
          </DialogHeader>
          <RepositoryForm
            initialData={editingRepo || undefined}
            projects={projects}
            onSubmit={handleFormSubmit}
            onCancel={closeDialog}
            onTestConnection={handleTestConnection}
            isLoading={createMutation.isPending || updateMutation.isPending}
          />
        </DialogContent>
      </Dialog>

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
