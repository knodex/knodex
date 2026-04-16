// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Destinations Tab — Vercel/Dokploy-style flat rows.
 * Namespace list with inline add/remove. No Card wrappers.
 */
import { useState, useCallback, useMemo } from "react";
import { Plus, Trash2, MapPin, AlertTriangle } from "@/lib/icons";
import { toast } from "sonner";
import { AxiosError } from "axios";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { listInstances } from "@/api/rgd";
import { ConfirmNamespaceRemovalDialog } from "@/components/projects/ConfirmNamespaceRemovalDialog";
import { toUserFriendlyError } from "@/lib/errors";
import type { Project, Destination, UpdateProjectRequest } from "@/types/project";
import { STALE_TIME } from "@/lib/query-client";

interface ProjectDestinationsTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

// DNS-1123 label: lowercase alphanumeric and hyphens, start/end with alphanumeric
const DNS_1123_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

function isValidNamespace(value: string): boolean {
  if (!value || value.length > 63) return false;
  if (value === "*") return true;
  if (value.startsWith("*") || value.endsWith("*")) {
    const nonWildcard = value.replace(/\*/g, "");
    return nonWildcard.length === 0 || /^[a-z0-9-]+$/.test(nonWildcard);
  }
  if (value.includes("*")) return false;
  return DNS_1123_PATTERN.test(value);
}

export function ProjectDestinationsTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectDestinationsTabProps) {
  const [newNamespace, setNewNamespace] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);

  // Removal dialog state
  const [pendingRemoval, setPendingRemoval] = useState<{
    destination: Destination;
    index: number;
  } | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);

  const destinations = useMemo(() => project.destinations || [], [project.destinations]);
  const isLastDestination = destinations.length <= 1;

  // Query instances in the namespace being removed (only when dialog is open)
  const namespaceToCheck = pendingRemoval?.destination.namespace || "";
  const isWildcard = namespaceToCheck.includes("*");
  const { data: instanceData, isLoading: isLoadingInstances } = useQuery({
    queryKey: ["instances", { namespace: namespaceToCheck }],
    queryFn: () => listInstances({ namespace: namespaceToCheck }),
    enabled: !!pendingRemoval && !isWildcard,
    staleTime: STALE_TIME.FREQUENT,
  });

  const handleAdd = useCallback(async () => {
    const trimmed = newNamespace.trim();
    if (!trimmed) return;

    if (!isValidNamespace(trimmed)) {
      setValidationError("Invalid namespace format. Use lowercase letters, numbers, hyphens, or wildcard patterns.");
      return;
    }

    if (destinations.some((d) => d.namespace === trimmed)) {
      setValidationError(`Namespace "${trimmed}" already exists.`);
      return;
    }

    setValidationError(null);
    const updatedDestinations = [...destinations, { namespace: trimmed }];
    try {
      await onUpdate({ destinations: updatedDestinations });
      setNewNamespace("");
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      const errorMessage = toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to add destination"
      );
      toast.error(errorMessage);
    }
  }, [newNamespace, destinations, onUpdate]);

  const handleRemoveClick = useCallback((index: number) => {
    const destination = destinations[index];
    setPendingRemoval({ destination, index });
  }, [destinations]);

  const handleConfirmRemoval = useCallback(async () => {
    if (!pendingRemoval) return;

    setIsRemoving(true);
    const updatedDestinations = destinations.filter(
      (_, i) => i !== pendingRemoval.index
    );
    try {
      await onUpdate({ destinations: updatedDestinations });
      setPendingRemoval(null);
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      const errorMessage = toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to remove destination"
      );
      toast.error(errorMessage);
    } finally {
      setIsRemoving(false);
    }
  }, [pendingRemoval, destinations, onUpdate]);

  const handleCancelRemoval = useCallback(() => {
    setPendingRemoval(null);
  }, []);

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAdd();
    }
  }, [handleAdd]);

  const instanceCount = useMemo(() => {
    if (!pendingRemoval) return null;
    return isWildcard ? null : (instanceData?.totalCount ?? null);
  }, [pendingRemoval, isWildcard, instanceData?.totalCount]);

  if (destinations.length === 0 && !canManage) {
    return (
      <div className="py-12 text-center">
        <MapPin className="h-8 w-8 mx-auto mb-3 text-muted-foreground opacity-50" />
        <p className="text-sm font-medium text-foreground">No destinations configured</p>
        <p className="text-xs text-muted-foreground mt-1">
          This project has no destination namespaces defined.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-0">
      {/* Section header */}
      <div className="flex items-center justify-between mb-1">
        <h3 className="text-sm font-medium text-foreground">Destination Namespaces</h3>
        <span className="text-xs text-muted-foreground">
          {destinations.length} namespace{destinations.length !== 1 ? "s" : ""}
        </span>
      </div>
      <p className="text-xs text-muted-foreground mb-4">
        Kubernetes namespaces where this project can deploy resources. Supports wildcards like <code className="px-1 py-0.5 rounded bg-secondary text-xs">dev-*</code>.
      </p>

      {/* Namespace rows */}
      <div className="border-t border-border">
        {destinations.map((dest, index) => (
          <div
            key={`${dest.namespace}-${index}`}
            className="flex items-center justify-between py-3 border-b border-border group"
          >
            <div className="flex items-center gap-2.5 min-w-0">
              <MapPin className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <code className="text-sm font-medium">{dest.namespace || "*"}</code>
              {dest.name && (
                <span className="text-xs text-muted-foreground">
                  ({dest.name})
                </span>
              )}
              {dest.namespace?.includes("*") && (
                <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                  wildcard
                </Badge>
              )}
            </div>
            {canManage && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRemoveClick(index)}
                      disabled={isLastDestination || isUpdating}
                      className="h-7 w-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                      aria-label="Remove destination"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </span>
                </TooltipTrigger>
                {isLastDestination && (
                  <TooltipContent>
                    At least one destination is required
                  </TooltipContent>
                )}
              </Tooltip>
            )}
          </div>
        ))}
      </div>

      {/* Add Destination */}
      {canManage && (
        <div className="pt-4">
          <div className="flex gap-2">
            <Input
              value={newNamespace}
              onChange={(e) => {
                setNewNamespace(e.target.value);
                if (validationError) setValidationError(null);
              }}
              onKeyDown={handleKeyDown}
              placeholder="Add namespace (e.g., production, dev-*)"
              disabled={isUpdating}
              className="flex-1 h-8 text-sm"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handleAdd}
              disabled={!newNamespace.trim() || isUpdating}
              className="h-8"
              aria-label="Add destination"
            >
              <Plus className="h-3.5 w-3.5 mr-1" />
              Add
            </Button>
          </div>
          {validationError && (
            <p className="mt-2 text-xs text-destructive flex items-center gap-1">
              <AlertTriangle className="h-3 w-3" />
              {validationError}
            </p>
          )}
        </div>
      )}

      {/* Confirmation Dialog */}
      {pendingRemoval && (
        <ConfirmNamespaceRemovalDialog
          isOpen={true}
          namespace={pendingRemoval.destination.namespace || "*"}
          instanceCount={isWildcard ? null : instanceCount}
          isLoadingCount={!isWildcard && isLoadingInstances}
          onConfirm={handleConfirmRemoval}
          onCancel={handleCancelRemoval}
          isRemoving={isRemoving}
        />
      )}
    </div>
  );
}
