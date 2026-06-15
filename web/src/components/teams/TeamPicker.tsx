// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * TeamPicker — binds one or more Teams to a project role (Story 10.4, AC #4).
 *
 * Writes `roles[].teams[]`. Each bound Team is resolved to its OIDC groups
 * server-side (Story 10.2), so binding a team grants its groups through the one
 * Casbin enforcer. Teams are the sole identity-binding control (Story 10.7).
 * When no teams exist yet, a hint links to /settings/teams.
 */
import { useMemo } from "react";
import { Link } from "react-router-dom";
import { Label } from "@/components/ui/label";
import { MultiSelect } from "@/components/ui/multi-select";
import { Badge } from "@/components/ui/badge";
import { useTeams } from "@/hooks/useTeams";

interface TeamPickerProps {
  /** Currently bound team names. */
  selected: string[];
  /** Called with the new selection of team names. */
  onChange: (teams: string[]) => void;
  /** Whether the user can edit. */
  canEdit?: boolean;
}

export function TeamPicker({ selected, onChange, canEdit = true }: TeamPickerProps) {
  const { data, isLoading } = useTeams();
  const teams = useMemo(() => data?.items ?? [], [data]);

  const options = useMemo(
    () => teams.map((t) => ({ label: t.name, value: t.name })),
    [teams]
  );

  const chips =
    selected.length > 0 ? (
      <div className="flex flex-wrap gap-1.5" data-testid="team-picker-chips">
        {selected.map((name) => (
          <Badge key={name} variant="secondary" className="font-mono text-xs">
            {name}
          </Badge>
        ))}
      </div>
    ) : null;

  // Read-only view: show bound teams as badges, never a (dead) dropdown.
  if (!canEdit) {
    return (
      <div className="space-y-2" data-testid="team-picker">
        <Label className="text-muted-foreground">Teams</Label>
        {chips ?? (
          <p className="text-sm text-muted-foreground italic">No teams bound</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-2" data-testid="team-picker">
      <Label className="text-muted-foreground">Teams</Label>

      {!isLoading && teams.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No teams defined yet.{" "}
          <Link to="/settings/teams" className="underline">
            Create one in Settings → Teams
          </Link>{" "}
          to bind a reusable group set to this role.
        </p>
      ) : (
        <>
          <MultiSelect
            options={options}
            selected={selected}
            onChange={onChange}
            placeholder="Bind teams…"
          />
          {chips}
        </>
      )}
    </div>
  );
}
