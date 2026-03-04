import { useMemo, useEffect } from "react";
import {
  Zap,
  GitBranch,
  Layers,
  AlertCircle,
  Loader2,
  Info,
  FolderOpen,
  Lock,
} from "lucide-react";
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

// Total number of deployment modes available
const TOTAL_DEPLOYMENT_MODES = Object.keys(DEPLOYMENT_MODE_INFO).length;

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

  // Track if modes are restricted (for showing info banner)
  const isRestricted = allowedModes && allowedModes.length > 0 && allowedModes.length < TOTAL_DEPLOYMENT_MODES;

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
        <label className="text-sm font-medium text-foreground flex items-center gap-2">
          Deployment Mode
          <span className="text-xs text-muted-foreground font-normal">
            (How the instance will be deployed)
          </span>
        </label>

        {/* Restricted modes info banner */}
        {isRestricted && (
          <div className="flex items-start gap-2 p-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30">
            <Lock className="h-4 w-4 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
            <p className="text-xs text-amber-700 dark:text-amber-300">
              {availableModes.length === 1 ? (
                <>
                  This RGD only allows <span className="font-medium">{DEPLOYMENT_MODE_INFO[availableModes[0]].label}</span> deployment mode.
                </>
              ) : (
                <>
                  This RGD is restricted to the following deployment modes:{" "}
                  <span className="font-medium">
                    {availableModes.map((m) => DEPLOYMENT_MODE_INFO[m].label).join(", ")}
                  </span>
                </>
              )}
            </p>
          </div>
        )}

        <div className={cn(
          "grid gap-2",
          availableModes.length === 1 ? "grid-cols-1" :
          availableModes.length === 2 ? "grid-cols-2" : "grid-cols-3"
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
                  "flex flex-col items-center gap-2 p-3 rounded-lg border transition-all",
                  isOnlyOption
                    ? "border-primary bg-primary/5 ring-1 ring-primary/20 cursor-default"
                    : "hover:border-primary/50 hover:bg-secondary/50",
                  isSelected && !isOnlyOption
                    ? "border-primary bg-primary/5 ring-1 ring-primary/20"
                    : !isOnlyOption && "border-border bg-background"
                )}
              >
                <div
                  className={cn(
                    "flex items-center justify-center h-8 w-8 rounded-full",
                    isSelected || isOnlyOption
                      ? "bg-primary/10 text-primary"
                      : "bg-secondary text-muted-foreground"
                  )}
                >
                  {MODE_ICONS[modeKey]}
                </div>
                <div className="text-center">
                  <span
                    className={cn(
                      "text-sm font-medium block",
                      isSelected || isOnlyOption ? "text-foreground" : "text-muted-foreground"
                    )}
                  >
                    {info.label}
                  </span>
                </div>
              </button>
            );
          })}
        </div>

        {/* Mode Description */}
        <div className="flex items-start gap-2 p-3 rounded-lg bg-secondary/50">
          <Info className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
          <p className="text-xs text-muted-foreground">
            {DEPLOYMENT_MODE_INFO[mode].description}
          </p>
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
            <div className="flex items-center gap-2 p-3 rounded-md border border-status-warning/30 bg-status-warning/5">
              <AlertCircle className="h-4 w-4 text-status-warning" />
              <div className="text-sm">
                <span className="text-status-warning font-medium">
                  No repositories configured.
                </span>
                <span className="text-muted-foreground">
                  {" "}
                  Configure a Git repository in project settings to use{" "}
                  {mode === "gitops" ? "GitOps" : "Hybrid"} deployment mode.
                </span>
              </div>
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
        <div className="space-y-3 pt-2 border-t border-border">
          <div className="flex items-center gap-2">
            <Info className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">
              Branch and path are auto-populated. Modify them if you need different values.
            </span>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            {/* Branch Override */}
            <div className="space-y-1.5">
              <label
                htmlFor="gitBranch"
                className="text-sm font-medium text-foreground flex items-center gap-1.5"
              >
                <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                Branch Override
              </label>
              <input
                id="gitBranch"
                type="text"
                value={gitBranch}
                onChange={(e) => onGitBranchChange(e.target.value)}
                placeholder={selectedRepository.defaultBranch || "main"}
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
              <p className="text-xs text-muted-foreground">
                Auto-populated from repository. Override if needed.
              </p>
            </div>

            {/* Path Override */}
            <div className="space-y-1.5">
              <label
                htmlFor="gitPath"
                className="text-sm font-medium text-foreground flex items-center gap-1.5"
              >
                <FolderOpen className="h-3.5 w-3.5 text-muted-foreground" />
                Path Override
              </label>
              <input
                id="gitPath"
                type="text"
                value={gitPath}
                onChange={(e) => onGitPathChange(e.target.value)}
                placeholder="manifests/project/namespace/rgd/instance.yaml"
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
              <p className="text-xs text-muted-foreground">
                Auto-generated semantic path. Override if needed.
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Hybrid mode explanation */}
      {mode === "hybrid" && (
        <div className="p-3 rounded-lg border border-primary/20 bg-primary/5">
          <h4 className="text-sm font-medium text-foreground mb-1">
            Hybrid Mode Behavior
          </h4>
          <ul className="text-xs text-muted-foreground space-y-1 list-disc list-inside">
            <li>Manifest is applied to the cluster immediately</li>
            <li>Manifest is then pushed to Git asynchronously</li>
            <li>
              <span className="text-primary font-medium">
                Git push failure does NOT fail the deployment
              </span>
            </li>
            <li>Git commit info is stored on the instance (if successful)</li>
          </ul>
        </div>
      )}
    </div>
  );
}
