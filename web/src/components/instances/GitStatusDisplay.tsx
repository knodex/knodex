import {
  GitBranch,
  GitCommit,
  ExternalLink,
  CheckCircle2,
  Clock,
  AlertCircle,
  Loader2,
  Zap,
  Layers,
  FileCode,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { DeploymentMode, GitInfo, GitPushStatus } from "@/types/rgd";
import { extractGitOpsLocation, buildGitOpsFileURL } from "@/types/rgd";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface GitStatusDisplayProps {
  deploymentMode?: DeploymentMode;
  gitInfo?: GitInfo;
  annotations?: Record<string, string>;
  className?: string;
}

const DEPLOYMENT_MODE_LABELS: Record<DeploymentMode, { label: string; icon: React.ReactNode }> = {
  direct: {
    label: "Direct Deployment",
    icon: <Zap className="h-4 w-4" />,
  },
  gitops: {
    label: "GitOps Deployment",
    icon: <GitBranch className="h-4 w-4" />,
  },
  hybrid: {
    label: "Hybrid Deployment",
    icon: <Layers className="h-4 w-4" />,
  },
};

const GIT_PUSH_STATUS_CONFIG: Record<
  GitPushStatus,
  { label: string; icon: React.ReactNode; className: string }
> = {
  not_applicable: {
    label: "N/A",
    icon: null,
    className: "text-muted-foreground",
  },
  pending: {
    label: "Pending",
    icon: <Clock className="h-3.5 w-3.5" />,
    className: "text-status-warning",
  },
  in_progress: {
    label: "Pushing...",
    icon: <Loader2 className="h-3.5 w-3.5 animate-spin" />,
    className: "text-primary",
  },
  completed: {
    label: "Pushed",
    icon: <CheckCircle2 className="h-3.5 w-3.5" />,
    className: "text-status-success",
  },
  failed: {
    label: "Failed",
    icon: <AlertCircle className="h-3.5 w-3.5" />,
    className: "text-destructive",
  },
};

export function GitStatusDisplay({
  deploymentMode,
  gitInfo,
  annotations,
  className,
}: GitStatusDisplayProps) {
  // Don't render if no deployment mode info
  if (!deploymentMode) {
    return null;
  }

  const modeConfig = DEPLOYMENT_MODE_LABELS[deploymentMode];
  const showGitInfo = deploymentMode !== "direct" && gitInfo;
  const pushStatusConfig = gitInfo
    ? GIT_PUSH_STATUS_CONFIG[gitInfo.pushStatus]
    : GIT_PUSH_STATUS_CONFIG.not_applicable;

  // Extract GitOps location from annotations
  const gitOpsLocation = extractGitOpsLocation(annotations);
  const manifestFileURL = gitOpsLocation ? buildGitOpsFileURL(gitOpsLocation) : null;

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-card overflow-hidden",
        className
      )}
    >
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-sm font-medium text-foreground">
          Deployment Information
        </h3>
      </div>

      <div className="divide-y divide-border">
        {/* Deployment Mode */}
        <div className="px-4 py-3 flex items-center justify-between">
          <span className="text-sm text-muted-foreground">Deployment Mode</span>
          <div className="flex items-center gap-2 text-sm font-medium text-foreground">
            {modeConfig.icon}
            <span>{modeConfig.label}</span>
          </div>
        </div>

        {/* Git Status - Only show for gitops/hybrid modes */}
        {showGitInfo && (
          <>
            {/* Git Push Status */}
            <div className="px-4 py-3 flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Git Push Status</span>
              <div
                className={cn(
                  "flex items-center gap-1.5 text-sm font-medium",
                  pushStatusConfig.className
                )}
              >
                {pushStatusConfig.icon}
                <span>{pushStatusConfig.label}</span>
              </div>
            </div>

            {/* Git Branch */}
            {gitInfo.branch && (
              <div className="px-4 py-3 flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Branch</span>
                <div className="flex items-center gap-1.5 text-sm font-mono text-foreground">
                  <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{gitInfo.branch}</span>
                </div>
              </div>
            )}

            {/* Git Path */}
            {gitInfo.path && (
              <div className="px-4 py-3 flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Path</span>
                <span className="text-sm font-mono text-foreground">
                  {gitInfo.path}
                </span>
              </div>
            )}

            {/* View Manifest in Git: AC-LINK-01, AC-LINK-02 */}
            {manifestFileURL && (
              <div className="px-4 py-3 flex items-center justify-between bg-primary/5">
                <span className="text-sm text-muted-foreground">Manifest File</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <a
                      href={manifestFileURL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1.5 text-sm font-medium text-primary hover:text-primary/80 transition-colors"
                    >
                      <FileCode className="h-3.5 w-3.5" />
                      <span>View in {gitOpsLocation?.vcs?.charAt(0).toUpperCase()}{gitOpsLocation?.vcs?.slice(1) || "GitHub"}</span>
                      <ExternalLink className="h-3.5 w-3.5" />
                    </a>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>View manifest file in repository</p>
                  </TooltipContent>
                </Tooltip>
              </div>
            )}

            {/* Git Commit (if successfully pushed) */}
            {gitInfo.commitSha && (
              <div className="px-4 py-3 flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Commit</span>
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-1.5 text-sm font-mono text-foreground">
                    <GitCommit className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>{gitInfo.commitSha.substring(0, 8)}</span>
                  </div>
                  {gitInfo.commitUrl && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <a
                          href={gitInfo.commitUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-primary hover:text-primary/80 transition-colors"
                        >
                          <ExternalLink className="h-3.5 w-3.5" />
                        </a>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p>View commit on GitHub</p>
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
              </div>
            )}

            {/* Pushed At */}
            {gitInfo.pushedAt && (
              <div className="px-4 py-3 flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Pushed At</span>
                <span className="text-sm text-foreground">
                  {new Date(gitInfo.pushedAt).toLocaleString()}
                </span>
              </div>
            )}

            {/* Git Push Error */}
            {gitInfo.pushStatus === "failed" && gitInfo.pushError && (
              <div className="px-4 py-3 bg-destructive/5">
                <div className="flex items-start gap-2">
                  <AlertCircle className="h-4 w-4 text-destructive shrink-0 mt-0.5" />
                  <div>
                    <span className="text-sm font-medium text-destructive block">
                      Git Push Failed
                    </span>
                    <p className="text-xs text-destructive/80 mt-1">
                      {gitInfo.pushError}
                    </p>
                  </div>
                </div>
              </div>
            )}
          </>
        )}

        {/* Direct mode message */}
        {deploymentMode === "direct" && (
          <div className="px-4 py-3 text-sm text-muted-foreground">
            Instance was deployed directly to the cluster via API.
          </div>
        )}
      </div>
    </div>
  );
}
