// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Users, ArrowLeft, Plus, Shield, ShieldAlert } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { useCanI } from "@/hooks/useCanI";
import { useProjects, useCreateProject, useDeleteProject } from "@/hooks/useProjects";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ProjectList, ProjectForm, DeleteProjectDialog } from "@/components/projects";
import type { Project, CreateProjectRequest } from "@/types/project";
import { toUserFriendlyError } from "@/lib/errors";

/**
 * Projects Settings page - Manage RBAC projects and policies
 *
 * Access control: Accessible to all authenticated users. Authorization is handled
 * by the API via Casbin permission checks. If the API returns 403, the page
 * displays an Access Denied message. This follows the ArgoCD pattern of pure
 * Casbin permission checks at the API layer.
 *
 * Provides comprehensive UI for project management following ArgoCD AppProject patterns:
 * - List all configured projects with role/repo counts
 * - Create new projects with configuration (requires projects:create permission)
 * - Edit existing project configurations
 * - Delete projects (with confirmation, requires projects:delete permission)
 * - Navigate to project detail view for full management
 */
export function ProjectsSettings() {
  // Real-time permission checks via backend Casbin enforcer (ArgoCD-aligned pattern)
  // Uses /api/v1/account/can-i endpoint for server-side authorization evaluation
  const { allowed: canCreateProject, isLoading: isLoadingCreate, isError: isErrorCreate } = useCanI('projects', 'create');
  const { allowed: canDeleteProject, isLoading: isLoadingDelete, isError: isErrorDelete } = useCanI('projects', 'delete');
  const navigate = useNavigate();

  // Local state for form and dialogs
  const [showForm, setShowForm] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [deletingProject, setDeletingProject] = useState<Project | null>(null);

  // Data fetching
  const { data: projectsData, isLoading, error } = useProjects();
  const createMutation = useCreateProject();
  const deleteMutation = useDeleteProject();

  const projects = projectsData?.items || [];

  // Handle create/edit form submit
  const handleFormSubmit = async (data: CreateProjectRequest) => {
    if (editingProject) {
      // For editing, navigate to project detail page
      // Updates are handled there with more granular controls
      navigate(`/settings/projects/${editingProject.name}`);
    } else {
      try {
        await createMutation.mutateAsync(data);
        toast.success(`Project "${data.name}" created successfully`);
        setShowForm(false);
      } catch (err) {
        // Extract error message from API response or use fallback
        // API returns { code, message, details } structure
        const axiosError = err as AxiosError<{
          message?: string;
          details?: Record<string, string>;
        }>;
        const responseData = axiosError?.response?.data;

        // Format validation details into readable message
        let errorMessage = responseData?.message || (err as Error).message || "Failed to create project";
        if (responseData?.details && Object.keys(responseData.details).length > 0) {
          // Transform each detail into user-friendly message
          const detailMessages = Object.values(responseData.details)
            .map(msg => toUserFriendlyError(msg))
            .join("; ");
          errorMessage = detailMessages;
        } else {
          // Transform the main message
          errorMessage = toUserFriendlyError(errorMessage);
        }

        toast.error(errorMessage);
      }
    }
  };

  // Handle delete
  const handleDelete = (projectName: string) => {
    const project = projects.find((p) => p.name === projectName);
    if (project) {
      setDeletingProject(project);
    }
  };

  // Handle project click - navigate to detail
  const handleProjectClick = (project: Project) => {
    navigate(`/settings/projects/${project.name}`);
  };

  // Handle edit click
  const handleEdit = (project: Project) => {
    // Navigate to detail page for full editing
    navigate(`/settings/projects/${project.name}`);
  };

  // Handle cancel form
  const handleCancel = () => {
    setShowForm(false);
    setEditingProject(null);
  };

  // Render form view
  if (showForm) {
    return (
      <div className="py-6">
        <div className="mb-8">
          <Link
            to="/settings/projects"
            onClick={(e) => {
              e.preventDefault();
              handleCancel();
            }}
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Projects
          </Link>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Shield className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">
                {editingProject ? "Edit Project" : "Create Project"}
              </h2>
              <p className="text-muted-foreground">
                {editingProject
                  ? "Update project configuration"
                  : "Define a new RBAC project for deployment boundaries"}
              </p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <ProjectForm
              initialData={editingProject || undefined}
              onSubmit={handleFormSubmit}
              onCancel={handleCancel}
              isLoading={createMutation.isPending}
            />
          </CardContent>
        </Card>
      </div>
    );
  }

  // Check if error is a 403 Forbidden (access denied)
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  // Render access denied view for 403 errors
  if (is403Error) {
    return (
      <div className="py-6">
        <div className="mb-8">
          <Link
            to="/settings"
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Settings
          </Link>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Users className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">Projects</h2>
              <p className="text-muted-foreground">
                Manage RBAC projects, roles, and policies
              </p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to view projects.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Render list view
  return (
    <div className="py-6">
      {/* Header with back navigation */}
      <div className="mb-8">
        <Link
          to="/settings"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Settings
        </Link>
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Users className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">Projects</h2>
            <p className="text-muted-foreground">
              Manage RBAC projects, roles, and policies
            </p>
          </div>
        </div>
      </div>

      {/* Project management section */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Shield className="h-5 w-5" />
                Projects
              </CardTitle>
              <CardDescription className="mt-1">
                {projects.length} project{projects.length !== 1 ? "s" : ""} configured
              </CardDescription>
            </div>
            {isLoadingCreate ? (
              <Skeleton className="h-9 w-36" />
            ) : (isErrorCreate || canCreateProject) ? (
              <Button onClick={() => setShowForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Create Project
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {error && (
            <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load projects:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          )}

          <ProjectList
            projects={projects}
            onEdit={(isLoadingCreate || isErrorCreate || canCreateProject) ? handleEdit : undefined}
            onDelete={(isLoadingDelete || isErrorDelete || canDeleteProject) ? handleDelete : undefined}
            onClick={handleProjectClick}
            canManage={isLoadingCreate || isLoadingDelete || isErrorCreate || isErrorDelete || canCreateProject || canDeleteProject}
            isLoading={isLoading}
          />
        </CardContent>
      </Card>

      {/* Delete confirmation dialog */}
      {deletingProject && (
        <DeleteProjectDialog
          project={deletingProject}
          isOpen={!!deletingProject}
          onConfirm={async () => {
            try {
              await deleteMutation.mutateAsync(deletingProject.name);
              toast.success(`Project "${deletingProject.name}" deleted`);
            } catch (err) {
              const axiosError = err as AxiosError<{
                message?: string;
                details?: Record<string, string>;
              }>;
              const responseData = axiosError?.response?.data;
              const errorMessage = toUserFriendlyError(
                responseData?.message || (err as Error).message || "Failed to delete project"
              );
              toast.error(errorMessage);
            } finally {
              // Always close dialog - user sees result via toast
              setDeletingProject(null);
            }
          }}
          onCancel={() => setDeletingProject(null)}
          isDeleting={deleteMutation.isPending}
          error={deleteMutation.error}
        />
      )}
    </div>
  );
}

export default ProjectsSettings;
