// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import type { ProjectRole, UpdateProjectRequest } from "@/types/project";
import { logger } from "@/lib/logger";

interface UseRoleSaveLogicOptions {
  roles: ProjectRole[];
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
}

export function useRoleSaveLogic({ roles, onUpdate }: UseRoleSaveLogicOptions) {
  const [editingRole, setEditingRole] = useState<string | null>(null);
  const [pendingChanges, setPendingChanges] = useState<Map<string, ProjectRole>>(new Map());

  const getRoleData = useCallback((roleName: string): ProjectRole | undefined => {
    return pendingChanges.get(roleName) || roles.find(r => r.name === roleName);
  }, [pendingChanges, roles]);

  const handlePoliciesChange = useCallback((roleName: string, newPolicies: string[]) => {
    setPendingChanges(prev => {
      const currentRole = prev.get(roleName) || roles.find(r => r.name === roleName);
      if (!currentRole) return prev;
      const newMap = new Map(prev);
      newMap.set(roleName, {
        ...currentRole,
        policies: newPolicies,
      });
      return newMap;
    });
  }, [roles]);

  const handleGroupsChange = useCallback((roleName: string, newGroups: string[]) => {
    setPendingChanges(prev => {
      const currentRole = prev.get(roleName) || roles.find(r => r.name === roleName);
      if (!currentRole) return prev;
      const newMap = new Map(prev);
      newMap.set(roleName, {
        ...currentRole,
        groups: newGroups,
      });
      return newMap;
    });
  }, [roles]);

  const handleDestinationsChange = useCallback((roleName: string, newDestinations: string[]) => {
    setPendingChanges(prev => {
      const currentRole = prev.get(roleName) || roles.find(r => r.name === roleName);
      if (!currentRole) return prev;
      const newMap = new Map(prev);
      newMap.set(roleName, {
        ...currentRole,
        destinations: newDestinations.length > 0 ? newDestinations : undefined,
      });
      return newMap;
    });
  }, [roles]);

  const handleSaveRole = useCallback(async (roleName: string) => {
    const updatedRole = pendingChanges.get(roleName);
    if (!updatedRole) return;

    const updatedRoles = roles.map(r =>
      r.name === roleName ? updatedRole : r
    );

    try {
      await onUpdate({ roles: updatedRoles });
      setPendingChanges(prev => {
        const newMap = new Map(prev);
        newMap.delete(roleName);
        return newMap;
      });
      setEditingRole(null);
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to save role:", error);
    }
  }, [pendingChanges, roles, onUpdate]);

  const handleCancelEdit = useCallback((roleName: string) => {
    setPendingChanges(prev => {
      const newMap = new Map(prev);
      newMap.delete(roleName);
      return newMap;
    });
    setEditingRole(null);
  }, []);

  const hasUnsavedChanges = useCallback((roleName: string) => pendingChanges.has(roleName), [pendingChanges]);

  const clearAllPendingChanges = useCallback(() => {
    setPendingChanges(new Map());
  }, []);

  return {
    editingRole,
    setEditingRole,
    getRoleData,
    handlePoliciesChange,
    handleGroupsChange,
    handleDestinationsChange,
    handleSaveRole,
    handleCancelEdit,
    hasUnsavedChanges,
    clearAllPendingChanges,
  };
}
