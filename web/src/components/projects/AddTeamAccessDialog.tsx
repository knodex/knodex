// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * AddTeamAccessDialog — grant a Team access to a project by binding it to one
 * of the project's roles. Team-centric counterpart to the role editor: the
 * operator picks a Team (resolves to its OIDC groups server-side) and the role
 * it should have. On confirm, the team is added to that role's `teams[]`.
 */
import { useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ProjectRole } from "@/types/project";
import type { Team } from "@/types/team";

interface AddTeamAccessDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Teams not yet bound to the project (selectable). */
  availableTeams: Team[];
  /** Whether any teams exist at all (drives the "create a team" hint). */
  hasAnyTeams: boolean;
  /** The project's roles (the access level to assign). */
  roles: ProjectRole[];
  onAdd: (team: string, role: string) => Promise<void>;
  isSubmitting: boolean;
}

export function AddTeamAccessDialog({
  open,
  onOpenChange,
  availableTeams,
  hasAnyTeams,
  roles,
  onAdd,
  isSubmitting,
}: AddTeamAccessDialogProps) {
  const [team, setTeam] = useState("");
  const [role, setRole] = useState("");
  const [error, setError] = useState<string | null>(null);

  const reset = () => {
    setTeam("");
    setRole("");
    setError(null);
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) reset();
    onOpenChange(next);
  };

  const handleAdd = async () => {
    if (!team || !role) return;
    setError(null);
    try {
      await onAdd(team, role);
      reset();
      onOpenChange(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to add team");
    }
  };

  const noRoles = roles.length === 0;
  const noneAvailable = availableTeams.length === 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent data-testid="add-team-access-dialog">
        <DialogHeader>
          <DialogTitle>Add team access</DialogTitle>
          <DialogDescription>
            Bind a team to a role. The team&apos;s OIDC groups resolve to project
            permissions automatically.
          </DialogDescription>
        </DialogHeader>

        {noRoles ? (
          <p className="text-sm text-muted-foreground py-2">
            This project has no roles defined yet. Roles determine the access a
            team receives.
          </p>
        ) : !hasAnyTeams ? (
          <p className="text-sm text-muted-foreground py-2">
            No teams exist yet.{" "}
            <Link to="/settings/teams" className="underline" onClick={() => onOpenChange(false)}>
              Create a team
            </Link>{" "}
            first.
          </p>
        ) : noneAvailable ? (
          <p className="text-sm text-muted-foreground py-2">
            All teams already have access to this project.
          </p>
        ) : (
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <label htmlFor="add-team-team" className="text-sm font-medium text-foreground">Team</label>
              <Select value={team} onValueChange={setTeam}>
                <SelectTrigger id="add-team-team">
                  <SelectValue placeholder="Select a team" />
                </SelectTrigger>
                <SelectContent>
                  {availableTeams.map((t) => (
                    <SelectItem key={t.name} value={t.name} data-testid={`team-option-${t.name}`}>
                      {t.name}
                      {t.oidcGroups.length > 0 && (
                        <span className="text-muted-foreground">
                          {" "}
                          · {t.oidcGroups.length} OIDC group
                          {t.oidcGroups.length !== 1 ? "s" : ""}
                        </span>
                      )}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label htmlFor="add-team-role" className="text-sm font-medium text-foreground">Role</label>
              <Select value={role} onValueChange={setRole}>
                <SelectTrigger id="add-team-role">
                  <SelectValue placeholder="Select a role" />
                </SelectTrigger>
                <SelectContent>
                  {roles.map((r) => (
                    <SelectItem key={r.name} value={r.name} data-testid={`role-select-option-${r.name}`}>
                      {r.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            data-testid="add-team-submit"
            onClick={handleAdd}
            disabled={isSubmitting || noRoles || noneAvailable || !hasAnyTeams || !team || !role}
          >
            {isSubmitting ? "Adding…" : "Add team"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
