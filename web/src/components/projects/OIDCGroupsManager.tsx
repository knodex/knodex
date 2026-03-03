/**
 * OIDCGroupsManager - Manage OIDC group bindings for project roles
 * Implements ArgoCD-style group management UI
 *
 * Groups are OIDC group IDs (e.g., Azure AD Object IDs or OIDC group names)
 * that are mapped to project roles. Users in these groups automatically
 * receive the associated role permissions.
 */
import { useState, useCallback } from "react";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface OIDCGroupsManagerProps {
  /** Current groups assigned to the role */
  groups: string[];
  /** Callback when groups are updated */
  onGroupsChange: (groups: string[]) => void;
  /** Whether the user can edit groups */
  canEdit: boolean;
  /** Whether the component is in a loading/saving state */
  isLoading?: boolean;
}

/**
 * Validate an OIDC group identifier
 * Returns error message or null if valid
 */
function validateGroupId(groupId: string): string | null {
  if (!groupId.trim()) {
    return "Group ID cannot be empty";
  }

  // Basic validation: no spaces at start/end, reasonable length
  if (groupId.trim() !== groupId) {
    return "Group ID cannot have leading/trailing spaces";
  }

  if (groupId.length > 256) {
    return "Group ID is too long (max 256 characters)";
  }

  // Allow UUIDs (Azure AD Object IDs), alphanumeric with dashes/underscores
  // Be permissive since different IdPs have different formats
  if (!/^[a-zA-Z0-9._@\-/]+$/.test(groupId)) {
    return "Group ID contains invalid characters";
  }

  return null;
}

export function OIDCGroupsManager({
  groups,
  onGroupsChange,
  canEdit,
  isLoading = false,
}: OIDCGroupsManagerProps) {
  const [newGroupId, setNewGroupId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [showAddInput, setShowAddInput] = useState(false);

  // Handle adding a new group
  const handleAddGroup = useCallback(() => {
    const trimmedId = newGroupId.trim();

    // Validate
    const validationError = validateGroupId(trimmedId);
    if (validationError) {
      setError(validationError);
      return;
    }

    // Check for duplicates
    if (groups.includes(trimmedId)) {
      setError("This group is already assigned");
      return;
    }

    // Add the group
    onGroupsChange([...groups, trimmedId]);
    setNewGroupId("");
    setError(null);
    setShowAddInput(false);
  }, [newGroupId, groups, onGroupsChange]);

  // Handle removing a group
  const handleRemoveGroup = useCallback(
    (groupId: string) => {
      onGroupsChange(groups.filter((g) => g !== groupId));
    },
    [groups, onGroupsChange]
  );

  // Handle Enter key in input
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        e.preventDefault();
        handleAddGroup();
      } else if (e.key === "Escape") {
        setShowAddInput(false);
        setNewGroupId("");
        setError(null);
      }
    },
    [handleAddGroup]
  );

  // Handle cancel
  const handleCancel = useCallback(() => {
    setShowAddInput(false);
    setNewGroupId("");
    setError(null);
  }, []);

  return (
    <div className="space-y-3">
      <Label className="text-muted-foreground">OIDC Groups</Label>

      {/* Groups list */}
      {groups.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {groups.map((group) => (
            <Badge
              key={group}
              variant="secondary"
              className="flex items-center gap-1 py-1 px-2 font-mono text-xs"
            >
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="max-w-[200px] truncate cursor-default">
                    {group}
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  <p className="font-mono text-xs">{group}</p>
                </TooltipContent>
              </Tooltip>
              {canEdit && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      onClick={() => handleRemoveGroup(group)}
                      disabled={isLoading}
                      className="ml-1 hover:text-destructive focus:outline-none focus:ring-1 focus:ring-destructive rounded"
                      aria-label={`Remove group ${group}`}
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Remove group</p>
                  </TooltipContent>
                </Tooltip>
              )}
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground italic">
          No groups assigned
        </p>
      )}

      {/* Add group input */}
      {canEdit && (
        <div className="space-y-2">
          {showAddInput ? (
            <div className="flex flex-col gap-2">
              <div className="flex gap-2">
                <Input
                  value={newGroupId}
                  onChange={(e) => {
                    setNewGroupId(e.target.value);
                    setError(null);
                  }}
                  onKeyDown={handleKeyDown}
                  placeholder="Enter OIDC group ID (e.g., UUID or group name)"
                  className="flex-1 font-mono text-sm h-8"
                  disabled={isLoading}
                  autoFocus
                />
                <Button
                  size="sm"
                  onClick={handleAddGroup}
                  disabled={isLoading || !newGroupId.trim()}
                  className="h-8"
                >
                  Add
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={handleCancel}
                  disabled={isLoading}
                  className="h-8"
                >
                  Cancel
                </Button>
              </div>
              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}
              <p className="text-xs text-muted-foreground">
                Enter the OIDC group identifier from your identity provider.
                For Azure AD, use the Object ID (UUID format).
              </p>
            </div>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowAddInput(true)}
              disabled={isLoading}
              className="h-8"
            >
              <Plus className="h-4 w-4 mr-1" />
              Add Group
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
