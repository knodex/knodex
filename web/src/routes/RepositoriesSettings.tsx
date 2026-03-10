// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { FolderGit2, ArrowLeft } from "lucide-react";
import { Link } from "react-router-dom";
import { useCanI } from "@/hooks/useCanI";
import { useCurrentProject } from "@/hooks/useAuth";
import { RepositorySection } from "@/components/repositories/RepositorySection";

/**
 * Repositories Settings page - Manage Git repositories for RGDs
 *
 * Access control: Accessible to all authenticated users. Authorization is handled
 * by the API via Casbin permission checks. If the API returns 403, the
 * RepositorySection component displays an Access Denied message. This follows
 * the ArgoCD pattern of pure Casbin permission checks at the API layer.
 *
 * Provides comprehensive UI for repository connections following ArgoCD patterns:
 * - List all configured repositories with connection status
 * - Add new repositories with credential configuration (Project Admin+)
 * - Test repository connections
 * - Edit existing repository configurations
 * - Delete repositories (with confirmation, Project Admin+)
 */
export function RepositoriesSettings() {
  const currentProject = useCurrentProject();

  // Real-time permission check via backend Casbin enforcer
  // Check if user can create repositories in their current project context
  // Project-scoped permissions (e.g., proj:my-project:admin) grant repositories:* on project/*
  const { allowed: canManageRepos, isLoading: isLoadingRepoPermission, isError: isErrorRepoPermission } = useCanI(
    'repositories',
    'create',
    currentProject || '-'
  );

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
            <FolderGit2 className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">Repositories</h2>
            <p className="text-muted-foreground">
              Configure Git repositories for ResourceGraphDefinitions
            </p>
          </div>
        </div>
      </div>

      {/* Repository management section with RBAC protection */}
      <RepositorySection canManage={canManageRepos} isLoadingPermission={isLoadingRepoPermission} isErrorPermission={isErrorRepoPermission} />
    </div>
  );
}

export default RepositoriesSettings;
