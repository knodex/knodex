// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Plus, Sparkles } from "@/lib/icons";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PolicyRulesTable } from "./PolicyRulesTable";
import { OIDCGroupsManager } from "./OIDCGroupsManager";
import { DestinationScopeSelector } from "./DestinationScopeSelector";
import { ROLE_PRESETS, resolvePresetPolicies } from "@/lib/role-presets";
import type { ProjectRole, Destination } from "@/types/project";
import type { RoleAdditionState } from "./hooks/useRoleAddition";

interface AddRoleFormProps {
  projectName: string;
  projectDestinations: Destination[];
  roles: ProjectRole[];
  addition: RoleAdditionState;
  onCancel: () => void;
}

export function AddRoleForm({
  projectName,
  projectDestinations,
  roles,
  addition,
  onCancel,
}: AddRoleFormProps) {
  const {
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
    handleAddRole: onAdd,
  } = addition;
  return (
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
                        setNewRolePolicies(resolvePresetPolicies(preset, projectName));
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
            projectId={projectName}
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

        {/* Destination Scope */}
        {projectDestinations.length > 1 && (
          <DestinationScopeSelector
            projectDestinations={projectDestinations}
            selectedDestinations={newRoleDestinations}
            onChange={setNewRoleDestinations}
            canEdit={true}
            isLoading={isAdding}
          />
        )}

        {addRoleError && (
          <p className="text-sm text-destructive">{addRoleError}</p>
        )}
        <div className="flex gap-2 justify-end">
          <Button
            variant="outline"
            onClick={onCancel}
            disabled={isAdding}
          >
            Cancel
          </Button>
          <Button onClick={onAdd} disabled={!newRoleName.trim() || isAdding}>
            {isAdding ? "Adding..." : "Add Role"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
