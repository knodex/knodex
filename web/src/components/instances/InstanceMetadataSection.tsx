// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  GitCommit,
  AlertTriangle,
} from "@/lib/icons";
import type { Instance } from "@/types/rgd";

function buildRepoURL(
  repositoryUrl: string,
  annotations?: Record<string, string>,
  branch?: string,
  path?: string,
): string {
  const vcs = annotations?.["gitops.knodex.io/vcs"]?.toLowerCase() ?? "github";
  const cleanPath = path?.startsWith("/") ? path.slice(1) : path;

  if (repositoryUrl.includes("://")) {
    const base = repositoryUrl.replace(/\/$/, "");
    if (branch && cleanPath) {
      switch (vcs) {
        case "gitlab": return `${base}/-/blob/${branch}/${cleanPath}`;
        case "bitbucket": return `${base}/src/${branch}/${cleanPath}`;
        default: return `${base}/blob/${branch}/${cleanPath}`;
      }
    }
    return repositoryUrl;
  }

  if (branch && cleanPath) {
    switch (vcs) {
      case "gitlab": return `https://gitlab.com/${repositoryUrl}/-/blob/${branch}/${cleanPath}`;
      case "bitbucket": return `https://bitbucket.org/${repositoryUrl}/src/${branch}/${cleanPath}`;
      default: return `https://github.com/${repositoryUrl}/blob/${branch}/${cleanPath}`;
    }
  }

  switch (vcs) {
    case "gitlab": return `https://gitlab.com/${repositoryUrl}`;
    case "bitbucket": return `https://bitbucket.org/${repositoryUrl}`;
    default: return `https://github.com/${repositoryUrl}`;
  }
}

interface InstanceMetadataSectionProps {
  instance: Instance;
  isGitOps: boolean;
}

export function InstanceMetadataSection({ instance, isGitOps }: InstanceMetadataSectionProps) {
  return (
    <div className="px-6 py-3 flex items-center gap-4 text-sm" style={{ borderBottom: "1px solid var(--border-subtle)" }}>
      <span className="text-[var(--text-muted)]">Source</span>
      {isGitOps ? (
        <div className="flex items-center gap-4 text-[var(--text-primary)]">
          {instance.gitInfo?.repositoryUrl && (
            <a
              href={buildRepoURL(instance.gitInfo.repositoryUrl, instance.annotations, instance.gitInfo.branch, instance.gitInfo.path)}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 font-mono text-[var(--text-secondary)] hover:text-primary hover:underline transition-colors"
            >
              {instance.gitInfo.repositoryUrl}
            </a>
          )}
          {instance.gitInfo?.commitSha && (
            <span className="inline-flex items-center gap-1.5 font-mono">
              <GitCommit className="h-3.5 w-3.5 text-[var(--text-muted)]" />
              {instance.gitInfo.commitUrl ? (
                <a href={instance.gitInfo.commitUrl} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">
                  {instance.gitInfo.commitSha.substring(0, 8)}
                </a>
              ) : (
                instance.gitInfo.commitSha.substring(0, 8)
              )}
            </span>
          )}
          {!instance.gitInfo?.repositoryUrl && !instance.gitInfo?.commitSha && (
            <span className="text-[var(--text-secondary)]">GitOps</span>
          )}
          {instance.gitopsDrift && (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-status-warning">
              <AlertTriangle className="h-3.5 w-3.5" />
              Drifted
            </span>
          )}
        </div>
      ) : (
        <span className="text-[var(--text-secondary)]">Direct deployment</span>
      )}
    </div>
  );
}
