// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Pencil, Trash2, ExternalLink } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface InstanceActionButtonsProps {
  instanceUrl: string | null | undefined;
  canUpdate: boolean;
  isLoadingCanUpdate: boolean;
  isErrorCanUpdate: boolean;
  canDelete: boolean;
  isLoadingCanDelete: boolean;
  isErrorCanDelete: boolean;
  isTerminal: boolean;
  isDeleting: boolean;
  kroState: string;
  onEdit: () => void;
  onDelete: () => void;
}

export function InstanceActionButtons({
  instanceUrl,
  canUpdate,
  isLoadingCanUpdate,
  isErrorCanUpdate,
  canDelete,
  isLoadingCanDelete,
  isErrorCanDelete,
  isTerminal,
  isDeleting,
  kroState,
  onEdit,
  onDelete,
}: InstanceActionButtonsProps) {
  return (
    <div className="flex items-center gap-2">
      {instanceUrl && (
        <Button variant="outline" size="sm" className="gap-1.5" asChild>
          <a href={instanceUrl} target="_blank" rel="noopener noreferrer">
            <ExternalLink className="h-3.5 w-3.5" />
            Visit
          </a>
        </Button>
      )}
      {(isLoadingCanUpdate || isErrorCanUpdate || canUpdate) && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              <Button
                variant="outline"
                size="sm"
                onClick={onEdit}
                disabled={isTerminal}
                className="gap-1.5"
              >
                <Pencil className="h-3.5 w-3.5" />
                Edit Spec
              </Button>
            </span>
          </TooltipTrigger>
          {isTerminal && (
            <TooltipContent>Cannot edit while instance is {kroState.toLowerCase()}</TooltipContent>
          )}
        </Tooltip>
      )}
      {!isDeleting && (isLoadingCanDelete || isErrorCanDelete || canDelete) && (
        <Button
          variant="outline"
          size="sm"
          onClick={onDelete}
          className="gap-1.5 text-destructive border-destructive/30 hover:bg-destructive/10"
        >
          <Trash2 className="h-3.5 w-3.5" />
          Delete
        </Button>
      )}
    </div>
  );
}
