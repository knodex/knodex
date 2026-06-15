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
import { TeamPicker } from "@/components/teams/TeamPicker";
import { DestinationScopeSelector } from "./DestinationScopeSelector";
import { resolvePresetPolicies } from "@/lib/role-presets";
import { useRoleTemplates } from "@/hooks/useRoleTemplates";
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
    newRoleTeams,
    setNewRoleTeams,
    newRoleDestinations,
    setNewRoleDestinations,
    isAdding,
    addRoleError,
    setAddRoleError,
    handleAddRole: onAdd,
  } = addition;
  // Presets come from the server-backed catalog (Story 18.1). While the query
  // loads (or if it 403s for a non-operator) the preset buttons are simply
  // absent — "Custom Role" is always available, so role creation never blocks.
  const { data: presets = [] } = useRoleTemplates();
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
          {presets.map((preset) => {
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
                        setNewRoleDescription(preset.description ?? "");
                        setNewRolePolicies(resolvePresetPolicies(preset, projectName));
                        setNewRoleTeams([]);
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
              setNewRoleTeams([]);
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

        {/* Teams (resolve to OIDC groups server-side; reusable) */}
        <TeamPicker
          selected={newRoleTeams}
          onChange={setNewRoleTeams}
          canEdit={!isAdding}
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
