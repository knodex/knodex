// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { ChevronDown } from "@/lib/icons";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { getEnforcementColors, type EnforcementAction } from "@/types/compliance";
import { ConfirmEnforcementDialog } from "./ConfirmEnforcementDialog";

/**
 * Enforcement action options with descriptions
 */
const ENFORCEMENT_OPTIONS: {
  value: EnforcementAction;
  label: string;
  description: string;
}[] = [
  {
    value: "dryrun",
    label: "Dry Run",
    description: "Audit violations without blocking or warning",
  },
  {
    value: "warn",
    label: "Warn",
    description: "Log warnings but allow resources to be created",
  },
  {
    value: "deny",
    label: "Deny",
    description: "Block resources that violate this constraint",
  },
];

interface EnforcementSelectorProps {
  /** Current enforcement action */
  currentAction: EnforcementAction;
  /** Constraint kind (e.g., K8sRequiredLabels) */
  constraintKind: string;
  /** Constraint name */
  constraintName: string;
  /** Callback when enforcement is changed */
  onEnforcementChange: (newAction: EnforcementAction) => Promise<void>;
  /** Whether the mutation is in progress */
  isUpdating?: boolean;
  /** Whether the user has permission to update */
  canUpdate?: boolean;
  /** Optional className */
  className?: string;
}

/**
 * Dropdown selector for constraint enforcement action with confirmation dialog
 * AC-UI-01: Dropdown showing current enforcement action
 * AC-UI-02: Options: dryrun, warn, deny with color coding
 * AC-UI-03: Disabled state when user lacks permission
 */
export function EnforcementSelector({
  currentAction,
  constraintKind,
  constraintName,
  onEnforcementChange,
  isUpdating = false,
  canUpdate = true,
  className,
}: EnforcementSelectorProps) {
  const [pendingAction, setPendingAction] = useState<EnforcementAction | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const handleSelect = (value: string) => {
    const newAction = value as EnforcementAction;
    if (newAction === currentAction) return;

    setPendingAction(newAction);
    setIsDialogOpen(true);
  };

  const handleConfirm = async () => {
    if (!pendingAction) return;

    try {
      await onEnforcementChange(pendingAction);
      setIsDialogOpen(false);
      setPendingAction(null);
    } catch {
      // Error handling is done in the parent component via mutation
      // Keep dialog open so user can see the error or retry
    }
  };

  const handleCancel = () => {
    setIsDialogOpen(false);
    setPendingAction(null);
  };

  const currentColors = getEnforcementColors(currentAction);

  return (
    <>
      <Select
        value={currentAction}
        onValueChange={handleSelect}
        disabled={!canUpdate || isUpdating}
      >
        <SelectTrigger
          className={cn(
            "w-[160px]",
            currentColors.bg,
            currentColors.border,
            currentColors.text,
            "font-medium",
            !canUpdate && "opacity-50 cursor-not-allowed",
            className
          )}
        >
          <SelectValue placeholder="Select enforcement">
            {ENFORCEMENT_OPTIONS.find((o) => o.value === currentAction)?.label || currentAction}
          </SelectValue>
          {isUpdating && (
            <span className="ml-2 animate-spin">
              <ChevronDown className="h-4 w-4" />
            </span>
          )}
        </SelectTrigger>
        <SelectContent>
          {ENFORCEMENT_OPTIONS.map((option) => {
            const colors = getEnforcementColors(option.value);
            return (
              <SelectItem
                key={option.value}
                value={option.value}
                className={cn(
                  "flex flex-col items-start py-2",
                  option.value === currentAction && "bg-accent"
                )}
              >
                <div className="flex items-center gap-2">
                  <span
                    className={cn(
                      "w-2 h-2 rounded-full",
                      option.value === "deny" && "bg-red-500",
                      option.value === "warn" && "bg-amber-500",
                      option.value === "dryrun" && "bg-blue-500"
                    )}
                  />
                  <span className={cn("font-medium", colors.text)}>
                    {option.label}
                  </span>
                </div>
                <span className="text-xs text-muted-foreground ml-4">
                  {option.description}
                </span>
              </SelectItem>
            );
          })}
        </SelectContent>
      </Select>

      <ConfirmEnforcementDialog
        open={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        constraintKind={constraintKind}
        constraintName={constraintName}
        currentAction={currentAction}
        newAction={pendingAction}
        onConfirm={handleConfirm}
        onCancel={handleCancel}
        isLoading={isUpdating}
      />
    </>
  );
}

export default EnforcementSelector;
