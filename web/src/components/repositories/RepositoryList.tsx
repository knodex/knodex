// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Repository list component for displaying configured repositories
 * Supports both legacy (owner/repo) and new (repoURL/authType) formats
 */
import { useState } from "react";
import { GitBranch, Edit, Trash2, CheckCircle2, XCircle, AlertCircle, Key, Lock, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { RepositoryConfig, AuthType } from "@/types/repository";
import { getRepositoryDisplayURL, getAuthTypeDisplayName } from "@/types/repository";

interface RepositoryListProps {
  repositories: RepositoryConfig[];
  onEdit?: (repo: RepositoryConfig) => void;
  onDelete?: (repoID: string) => void;
  onTestConnection?: (repoID: string) => Promise<void>;
  canManage?: boolean;
  isLoading?: boolean;
}

function getValidationIcon(status?: string) {
  switch (status) {
    case "valid":
      return <CheckCircle2 className="h-4 w-4 text-status-success" />;
    case "invalid":
      return <XCircle className="h-4 w-4 text-destructive" />;
    default:
      return <AlertCircle className="h-4 w-4 text-muted-foreground" />;
  }
}

function getValidationBadgeClass(status?: string) {
  switch (status) {
    case "valid":
      return "bg-status-success/10 text-status-success";
    case "invalid":
      return "bg-destructive/10 text-destructive";
    default:
      return "bg-secondary text-muted-foreground";
  }
}

function getAuthTypeIcon(authType?: AuthType) {
  switch (authType) {
    case "ssh":
      return <Key className="h-3.5 w-3.5" />;
    case "https":
      return <Lock className="h-3.5 w-3.5" />;
    case "github-app":
      return <Shield className="h-3.5 w-3.5" />;
    default:
      return null;
  }
}

function getAuthTypeBadgeClass(authType?: AuthType) {
  switch (authType) {
    case "ssh":
      return "bg-primary/10 text-primary";
    case "https":
      return "bg-status-info/10 text-status-info";
    case "github-app":
      return "bg-status-warning/10 text-status-warning";
    default:
      return "bg-secondary text-muted-foreground";
  }
}

export function RepositoryList({
  repositories,
  onEdit,
  onDelete,
  onTestConnection,
  canManage = false,
  isLoading = false,
}: RepositoryListProps) {
  const [testingRepo, setTestingRepo] = useState<string | null>(null);

  const handleTestConnection = async (repoID: string) => {
    if (!onTestConnection) return;

    setTestingRepo(repoID);
    try {
      await onTestConnection(repoID);
    } finally {
      setTestingRepo(null);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        Loading repositories...
      </div>
    );
  }

  if (repositories.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        <GitBranch className="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p className="text-sm">No repositories configured yet.</p>
        {canManage && (
          <p className="text-xs mt-2">Click "Add Repository" to configure your first repository.</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {repositories.map((repo) => {
        const displayURL = getRepositoryDisplayURL(repo);

        return (
          <div
            key={repo.id}
            className="flex items-center justify-between p-4 rounded-lg border border-border hover:bg-secondary/50 transition-colors"
            data-testid="repository-card"
          >
            <div className="flex items-center gap-4 flex-1 min-w-0">
              {/* Icon */}
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 flex-shrink-0">
                <GitBranch className="h-5 w-5 text-primary" />
              </div>

              {/* Repository Info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm truncate">{repo.name}</span>
                  {repo.projectId && (
                    <Badge variant="outline" className="text-xs px-1.5 py-0">
                      {repo.projectId}
                    </Badge>
                  )}
                </div>
                <div className="text-xs text-muted-foreground truncate">
                  {displayURL} ({repo.defaultBranch})
                </div>
                {repo.validationMessage && (
                  <div className="text-xs text-muted-foreground mt-1 italic">
                    {repo.validationMessage}
                  </div>
                )}
              </div>

              {/* Status Badges */}
              <div className="flex items-center gap-2 flex-wrap">
                {/* Auth Type Badge */}
                {repo.authType && (
                  <Badge
                    className={`flex items-center gap-1 text-xs ${getAuthTypeBadgeClass(repo.authType)}`}
                    data-testid="auth-type-badge"
                  >
                    {getAuthTypeIcon(repo.authType)}
                    <span>{getAuthTypeDisplayName(repo.authType)}</span>
                  </Badge>
                )}

                {/* Validation Status Badge */}
                {repo.validationStatus && (
                  <Badge
                    className={`flex items-center gap-1.5 ${getValidationBadgeClass(repo.validationStatus)}`}
                    data-testid="validation-status-badge"
                  >
                    {getValidationIcon(repo.validationStatus)}
                    <span>{repo.validationStatus}</span>
                  </Badge>
                )}

              </div>
            </div>

            {/* Actions */}
            {canManage && (
              <div className="flex items-center gap-2 ml-4">
                {onTestConnection && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleTestConnection(repo.id)}
                    disabled={testingRepo === repo.id}
                    className="text-xs"
                    data-testid="test-connection-btn"
                  >
                    {testingRepo === repo.id ? "Testing..." : "Test"}
                  </Button>
                )}
                {onEdit && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onEdit(repo)}
                    data-testid="edit-repo-btn"
                  >
                    <Edit className="h-4 w-4" />
                  </Button>
                )}
                {onDelete && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onDelete(repo.id)}
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                    data-testid="delete-repo-btn"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
