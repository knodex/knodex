// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { ShieldAlert } from "@/lib/icons";
import { useNavigate } from "react-router-dom";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { useCanI } from "@/hooks/useCanI";
import { useProjects, useCreateProject, useDeleteProject } from "@/hooks/useProjects";
import { ProjectList, CreateProjectModal, DeleteProjectDialog } from "@/components/projects";
import { PageHeader } from "@/components/layout/PageHeader";
import type { Project, CreateProjectRequest } from "@/types/project";
import { toUserFriendlyError } from "@/lib/errors";

export function ProjectsSettings() {
  const { allowed: canCreateProject, isLoading: isLoadingCreate, isError: isErrorCreate } = useCanI('projects', 'create');
  const { allowed: canDeleteProject, isLoading: isLoadingDelete, isError: isErrorDelete } = useCanI('projects', 'delete');
  const navigate = useNavigate();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [deletingProject, setDeletingProject] = useState<Project | null>(null);

  const { data: projectsData, isLoading, error } = useProjects();
  const createMutation = useCreateProject();
  const deleteMutation = useDeleteProject();

  const projects = projectsData?.items || [];

  const handleCreateSubmit = async (data: CreateProjectRequest) => {
    try {
      await createMutation.mutateAsync(data);
      toast.success(`Project "${data.name}" created successfully`);
      setShowCreateModal(false);
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      let errorMessage = responseData?.message || (err as Error).message || "Failed to create project";
      if (responseData?.details && Object.keys(responseData.details).length > 0) {
        const detailMessages = Object.values(responseData.details).map(msg => toUserFriendlyError(msg)).join("; ");
        errorMessage = detailMessages;
      } else {
        errorMessage = toUserFriendlyError(errorMessage);
      }
      toast.error(errorMessage);
    }
  };

  const handleDelete = (projectName: string) => {
    const project = projects.find((p) => p.name === projectName);
    if (project) setDeletingProject(project);
  };

  const handleProjectClick = (project: Project) => {
    navigate(`/projects/${project.name}`);
  };

  const handleEdit = (project: Project) => {
    navigate(`/projects/${project.name}`);
  };

  // 403 Access Denied
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  if (is403Error) {
    return (
      <section className="flex flex-col items-center justify-center py-16 text-center">
        <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-semibold">Access Denied</h2>
        <p className="text-sm text-muted-foreground mt-1">
          You do not have permission to view projects.
        </p>
      </section>
    );
  }

  return (
    <section className="space-y-6">
      <PageHeader title="Projects" />

      {error && !is403Error && (
        <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
          <p className="text-sm text-destructive">
            Failed to load projects: {error instanceof Error ? error.message : "Unknown error"}
          </p>
        </div>
      )}

      <ProjectList
        projects={projects}
        onEdit={(isLoadingCreate || isErrorCreate || canCreateProject) ? handleEdit : undefined}
        onDelete={(isLoadingDelete || isErrorDelete || canDeleteProject) ? handleDelete : undefined}
        onClick={handleProjectClick}
        onCreate={() => setShowCreateModal(true)}
        canManage={isLoadingCreate || isLoadingDelete || isErrorCreate || isErrorDelete || canCreateProject || canDeleteProject}
        isLoading={isLoading}
      />

      <CreateProjectModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={handleCreateSubmit}
        isLoading={createMutation.isPending}
      />

      {deletingProject && (
        <DeleteProjectDialog
          project={deletingProject}
          isOpen={!!deletingProject}
          onConfirm={async () => {
            try {
              await deleteMutation.mutateAsync(deletingProject.name);
              toast.success(`Project "${deletingProject.name}" deleted`);
            } catch (err) {
              const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
              const responseData = axiosError?.response?.data;
              const errorMessage = toUserFriendlyError(
                responseData?.message || (err as Error).message || "Failed to delete project"
              );
              toast.error(errorMessage);
            } finally {
              setDeletingProject(null);
            }
          }}
          onCancel={() => setDeletingProject(null)}
          isDeleting={deleteMutation.isPending}
          error={deleteMutation.error}
        />
      )}
    </section>
  );
}

export default ProjectsSettings;
