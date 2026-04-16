// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, useCallback } from "react";
import { ChevronDown, ChevronRight, Edit2, Save, X, Trash2, Shield } from "@/lib/icons";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { PolicyRulesTable } from "./PolicyRulesTable";
import { OIDCGroupsManager } from "./OIDCGroupsManager";
import { DestinationScopeSelector, DestinationScopeBadge } from "./DestinationScopeSelector";
import type { ProjectRole, Destination } from "@/types/project";

const BUILT_IN_ROLES = ["admin", "developer", "viewer", "readonly"];

interface RoleListItemProps {
  originalRole: ProjectRole;
  role: ProjectRole;
  projectName: string;
  projectDestinations: Destination[];
  isExpanded: boolean;
  isEditing: boolean;
  hasChanges: boolean;
  canManage: boolean;
  isUpdating: boolean;
  onToggleExpand: (roleName: string) => void;
  onEdit: (roleName: string) => void;
  onSave: (roleName: string) => void;
  onCancelEdit: (roleName: string) => void;
  onDelete: (roleName: string) => void;
  onPoliciesChange: (roleName: string, policies: string[]) => void;
  onGroupsChange: (roleName: string, groups: string[]) => void;
  onDestinationsChange: (roleName: string, destinations: string[]) => void;
}

export const RoleListItem = memo(function RoleListItem({
  originalRole,
  role,
  projectName,
  projectDestinations,
  isExpanded,
  isEditing,
  hasChanges,
  canManage,
  isUpdating,
  onToggleExpand,
  onEdit,
  onSave,
  onCancelEdit,
  onDelete,
  onPoliciesChange,
  onGroupsChange,
  onDestinationsChange,
}: RoleListItemProps) {
  const roleName = originalRole.name;
  const isBuiltIn = BUILT_IN_ROLES.includes(roleName);

  const handlePoliciesChange = useCallback(
    (policies: string[]) => onPoliciesChange(roleName, policies),
    [onPoliciesChange, roleName]
  );

  const handleGroupsChange = useCallback(
    (groups: string[]) => onGroupsChange(roleName, groups),
    [onGroupsChange, roleName]
  );

  const handleDestinationsChange = useCallback(
    (destinations: string[]) => onDestinationsChange(roleName, destinations),
    [onDestinationsChange, roleName]
  );

  return (
    <Card className={hasChanges ? "ring-2 ring-primary/50" : ""}>
      <CardHeader className="py-4">
        <div
          role="button"
          tabIndex={0}
          className="flex items-center justify-between cursor-pointer"
          onClick={() => onToggleExpand(roleName)}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onToggleExpand(roleName); } }}
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
                {isBuiltIn && (
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
          <div role="toolbar" className="flex items-center gap-2" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
            <Badge variant="outline">
              {role.policies?.length || 0} policies
            </Badge>
            <Badge variant="outline">
              {role.groups?.length || 0} groups
            </Badge>
            <DestinationScopeBadge destinations={role.destinations} />
            {canManage && (
              <>
                {hasChanges ? (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onSave(roleName)}
                      disabled={isUpdating}
                      className="text-status-success hover:text-status-success hover:bg-status-success/10"
                    >
                      <Save className="h-4 w-4 mr-1" />
                      Save
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onCancelEdit(roleName)}
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
                      onEdit(roleName);
                      if (!isExpanded) {
                        onToggleExpand(roleName);
                      }
                    }}
                  >
                    <Edit2 className="h-4 w-4" />
                  </Button>
                )}
                {!isBuiltIn && !hasChanges && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onDelete(roleName)}
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
                projectId={projectName}
                roleName={roleName}
                policies={role.policies || []}
                onPoliciesChange={handlePoliciesChange}
                canEdit={canManage && (isEditing || hasChanges)}
                isLoading={isUpdating}
              />
            </div>

            {/* OIDC Groups Manager */}
            <OIDCGroupsManager
              groups={role.groups || []}
              onGroupsChange={handleGroupsChange}
              canEdit={canManage && (isEditing || hasChanges)}
              isLoading={isUpdating}
            />

            {/* Destination Scope */}
            {projectDestinations.length > 1 && (
              <DestinationScopeSelector
                projectDestinations={projectDestinations}
                selectedDestinations={role.destinations || []}
                onChange={handleDestinationsChange}
                canEdit={canManage && (isEditing || hasChanges)}
                isLoading={isUpdating}
              />
            )}
          </div>
        </CardContent>
      )}
    </Card>
  );
});
