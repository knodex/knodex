// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useState } from "react";
import { Plus, X, Shield, Sparkles, ChevronDown, ChevronUp } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { Destination, ProjectRole } from "@/types/project";
import { PolicyRulesTable } from "../PolicyRulesTable";
import { TeamPicker } from "@/components/teams/TeamPicker";
import { DestinationScopeSelector } from "../DestinationScopeSelector";
import { resolvePreset } from "@/lib/role-presets";
import { useRoleTemplates } from "@/hooks/useRoleTemplates";

interface RolesStepProps {
  projectName: string;
  destinations: Destination[];
  roles: ProjectRole[];
  onRolesChange: (roles: ProjectRole[]) => void;
  error?: string;
}

export function RolesStep({
  projectName,
  destinations,
  roles,
  onRolesChange,
  error,
}: RolesStepProps) {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(
    roles.length > 0 ? 0 : null,
  );

  // Presets come from the server-backed catalog (Story 18.1). While the query
  // loads (or 403s for a non-operator) the preset buttons are simply absent —
  // "Custom Role" is always available, so role creation never blocks.
  const { data: presets = [] } = useRoleTemplates();

  const addPresetRole = useCallback(
    (presetName: string) => {
      const preset = presets.find((p) => p.name === presetName);
      if (!preset) return;
      const role = resolvePreset(preset, projectName);
      const newRoles = [...roles, role];
      onRolesChange(newRoles);
      setExpandedIndex(newRoles.length - 1);
    },
    [presets, roles, projectName, onRolesChange],
  );

  const addCustomRole = useCallback(() => {
    const newRoles = [
      ...roles,
      { name: "", description: "", policies: [] },
    ];
    onRolesChange(newRoles);
    setExpandedIndex(newRoles.length - 1);
  }, [roles, onRolesChange]);

  const removeRole = useCallback(
    (index: number) => {
      onRolesChange(roles.filter((_, i) => i !== index));
      if (expandedIndex === index) setExpandedIndex(null);
      else if (expandedIndex !== null && expandedIndex > index)
        setExpandedIndex(expandedIndex - 1);
    },
    [roles, expandedIndex, onRolesChange],
  );

  const updateRole = useCallback(
    (index: number, updates: Partial<ProjectRole>) => {
      onRolesChange(
        roles.map((r, i) => (i === index ? { ...r, ...updates } : r)),
      );
    },
    [roles, onRolesChange],
  );

  const toggleExpand = useCallback(
    (index: number) => {
      setExpandedIndex(expandedIndex === index ? null : index);
    },
    [expandedIndex],
  );

  return (
    <div className="space-y-5" data-testid="roles-step">
      <div className="space-y-1.5">
        <Label>Roles (Optional)</Label>
        <p className="text-xs text-[var(--text-muted)]">
          Add roles to control who can do what. You can also add roles later from
          the project detail page.
        </p>
      </div>

      {error && (
        <p className="text-xs text-[var(--status-error)]">{error}</p>
      )}

      {/* Preset buttons */}
      <div className="flex flex-wrap gap-2">
        {presets.map((preset) => {
          const exists = roles.some((r) => r.name === preset.name);
          return (
            <Tooltip key={preset.name}>
              <TooltipTrigger asChild>
                <span>
                  <button
                    type="button"
                    disabled={exists}
                    onClick={() => addPresetRole(preset.name)}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    style={{
                      backgroundColor: exists
                        ? "transparent"
                        : "rgba(255,255,255,0.06)",
                      color: exists
                        ? "var(--text-muted)"
                        : "var(--text-secondary)",
                      border: "1px solid rgba(255,255,255,0.10)",
                    }}
                  >
                    <Sparkles className="h-3.5 w-3.5" />
                    {preset.label}
                  </button>
                </span>
              </TooltipTrigger>
              {exists && <TooltipContent>Role already added</TooltipContent>}
              {!exists && (
                <TooltipContent>{preset.description}</TooltipContent>
              )}
            </Tooltip>
          );
        })}
        <button
          type="button"
          onClick={addCustomRole}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          style={{
            backgroundColor: "rgba(255,255,255,0.06)",
            color: "var(--text-secondary)",
            border: "1px solid rgba(255,255,255,0.10)",
          }}
        >
          <Plus className="h-3.5 w-3.5" />
          Custom Role
        </button>
      </div>

      {/* Role cards */}
      {roles.length > 0 && (
        <div className="space-y-2">
          {roles.map((role, index) => {
            const isExpanded = expandedIndex === index;
            const isPreset = presets.some((p) => p.name === role.name);

            return (
              <div
                key={index}
                className="rounded-lg border overflow-hidden"
                style={{ borderColor: "rgba(255,255,255,0.08)" }}
              >
                {/* Role header */}
                <button
                  type="button"
                  onClick={() => toggleExpand(index)}
                  className="flex items-center justify-between w-full px-4 py-3 text-left transition-colors hover:bg-white/[0.02]"
                  style={{ backgroundColor: "rgba(255,255,255,0.02)" }}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <Shield
                      className="h-4 w-4 shrink-0"
                      style={{ color: "var(--brand-primary)" }}
                    />
                    <span
                      className="text-sm font-medium"
                      style={{ color: "var(--text-primary)" }}
                    >
                      {role.name || "Custom Role"}
                    </span>
                    {role.description && (
                      <span
                        className="text-xs truncate"
                        style={{ color: "var(--text-muted)" }}
                      >
                        — {role.description}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span
                      className="text-[11px] px-1.5 py-0.5 rounded"
                      style={{
                        backgroundColor: "rgba(255,255,255,0.06)",
                        color: "var(--text-muted)",
                      }}
                    >
                      {role.policies?.length || 0} policies
                    </span>
                    <span
                      className="text-[11px] px-1.5 py-0.5 rounded"
                      style={{
                        backgroundColor: "rgba(255,255,255,0.06)",
                        color: "var(--text-muted)",
                      }}
                    >
                      {role.teams?.length || 0} teams
                    </span>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        removeRole(index);
                      }}
                      className="p-1 rounded-md transition-colors hover:bg-[rgba(255,255,255,0.06)]"
                      style={{ color: "var(--text-muted)" }}
                      aria-label={`Remove ${role.name || "role"}`}
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                    {isExpanded ? (
                      <ChevronUp
                        className="h-4 w-4"
                        style={{ color: "var(--text-muted)" }}
                      />
                    ) : (
                      <ChevronDown
                        className="h-4 w-4"
                        style={{ color: "var(--text-muted)" }}
                      />
                    )}
                  </div>
                </button>

                {/* Role body (expanded) */}
                {isExpanded && (
                  <div
                    className="px-4 py-4 space-y-4"
                    style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}
                  >
                    {/* Custom role name */}
                    {!isPreset && (
                      <div className="space-y-1.5">
                        <Label className="text-xs" style={{ color: "var(--text-muted)" }}>
                          Role Name
                        </Label>
                        <Input
                          value={role.name}
                          onChange={(e) => {
                            const newName = e.target.value
                              .toLowerCase()
                              .replace(/\s+/g, "-");
                            updateRole(index, { name: newName });
                          }}
                          placeholder="Role name (e.g., deployer)"
                        />
                      </div>
                    )}

                    {/* Policy Rules */}
                    <div>
                      <Label
                        className="text-xs mb-2 block"
                        style={{ color: "var(--text-muted)" }}
                      >
                        Policy Rules
                      </Label>
                      <PolicyRulesTable
                        key={`${index}-${role.name}`}
                        projectId={projectName}
                        roleName={role.name || "custom-role"}
                        policies={role.policies || []}
                        onPoliciesChange={(policies) =>
                          updateRole(index, { policies })
                        }
                        canEdit={true}
                      />
                    </div>

                    {/* Teams (resolve to OIDC groups server-side; reusable) */}
                    <TeamPicker
                      selected={role.teams || []}
                      onChange={(teams) =>
                        updateRole(index, {
                          teams: teams.length > 0 ? teams : undefined,
                        })
                      }
                      canEdit={true}
                    />

                    {/* Destination Scope (only when 2+ destinations) */}
                    {destinations.length > 1 && (
                      <DestinationScopeSelector
                        projectDestinations={destinations}
                        selectedDestinations={role.destinations || []}
                        onChange={(dests) =>
                          updateRole(index, {
                            destinations:
                              dests.length > 0 ? dests : undefined,
                          })
                        }
                        canEdit={true}
                      />
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Empty state */}
      {roles.length === 0 && (
        <div
          className="flex flex-col items-center gap-2 py-6 rounded-md border border-dashed"
          style={{
            borderColor: "rgba(255,255,255,0.10)",
            color: "var(--text-muted)",
          }}
        >
          <Shield className="h-5 w-5" />
          <p className="text-xs text-center">
            No roles yet. Use the presets above or add a custom role.
            <br />
            You can always add roles later from the project detail page.
          </p>
        </div>
      )}
    </div>
  );
}
