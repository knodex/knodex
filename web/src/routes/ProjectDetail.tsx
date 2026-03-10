// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { ArrowLeft, Shield, ShieldAlert, Users, Settings, MapPin, Loader2 } from "lucide-react";
import { AxiosError } from "axios";
import { useCanI } from "@/hooks/useCanI";
import { useProject, useUpdateProject } from "@/hooks/useProjects";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { UpdateProjectRequest } from "@/types/project";

// Tab components
import { ProjectOverviewTab } from "@/components/projects/tabs/ProjectOverviewTab";
import { ProjectRolesTab } from "@/components/projects/tabs/ProjectRolesTab";
import { ProjectDestinationsTab } from "@/components/projects/tabs/ProjectDestinationsTab";
/**
 * Project Detail page - View and manage a single project
 *
 * Access control: Accessible to all authenticated users. Authorization is handled
 * by the API via Casbin permission checks. If the API returns 403, the page
 * displays an Access Denied message. This follows the ArgoCD pattern of pure
 * Casbin permission checks at the API layer.
 *
 * Provides tabbed interface for managing:
 * - Overview: Project metadata and summary
 * - Roles: Role definitions and permissions (includes Casbin policies)
 */
export function ProjectDetail() {
  const { name } = useParams<{ name: string }>();
  // Real-time permission check via backend Casbin enforcer
  // Check if user can update this specific project (requires projects:update:projectName)
  const { allowed: canManageProject, isLoading: isLoadingManage, isError: isErrorManage } = useCanI('projects', 'update', name || '-');
  const [activeTab, setActiveTab] = useState("overview");

  // Fetch project data
  const { data: project, isLoading, error } = useProject(name || "");
  const updateMutation = useUpdateProject();

  // Loading state
  if (isLoading) {
    return (
      <div className="py-6">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  // Check if error is a 403 Forbidden (access denied)
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  // 403 Access Denied state
  if (is403Error) {
    return (
      <div className="py-6">
        <div className="mb-8">
          <Link
            to="/settings/projects"
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Projects
          </Link>
        </div>
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
              <p className="text-lg font-medium">Access Denied</p>
              <p className="text-sm text-muted-foreground mt-2">
                You do not have permission to view this project.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
              <Button asChild className="mt-4">
                <Link to="/settings/projects">Return to Projects</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Other error state (not found, etc.)
  if (error || !project) {
    return (
      <div className="py-6">
        <div className="mb-8">
          <Link
            to="/settings/projects"
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Projects
          </Link>
        </div>
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <Shield className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
              <p className="text-lg font-medium text-destructive">Project Not Found</p>
              <p className="text-sm text-muted-foreground mt-2">
                {error instanceof Error ? error.message : `Project "${name}" does not exist.`}
              </p>
              <Button asChild className="mt-4">
                <Link to="/settings/projects">Return to Projects</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Handle project update
  const handleUpdate = async (updates: Partial<UpdateProjectRequest>) => {
    await updateMutation.mutateAsync({
      name: project.name,
      request: {
        ...updates,
        resourceVersion: project.resourceVersion,
      },
    });
  };

  const roleCount = project.roles?.length || 0;
  const destinationCount = project.destinations?.length || 0;

  return (
    <div className="py-6">
      {/* Header with back navigation */}
      <div className="mb-8">
        <Link
          to="/settings/projects"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Projects
        </Link>
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Shield className="h-5 w-5" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight">{project.name}</h1>
              {project.description && (
                <p className="text-muted-foreground">{project.description}</p>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="flex items-center gap-1">
              <Users className="h-3 w-3" />
              <span>{roleCount} role{roleCount !== 1 ? "s" : ""}</span>
            </Badge>
            {destinationCount > 0 && (
              <Badge variant="outline">
                {destinationCount} destination{destinationCount !== 1 ? "s" : ""}
              </Badge>
            )}
          </div>
        </div>
      </div>

      {/* Tabbed content */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
        <TabsList className="grid w-full grid-cols-3 lg:w-auto lg:inline-grid">
          <TabsTrigger value="overview" className="flex items-center gap-2">
            <Settings className="h-4 w-4" />
            Overview
          </TabsTrigger>
          <TabsTrigger value="roles" className="flex items-center gap-2">
            <Users className="h-4 w-4" />
            Roles
          </TabsTrigger>
          <TabsTrigger value="destinations" className="flex items-center gap-2">
            <MapPin className="h-4 w-4" />
            Destinations
          </TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <ProjectOverviewTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        </TabsContent>

        <TabsContent value="roles">
          <ProjectRolesTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        </TabsContent>

        <TabsContent value="destinations">
          <ProjectDestinationsTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        </TabsContent>

      </Tabs>
    </div>
  );
}

export default ProjectDetail;
