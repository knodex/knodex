// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import type { ProjectRole, UpdateProjectRequest } from "@/types/project";
import { logger } from "@/lib/logger";

interface UseRoleAdditionOptions {
  projectName: string;
  roles: ProjectRole[];
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  onSuccess: () => void;
}

export type RoleAdditionState = ReturnType<typeof useRoleAddition>;

export function useRoleAddition({ projectName, roles, onUpdate, onSuccess }: UseRoleAdditionOptions) {
  const [showAddForm, setShowAddForm] = useState(false);
  const [newRoleName, setNewRoleName] = useState("");
  const [newRoleDescription, setNewRoleDescription] = useState("");
  const [newRolePolicies, setNewRolePolicies] = useState<string[]>([]);
  const [newRoleGroups, setNewRoleGroups] = useState<string[]>([]);
  const [newRoleDestinations, setNewRoleDestinations] = useState<string[]>([]);
  const [isAdding, setIsAdding] = useState(false);
  const [addRoleError, setAddRoleError] = useState<string | null>(null);

  const resetForm = useCallback(() => {
    setNewRoleName("");
    setNewRoleDescription("");
    setNewRolePolicies([]);
    setNewRoleGroups([]);
    setNewRoleDestinations([]);
    setAddRoleError(null);
  }, []);

  const handleAddRole = useCallback(async () => {
    if (!newRoleName.trim() || isAdding) return;

    const roleName = newRoleName.trim().toLowerCase().replace(/\s+/g, "-");

    if (roles.some(r => r.name === roleName)) {
      setAddRoleError(`Role "${roleName}" already exists`);
      return;
    }

    const policies = newRolePolicies.length > 0
      ? newRolePolicies
      : [`p, proj:${projectName}:${roleName}, *, get, ${projectName}/*, allow`];

    const newRole: ProjectRole = {
      name: roleName,
      description: newRoleDescription.trim() || undefined,
      policies,
      groups: newRoleGroups.length > 0 ? newRoleGroups : [],
      destinations: newRoleDestinations.length > 0 ? newRoleDestinations : undefined,
    };

    setIsAdding(true);
    setAddRoleError(null);
    try {
      const updatedRoles = [...roles, newRole];
      await onUpdate({ roles: updatedRoles });
      setShowAddForm(false);
      resetForm();
      onSuccess();
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to add role:", error);
      setAddRoleError("Failed to add role. Please try again.");
    } finally {
      setIsAdding(false);
    }
  }, [newRoleName, isAdding, roles, newRolePolicies, projectName, newRoleDescription, newRoleGroups, newRoleDestinations, onUpdate, resetForm, onSuccess]);

  return {
    showAddForm,
    setShowAddForm,
    newRoleName,
    setNewRoleName,
    newRoleDescription,
    setNewRoleDescription,
    newRolePolicies,
    setNewRolePolicies,
    newRoleGroups,
    setNewRoleGroups,
    newRoleDestinations,
    setNewRoleDestinations,
    isAdding,
    addRoleError,
    setAddRoleError,
    resetForm,
    handleAddRole,
  };
}
