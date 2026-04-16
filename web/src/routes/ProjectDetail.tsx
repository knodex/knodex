// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useParams, Link } from "react-router-dom";
import { ArrowLeft, Shield, ShieldAlert, Users, MapPin, Layers, Loader2, Settings } from "@/lib/icons";
import { AxiosError } from "axios";
import { useCanI } from "@/hooks/useCanI";
import { useProject, useUpdateProject } from "@/hooks/useProjects";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { UpdateProjectRequest } from "@/types/project";
import { isMultiCluster } from "@/types/project";
import { formatDateTime } from "@/lib/date";
import { TabBar } from "@/components/shared/TabBar";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab } from "@/hooks/useDynamicTabs";

// Tab components
import { ProjectOverviewTab } from "@/components/projects/tabs/ProjectOverviewTab";
import { ProjectRolesTab } from "@/components/projects/tabs/ProjectRolesTab";
import { ProjectDestinationsTab } from "@/components/projects/tabs/ProjectDestinationsTab";
import { ProjectResourcesTab } from "@/components/projects/tabs/ProjectResourcesTab";

type ProjectTabId = "overview" | "roles" | "destinations" | "resources";

const BASE_TABS: Tab<ProjectTabId>[] = [
  { id: "overview", label: "Overview", icon: <Settings className="h-4 w-4" /> },
  { id: "roles", label: "Roles", icon: <Users className="h-4 w-4" /> },
  { id: "destinations", label: "Destinations", icon: <MapPin className="h-4 w-4" /> },
];

/**
 * Project Detail page - View and manage a single project
 *
 * Access control: Accessible to all authenticated users. Authorization is handled
 * by the API via Casbin permission checks. If the API returns 403, the page
 * displays an Access Denied message.
 */
export function ProjectDetail() {
  const { name } = useParams<{ name: string }>();
  const { allowed: canManageProject, isLoading: isLoadingManage, isError: isErrorManage } = useCanI('projects', 'update', name || '-');

  // Fetch project data
  const { data: project, isLoading, error } = useProject(name || "");
  const updateMutation = useUpdateProject();

  const showResourcesTab = project ? isMultiCluster(project) : false;

  const conditionalTabs = useMemo(() => [
    {
      condition: showResourcesTab,
      tab: { id: "resources" as ProjectTabId, label: "Resources", icon: <Layers className="h-4 w-4" /> },
    },
  ], [showResourcesTab]);

  const { tabs, activeTab, setActiveTab } = useDynamicTabs(BASE_TABS, conditionalTabs, "overview" as ProjectTabId);

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // 403 Access Denied
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  if (is403Error) {
    return (
      <div className="space-y-6 animate-fade-in">
        <Link
          to="/projects"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Projects
        </Link>
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
                <Link to="/projects">Return to Projects</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Not found
  if (error || !project) {
    return (
      <div className="space-y-6 animate-fade-in">
        <Link
          to="/projects"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Projects
        </Link>
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <Shield className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
              <p className="text-lg font-medium text-destructive">Project Not Found</p>
              <p className="text-sm text-muted-foreground mt-2">
                {error instanceof Error ? error.message : `Project "${name}" does not exist.`}
              </p>
              <Button asChild className="mt-4">
                <Link to="/projects">Return to Projects</Link>
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

  return (
    <div className="space-y-0 animate-fade-in">
      {/* Back navigation */}
      <Link
        to="/projects"
        className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to Projects
      </Link>

      {/* Header — Vercel-style flat, no card wrapper */}
      <div className="mb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">
              {project.name}
            </h1>
            {project.description && (
              <p className="text-sm text-muted-foreground mt-1">
                {project.description}
              </p>
            )}
            <p className="text-xs text-muted-foreground mt-2">
              Created {formatDateTime(project.createdAt)}
              {project.updatedAt && (
                <span className="ml-3">
                  Updated {formatDateTime(project.updatedAt)}
                  {project.updatedBy && <span> by {project.updatedBy}</span>}
                </span>
              )}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <TabBar tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      {/* Tab content */}
      <div className="pt-6 min-h-[300px]" id={`panel-${activeTab}`} role="tabpanel" aria-labelledby={`tab-${activeTab}`}>
        {activeTab === "overview" && (
          <ProjectOverviewTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        )}

        {activeTab === "roles" && (
          <ProjectRolesTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        )}

        {activeTab === "destinations" && (
          <ProjectDestinationsTab
            project={project}
            onUpdate={handleUpdate}
            isUpdating={updateMutation.isPending}
            canManage={isLoadingManage || isErrorManage || canManageProject}
          />
        )}

        {activeTab === "resources" && showResourcesTab && (
          <ProjectResourcesTab project={project} active={activeTab === "resources"} />
        )}
      </div>
    </div>
  );
}

export default ProjectDetail;
