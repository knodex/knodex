// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCanI } from "@/hooks/useCanI";
import type { Instance } from "@/types/rgd";

export interface InstancePermissions {
  canDelete: boolean;
  isLoadingCanDelete: boolean;
  isErrorCanDelete: boolean;
  canUpdate: boolean;
  isLoadingCanUpdate: boolean;
  isErrorCanUpdate: boolean;
  canReadRGD: boolean;
}

export function useInstancePermissions(
  instance: Instance,
  // parentRGD is fetched by useInstanceMetadata — passed in to avoid a duplicate useRGD call.
  // Falls back to the instance's project label while parentRGD is still loading, then
  // wildcard when no project label is present on either resource.
  parentRGD?: { labels?: Record<string, string> } | null,
): InstancePermissions {
  // The can-i endpoint expects a project name (or project/namespace) as the subresource,
  // NOT a bare namespace. Use the project label from the instance or parent RGD.
  // Fall back to '-' (no subresource) which lets Casbin evaluate without project scoping.
  const projectName = instance.labels?.['knodex.io/project'] || '-';
  const { allowed: canDelete, isLoading: isLoadingCanDelete, isError: isErrorCanDelete } = useCanI('instances', 'delete', projectName);
  const { allowed: canUpdate, isLoading: isLoadingCanUpdate, isError: isErrorCanUpdate } = useCanI('instances', 'update', projectName);

  const rgdProjectLabel = parentRGD?.labels?.['knodex.io/project']
    ?? instance.labels?.['knodex.io/project']
    ?? null;
  const rgdPermObject = rgdProjectLabel
    ? `${rgdProjectLabel}/${instance.rgdName}`
    : '-';
  const { allowed: canReadRGD } = useCanI('rgds', 'get', rgdPermObject);

  return {
    canDelete,
    isLoadingCanDelete,
    isErrorCanDelete,
    canUpdate,
    isLoadingCanUpdate,
    isErrorCanUpdate,
    canReadRGD,
  };
}
