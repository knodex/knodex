/**
 * Project Roles Tab - Manage project roles and permissions
 * Refactored to use ArgoCD-style PolicyRulesTable and OIDCGroupsManager
 */
import { useState, useCallback, useMemo } from "react";
import { Plus, Trash2, Users, Shield, ChevronDown, ChevronRight, Edit2, Save, X, Sparkles } from "lucide-react";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Project, ProjectRole, UpdateProjectRequest } from "@/types/project";
import { PolicyRulesTable } from "../PolicyRulesTable";
import { OIDCGroupsManager } from "../OIDCGroupsManager";
import { DeleteRoleDialog } from "../DeleteRoleDialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ROLE_PRESETS, resolvePresetPolicies } from "@/lib/role-presets";
import { logger } from "@/lib/logger";

interface ProjectRolesTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

// Built-in roles that cannot be deleted
const BUILT_IN_ROLES = ["admin", "developer", "viewer", "readonly"];

export function ProjectRolesTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectRolesTabProps) {
  const [expandedRoles, setExpandedRoles] = useState<Set<string>>(new Set());
  const [showAddForm, setShowAddForm] = useState(false);
  const [newRoleName, setNewRoleName] = useState("");
  const [newRoleDescription, setNewRoleDescription] = useState("");
  const [editingRole, setEditingRole] = useState<string | null>(null);
  const [pendingChanges, setPendingChanges] = useState<Map<string, ProjectRole>>(new Map());
  const [roleToDelete, setRoleToDelete] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteRoleError, setDeleteRoleError] = useState<string | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [addRoleError, setAddRoleError] = useState<string | null>(null);
  const [newRolePolicies, setNewRolePolicies] = useState<string[]>([]);
  const [newRoleGroups, setNewRoleGroups] = useState<string[]>([]);

  const roles = useMemo(() => project.roles || [], [project.roles]);

  const toggleRoleExpanded = (roleName: string) => {
    const newExpanded = new Set(expandedRoles);
    if (newExpanded.has(roleName)) {
      newExpanded.delete(roleName);
    } else {
      newExpanded.add(roleName);
    }
    setExpandedRoles(newExpanded);
  };

  // Get the current role data (from pending changes or original)
  const getRoleData = useCallback((roleName: string): ProjectRole | undefined => {
    return pendingChanges.get(roleName) || roles.find(r => r.name === roleName);
  }, [pendingChanges, roles]);

  // Handle policy changes for a role
  const handlePoliciesChange = useCallback((roleName: string, newPolicies: string[]) => {
    const currentRole = getRoleData(roleName);
    if (!currentRole) return;

    setPendingChanges(prev => {
      const newMap = new Map(prev);
      newMap.set(roleName, {
        ...currentRole,
        policies: newPolicies,
      });
      return newMap;
    });
  }, [getRoleData]);

  // Handle groups changes for a role
  const handleGroupsChange = useCallback((roleName: string, newGroups: string[]) => {
    const currentRole = getRoleData(roleName);
    if (!currentRole) return;

    setPendingChanges(prev => {
      const newMap = new Map(prev);
      newMap.set(roleName, {
        ...currentRole,
        groups: newGroups,
      });
      return newMap;
    });
  }, [getRoleData]);

  // Save changes for a specific role
  const handleSaveRole = async (roleName: string) => {
    const updatedRole = pendingChanges.get(roleName);
    if (!updatedRole) return;

    // Build updated roles array
    const updatedRoles = roles.map(r =>
      r.name === roleName ? updatedRole : r
    );

    try {
      // Call the project update API with roles
      await onUpdate({ roles: updatedRoles });

      // Clear pending changes for this role after successful save
      setPendingChanges(prev => {
        const newMap = new Map(prev);
        newMap.delete(roleName);
        return newMap;
      });
      setEditingRole(null);
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to save role:", error);
    }
  };

  // Cancel editing for a specific role
  const handleCancelEdit = (roleName: string) => {
    setPendingChanges(prev => {
      const newMap = new Map(prev);
      newMap.delete(roleName);
      return newMap;
    });
    setEditingRole(null);
  };

  const handleAddRole = async () => {
    if (!newRoleName.trim() || isAdding) return;

    const roleName = newRoleName.trim().toLowerCase().replace(/\s+/g, "-");

    // Check for duplicate role name
    if (roles.some(r => r.name === roleName)) {
      setAddRoleError(`Role "${roleName}" already exists`);
      return;
    }

    const policies = newRolePolicies.length > 0
      ? newRolePolicies
      : [`p, proj:${project.name}:${roleName}, *, get, ${project.name}/*, allow`];

    const newRole: ProjectRole = {
      name: roleName,
      description: newRoleDescription.trim() || undefined,
      policies,
      groups: newRoleGroups.length > 0 ? newRoleGroups : [],
    };

    setIsAdding(true);
    setAddRoleError(null);
    try {
      const updatedRoles = [...roles, newRole];
      await onUpdate({ roles: updatedRoles });
      setShowAddForm(false);
      setNewRoleName("");
      setNewRoleDescription("");
      setNewRolePolicies([]);
      setNewRoleGroups([]);
      // Clear any pending changes since we just saved
      setPendingChanges(new Map());
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to add role:", error);
      setAddRoleError("Failed to add role. Please try again.");
    } finally {
      setIsAdding(false);
    }
  };

  const handleDeleteRole = (roleName: string) => {
    setRoleToDelete(roleName);
  };

  const confirmDeleteRole = async () => {
    if (!roleToDelete) return;

    setIsDeleting(true);
    setDeleteRoleError(null);
    try {
      const updatedRoles = roles.filter(r => r.name !== roleToDelete);
      await onUpdate({ roles: updatedRoles });
      setRoleToDelete(null);
      // Clear any pending changes since we just saved
      setPendingChanges(new Map());
    } catch (error) {
      logger.error("[ProjectRolesTab] Failed to delete role:", error);
      setDeleteRoleError("Failed to delete role. Please try again.");
    } finally {
      setIsDeleting(false);
    }
  };

  const cancelDeleteRole = () => {
    setRoleToDelete(null);
    setDeleteRoleError(null);
  };

  const isBuiltInRole = (roleName: string) => BUILT_IN_ROLES.includes(roleName);
  const hasUnsavedChanges = (roleName: string) => pendingChanges.has(roleName);

  if (roles.length === 0 && !showAddForm) {
    return (
      <Card>
        <CardContent className="py-12">
          <div className="text-center">
            <Users className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
            <p className="text-lg font-medium">No roles defined</p>
            <p className="text-sm text-muted-foreground mt-2 max-w-md mx-auto">
              Roles define what actions users can perform within this project.
              Add custom roles to fine-tune access control.
            </p>
            {canManage && (
              <Button className="mt-4" onClick={() => setShowAddForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Add Role
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header with Add button */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-medium">Project Roles</h3>
          <p className="text-sm text-muted-foreground">
            {roles.length} role{roles.length !== 1 ? "s" : ""} defined
          </p>
        </div>
        {canManage && !showAddForm && (
          <Button onClick={() => setShowAddForm(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Add Role
          </Button>
        )}
      </div>

      {/* Add Role Form */}
      {showAddForm && (
        <Card>
          <CardHeader>
            <CardTitle>Add New Role</CardTitle>
            <CardDescription>
              Create a custom role with specific permissions
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Preset Buttons */}
            <div className="flex flex-wrap gap-2">
              {ROLE_PRESETS.map((preset) => {
                const exists = roles.some(r => r.name === preset.name);
                return (
                  <Tooltip key={preset.name}>
                    <TooltipTrigger asChild>
                      <span>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={exists || isAdding}
                          onClick={() => {
                            setNewRoleName(preset.name);
                            setNewRoleDescription(preset.description);
                            setNewRolePolicies(resolvePresetPolicies(preset, project.name));
                            setNewRoleGroups([]);
                            if (addRoleError) setAddRoleError(null);
                          }}
                        >
                          <Sparkles className="h-4 w-4 mr-1" />
                          {preset.label}
                        </Button>
                      </span>
                    </TooltipTrigger>
                    {exists && <TooltipContent>Role already exists</TooltipContent>}
                  </Tooltip>
                );
              })}
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isAdding}
                onClick={() => {
                  setNewRoleName("");
                  setNewRoleDescription("");
                  setNewRolePolicies([]);
                  setNewRoleGroups([]);
                  if (addRoleError) setAddRoleError(null);
                }}
              >
                <Plus className="h-4 w-4 mr-1" />
                Custom Role
              </Button>
            </div>

            <div>
              <Label htmlFor="role-name">Role Name</Label>
              <Input
                id="role-name"
                value={newRoleName}
                onChange={(e) => {
                  setNewRoleName(e.target.value);
                  if (addRoleError) setAddRoleError(null);
                }}
                placeholder="e.g., deployer"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="role-description">Description</Label>
              <Textarea
                id="role-description"
                value={newRoleDescription}
                onChange={(e) => setNewRoleDescription(e.target.value)}
                placeholder="What can this role do?"
                className="mt-1"
                rows={2}
              />
            </div>

            {/* Policy Rules */}
            <div>
              <Label className="text-muted-foreground mb-2 block">Policy Rules</Label>
              <PolicyRulesTable
                key={newRoleName}
                projectId={project.name}
                roleName={newRoleName.trim().toLowerCase().replace(/\s+/g, "-") || "new-role"}
                policies={newRolePolicies}
                onPoliciesChange={setNewRolePolicies}
                canEdit={true}
                isLoading={isAdding}
              />
            </div>

            {/* OIDC Groups */}
            <OIDCGroupsManager
              groups={newRoleGroups}
              onGroupsChange={setNewRoleGroups}
              canEdit={true}
              isLoading={isAdding}
            />

            {addRoleError && (
              <p className="text-sm text-destructive">{addRoleError}</p>
            )}
            <div className="flex gap-2 justify-end">
              <Button
                variant="outline"
                onClick={() => {
                  setShowAddForm(false);
                  setNewRoleName("");
                  setNewRoleDescription("");
                  setNewRolePolicies([]);
                  setNewRoleGroups([]);
                  setAddRoleError(null);
                }}
                disabled={isAdding}
              >
                Cancel
              </Button>
              <Button onClick={handleAddRole} disabled={!newRoleName.trim() || isAdding}>
                {isAdding ? "Adding..." : "Add Role"}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Role List */}
      <div className="space-y-3">
        {roles.map((originalRole) => {
          const roleName = originalRole.name;
          const role = getRoleData(roleName) || originalRole;
          const isExpanded = expandedRoles.has(roleName);
          const isEditing = editingRole === roleName;
          const hasChanges = hasUnsavedChanges(roleName);

          return (
            <Card key={roleName} className={hasChanges ? "ring-2 ring-primary/50" : ""}>
              <CardHeader className="py-4">
                <div
                  className="flex items-center justify-between cursor-pointer"
                  onClick={() => toggleRoleExpanded(roleName)}
                >
                  <div className="flex items-center gap-3">
                    <Button variant="ghost" size="sm" className="p-0 h-auto">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4" />
                      ) : (
                        <ChevronRight className="h-4 w-4" />
                      )}
                    </Button>
                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10">
                      <Shield className="h-4 w-4 text-primary" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{roleName}</span>
                        {isBuiltInRole(roleName) && (
                          <Badge variant="secondary" className="text-xs">
                            Built-in
                          </Badge>
                        )}
                        {hasChanges && (
                          <Badge variant="outline" className="text-xs text-primary border-primary">
                            Unsaved
                          </Badge>
                        )}
                      </div>
                      {role.description && (
                        <p className="text-xs text-muted-foreground">
                          {role.description}
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                    <Badge variant="outline">
                      {role.policies?.length || 0} policies
                    </Badge>
                    <Badge variant="outline">
                      {role.groups?.length || 0} groups
                    </Badge>
                    {canManage && (
                      <>
                        {hasChanges ? (
                          <>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleSaveRole(roleName)}
                              disabled={isUpdating}
                              className="text-status-success hover:text-status-success hover:bg-status-success/10"
                            >
                              <Save className="h-4 w-4 mr-1" />
                              Save
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleCancelEdit(roleName)}
                              disabled={isUpdating}
                            >
                              <X className="h-4 w-4" />
                            </Button>
                          </>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setEditingRole(roleName);
                              if (!isExpanded) {
                                toggleRoleExpanded(roleName);
                              }
                            }}
                          >
                            <Edit2 className="h-4 w-4" />
                          </Button>
                        )}
                        {!isBuiltInRole(roleName) && !hasChanges && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDeleteRole(roleName)}
                            className="text-destructive hover:text-destructive"
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        )}
                      </>
                    )}
                  </div>
                </div>
              </CardHeader>
              {isExpanded && (
                <CardContent className="pt-0">
                  <div className="space-y-6 pl-11">
                    {/* Policy Rules Table - ArgoCD-style editor */}
                    <div>
                      <Label className="text-muted-foreground mb-2 block">Policy Rules</Label>
                      <PolicyRulesTable
                        projectId={project.name}
                        roleName={roleName}
                        policies={role.policies || []}
                        onPoliciesChange={(policies) => handlePoliciesChange(roleName, policies)}
                        canEdit={canManage && (isEditing || hasChanges)}
                        isLoading={isUpdating}
                      />
                    </div>

                    {/* OIDC Groups Manager */}
                    <OIDCGroupsManager
                      groups={role.groups || []}
                      onGroupsChange={(groups) => handleGroupsChange(roleName, groups)}
                      canEdit={canManage && (isEditing || hasChanges)}
                      isLoading={isUpdating}
                    />
                  </div>
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>

      {/* Delete Role Confirmation Dialog */}
      {roleToDelete && (
        <DeleteRoleDialog
          roleName={roleToDelete}
          isOpen={true}
          onConfirm={confirmDeleteRole}
          onCancel={cancelDeleteRole}
          isDeleting={isDeleting}
          error={deleteRoleError}
        />
      )}
    </div>
  );
}
