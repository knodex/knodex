// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Roles Tab — Vercel/Dokploy-style flat layout.
 * Role list with expand/collapse. No Card wrappers.
 */
import { useState, useMemo, useCallback } from "react";
import { Plus, Users } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import type { Project, UpdateProjectRequest } from "@/types/project";
import { DeleteRoleDialog } from "../DeleteRoleDialog";
import { AddRoleForm } from "../AddRoleForm";
import { RoleListItem } from "../RoleListItem";
import { useRoleAddition } from "../hooks/useRoleAddition";
import { useRoleDeletion } from "../hooks/useRoleDeletion";
import { useRoleSaveLogic } from "../hooks/useRoleSaveLogic";

interface ProjectRolesTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

export function ProjectRolesTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectRolesTabProps) {
  const [expandedRoles, setExpandedRoles] = useState<Set<string>>(new Set());
  const roles = useMemo(() => project.roles || [], [project.roles]);

  const saveLogic = useRoleSaveLogic({ roles, onUpdate });

  const addition = useRoleAddition({
    projectName: project.name,
    roles,
    onUpdate,
    onSuccess: () => saveLogic.clearAllPendingChanges(),
  });

  const deletion = useRoleDeletion({
    roles,
    onUpdate,
    onSuccess: () => saveLogic.clearAllPendingChanges(),
  });

  const toggleRoleExpanded = useCallback((roleName: string) => {
    setExpandedRoles(prev => {
      const newExpanded = new Set(prev);
      if (newExpanded.has(roleName)) {
        newExpanded.delete(roleName);
      } else {
        newExpanded.add(roleName);
      }
      return newExpanded;
    });
  }, []);

  if (roles.length === 0 && !addition.showAddForm) {
    return (
      <div className="py-12 text-center">
        <Users className="h-8 w-8 mx-auto mb-3 text-muted-foreground opacity-50" />
        <p className="text-sm font-medium text-foreground">No roles defined</p>
        <p className="text-xs text-muted-foreground mt-1 max-w-sm mx-auto">
          Roles define what actions users can perform within this project.
        </p>
        {canManage && (
          <Button size="sm" className="mt-4" onClick={() => addition.setShowAddForm(true)}>
            <Plus className="h-3.5 w-3.5 mr-1.5" />
            Add Role
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium text-foreground">Project Roles</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            {roles.length} role{roles.length !== 1 ? "s" : ""} defined
          </p>
        </div>
        {canManage && !addition.showAddForm && (
          <Button size="sm" onClick={() => addition.setShowAddForm(true)}>
            <Plus className="h-3.5 w-3.5 mr-1.5" />
            Add Role
          </Button>
        )}
      </div>

      {/* Add Role Form */}
      {addition.showAddForm && (
        <AddRoleForm
          projectName={project.name}
          projectDestinations={project.destinations || []}
          roles={roles}
          addition={addition}
          onCancel={() => {
            addition.setShowAddForm(false);
            addition.resetForm();
          }}
        />
      )}

      {/* Role List */}
      <div className="space-y-2">
        {roles.map((originalRole) => {
          const roleName = originalRole.name;
          const role = saveLogic.getRoleData(roleName) || originalRole;

          return (
            <RoleListItem
              key={roleName}
              originalRole={originalRole}
              role={role}
              projectName={project.name}
              isExpanded={expandedRoles.has(roleName)}
              isEditing={saveLogic.editingRole === roleName}
              hasChanges={saveLogic.hasUnsavedChanges(roleName)}
              canManage={canManage}
              isUpdating={isUpdating}
              onToggleExpand={toggleRoleExpanded}
              onEdit={saveLogic.setEditingRole}
              onSave={saveLogic.handleSaveRole}
              onCancelEdit={saveLogic.handleCancelEdit}
              onDelete={deletion.handleDeleteRole}
              projectDestinations={project.destinations || []}
              onPoliciesChange={saveLogic.handlePoliciesChange}
              onGroupsChange={saveLogic.handleGroupsChange}
              onDestinationsChange={saveLogic.handleDestinationsChange}
            />
          );
        })}
      </div>

      {/* Delete Role Confirmation Dialog */}
      {deletion.roleToDelete && (
        <DeleteRoleDialog
          roleName={deletion.roleToDelete}
          isOpen={true}
          onConfirm={deletion.confirmDeleteRole}
          onCancel={deletion.cancelDeleteRole}
          isDeleting={deletion.isDeleting}
          error={deletion.deleteRoleError}
        />
      )}
    </div>
  );
}
