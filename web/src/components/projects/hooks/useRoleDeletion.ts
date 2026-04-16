// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import type { ProjectRole, UpdateProjectRequest } from "@/types/project";
import { logger } from "@/lib/logger";

interface UseRoleDeletionOptions {
  roles: ProjectRole[];
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  onSuccess: () => void;
}

export function useRoleDeletion({ roles, onUpdate, onSuccess }: UseRoleDeletionOptions) {
  const [roleToDelete, setRoleToDelete] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteRoleError, setDeleteRoleError] = useState<string | null>(null);

  const handleDeleteRole = useCallback((roleName: string) => {
    setRoleToDelete(roleName);
  }, []);

  const confirmDeleteRole = useCallback(async () => {
    if (!roleToDelete) return;

    setIsDeleting(true);
    setDeleteRoleError(null);
    try {
      const updatedRoles = roles.filter(r => r.name !== roleToDelete);
      await onUpdate({ roles: updatedRoles });
      setRoleToDelete(null);
      onSuccess();
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to delete role:", error);
      setDeleteRoleError("Failed to delete role. Please try again.");
    } finally {
      setIsDeleting(false);
    }
  }, [roleToDelete, roles, onUpdate, onSuccess]);

  const cancelDeleteRole = useCallback(() => {
    setRoleToDelete(null);
    setDeleteRoleError(null);
  }, []);

  return {
    roleToDelete,
    isDeleting,
    deleteRoleError,
    handleDeleteRole,
    confirmDeleteRole,
    cancelDeleteRole,
  };
}
