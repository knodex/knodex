// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useEffect } from "react";
import { Link } from "react-router-dom";
import {
  Zap,
  GitBranch,
  Layers,
  AlertCircle,
  Loader2,
  FolderOpen,
} from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { DeploymentMode } from "@/types/deployment";
import { DEPLOYMENT_MODE_INFO } from "@/types/deployment";
import type { RepositoryConfig } from "@/types/repository";
import { getRepositoryDisplayURL } from "@/types/repository";

interface DeploymentModeSelectorProps {
  mode: DeploymentMode;
  /** Callback when mode changes. Should be memoized with useCallback to avoid unnecessary re-renders. */
  onModeChange: (mode: DeploymentMode) => void;
  repositoryId: string;
  onRepositoryChange: (repositoryId: string) => void;
  gitBranch: string;
  onGitBranchChange: (branch: string) => void;
  gitPath: string;
  onGitPathChange: (path: string) => void;
  repositories: RepositoryConfig[];
  isLoadingRepositories?: boolean;
  repositoriesError?: string | null;
  /** Allowed deployment modes for this RGD. If undefined/empty, all modes are allowed. */
  allowedModes?: DeploymentMode[];
  className?: string;
}

const MODE_ICONS: Record<DeploymentMode, React.ReactNode> = {
  direct: <Zap className="h-4 w-4" />,
  gitops: <GitBranch className="h-4 w-4" />,
  hybrid: <Layers className="h-4 w-4" />,
};

export function DeploymentModeSelector({
  mode,
  onModeChange,
  repositoryId,
  onRepositoryChange,
  gitBranch,
  onGitBranchChange,
  gitPath,
  onGitPathChange,
  repositories,
  isLoadingRepositories = false,
  repositoriesError,
  allowedModes,
  className,
}: DeploymentModeSelectorProps) {
  const requiresRepository = DEPLOYMENT_MODE_INFO[mode].requiresRepository;
  const hasRepositories = repositories.length > 0;

  // Compute available modes: if allowedModes is set and non-empty, filter to those
  // Otherwise, all modes are available
  const availableModes = useMemo<DeploymentMode[]>(() => {
    const allModes = Object.keys(DEPLOYMENT_MODE_INFO) as DeploymentMode[];
    if (!allowedModes || allowedModes.length === 0) {
      return allModes;
    }
    // Filter to only allowed modes, preserving the order from DEPLOYMENT_MODE_INFO
    return allModes.filter((m) => allowedModes.includes(m));
  }, [allowedModes]);

  // Auto-select when only one mode is allowed and current mode is not in available modes
  useEffect(() => {
    if (availableModes.length === 1 && mode !== availableModes[0]) {
      onModeChange(availableModes[0]);
    } else if (availableModes.length > 0 && !availableModes.includes(mode)) {
      // Current mode is not allowed, switch to first available
      onModeChange(availableModes[0]);
    }
  }, [availableModes, mode, onModeChange]);

  const selectedRepository = useMemo(() => {
    return repositories.find((r) => r.id === repositoryId);
  }, [repositories, repositoryId]);

  return (
    <div className={cn("space-y-4", className)}>
      {/* Mode Selection */}
      <div className="space-y-2">
        {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
        <label className="text-sm font-medium text-foreground" id="deployment-mode-label">
          Deployment Mode
        </label>

        <div className={cn(
          "flex gap-1.5",
        )}>
          {availableModes.map((modeKey) => {
            const info = DEPLOYMENT_MODE_INFO[modeKey];
            const isSelected = mode === modeKey;
            const isOnlyOption = availableModes.length === 1;

            return (
              <button
                key={modeKey}
                type="button"
                onClick={() => onModeChange(modeKey)}
                disabled={isOnlyOption}
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all border",
                  isOnlyOption
                    ? "border-primary/30 bg-primary/5 text-foreground cursor-default"
                    : "hover:border-primary/40 hover:bg-secondary/50",
                  isSelected && !isOnlyOption
                    ? "border-primary/30 bg-primary/5 text-foreground"
                    : !isOnlyOption && "border-border bg-background text-muted-foreground"
                )}
              >
                <span className={cn(
                  isSelected || isOnlyOption ? "text-primary" : "text-muted-foreground"
                )}>
                  {MODE_ICONS[modeKey]}
                </span>
                {info.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Repository Selection - only for gitops and hybrid modes */}
      {requiresRepository && (
        <div className="space-y-2">
          <label
            htmlFor="repository"
            className="text-sm font-medium text-foreground flex items-center gap-1"
          >
            Git Repository
            <span className="text-destructive">*</span>
          </label>

          {isLoadingRepositories ? (
            <div className="flex items-center gap-2 p-3 rounded-md border border-border bg-background">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
              <span className="text-sm text-muted-foreground">
                Loading repositories...
              </span>
            </div>
          ) : repositoriesError ? (
            <div className="flex items-center gap-2 p-3 rounded-md border border-destructive/30 bg-destructive/5">
              <AlertCircle className="h-4 w-4 text-destructive" />
              <span className="text-sm text-destructive">
                {repositoriesError}
              </span>
            </div>
          ) : !hasRepositories ? (
            <div className="flex items-center gap-2 p-2.5 rounded-md border border-status-warning/30 bg-status-warning/5">
              <AlertCircle className="h-3.5 w-3.5 text-status-warning shrink-0" />
              <p className="text-xs text-muted-foreground">
                No repositories configured.{" "}
                <Link to="/repositories" className="text-primary hover:underline font-medium">
                  Add one
                </Link>
              </p>
            </div>
          ) : (
            <>
              <select
                id="repository"
                value={repositoryId}
                onChange={(e) => onRepositoryChange(e.target.value)}
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              >
                <option value="">Select a repository...</option>
                {repositories.map((repo) => (
                  <option key={repo.id} value={repo.id}>
                    {repo.name} ({getRepositoryDisplayURL(repo)})
                  </option>
                ))}
              </select>

              {/* Selected Repository Details */}
              {selectedRepository && (
                <div className="p-3 rounded-md border border-border bg-secondary/30 space-y-1">
                  <div className="flex items-center gap-2">
                    <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="text-xs text-muted-foreground">
                      Branch:{" "}
                      <span className="font-mono text-foreground">
                        {selectedRepository.defaultBranch}
                      </span>
                    </span>
                  </div>
                  {selectedRepository.authType && (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground ml-5">
                        Auth:{" "}
                        <span className="font-mono text-foreground">
                          {selectedRepository.authType.toUpperCase()}
                        </span>
                      </span>
                    </div>
                  )}
                </div>
              )}
            </>
          )}

          {/* Repository validation warning */}
          {requiresRepository && !repositoryId && hasRepositories && (
            <p className="text-xs text-destructive flex items-center gap-1">
              <AlertCircle className="h-3 w-3" />
              Please select a repository for{" "}
              {mode === "gitops" ? "GitOps" : "Hybrid"} deployment
            </p>
          )}
        </div>
      )}

      {/* Branch and Path Overrides - only when repository is selected */}
      {requiresRepository && repositoryId && selectedRepository && (
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1">
            <label
              htmlFor="gitBranch"
              className="text-xs font-medium text-muted-foreground flex items-center gap-1"
            >
              <GitBranch className="h-3 w-3" />
              Branch
            </label>
            <input
              id="gitBranch"
              type="text"
              value={gitBranch}
              onChange={(e) => onGitBranchChange(e.target.value)}
              placeholder={selectedRepository.defaultBranch || "main"}
              className="w-full px-2.5 py-1.5 text-xs rounded-md border border-border bg-background focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary"
            />
          </div>
          <div className="space-y-1">
            <label
              htmlFor="gitPath"
              className="text-xs font-medium text-muted-foreground flex items-center gap-1"
            >
              <FolderOpen className="h-3 w-3" />
              Path
            </label>
            <input
              id="gitPath"
              type="text"
              value={gitPath}
              onChange={(e) => onGitPathChange(e.target.value)}
              placeholder="auto-generated (namespace/kind/name.yaml)"
              className="w-full px-2.5 py-1.5 text-xs rounded-md border border-border bg-background focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary"
            />
          </div>
        </div>
      )}
    </div>
  );
}
