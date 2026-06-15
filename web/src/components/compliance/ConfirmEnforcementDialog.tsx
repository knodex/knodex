// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { AlertTriangle, ShieldAlert, ShieldCheck } from "@/lib/icons";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { cn } from "@/lib/utils";
import { getEnforcementColors, type EnforcementAction } from "@/types/compliance";

interface ConfirmEnforcementDialogProps {
  /** Dialog open state */
  open: boolean;
  /** Callback when open state changes */
  onOpenChange: (open: boolean) => void;
  /** Constraint kind */
  constraintKind: string;
  /** Constraint name */
  constraintName: string;
  /** Current enforcement action */
  currentAction: EnforcementAction;
  /** New enforcement action to apply */
  newAction: EnforcementAction | null;
  /** Callback when user confirms the change */
  onConfirm: () => void;
  /** Callback when user cancels */
  onCancel: () => void;
  /** Whether the mutation is in progress */
  isLoading?: boolean;
}

/**
 * Get warning message based on enforcement change direction
 */
function getWarningMessage(
  currentAction: EnforcementAction,
  newAction: EnforcementAction
): { title: string; message: string; severity: "info" | "warning" | "danger" } {
  // Escalation: dryrun -> warn -> deny
  const actionSeverity: Record<EnforcementAction, number> = {
    dryrun: 0,
    warn: 1,
    deny: 2,
  };

  const isEscalation = actionSeverity[newAction] > actionSeverity[currentAction];

  if (newAction === "deny") {
    return {
      title: "Enable Blocking Mode",
      message: `Changing to "deny" will block any resources that violate this constraint.
                Existing violations will prevent new resources from being created until they are resolved.`,
      severity: "danger",
    };
  }

  if (newAction === "warn" && currentAction === "dryrun") {
    return {
      title: "Enable Warning Mode",
      message: `Changing to "warn" will emit warnings for any resources that violate this constraint.
                Resources will still be allowed, but users will see warning messages.`,
      severity: "warning",
    };
  }

  if (!isEscalation) {
    // De-escalation: making it less strict
    return {
      title: "Reduce Enforcement Level",
      message: `Changing from "${currentAction}" to "${newAction}" will reduce the enforcement level.
                Violations will still be tracked but with a less strict response.`,
      severity: "info",
    };
  }

  // Default escalation message
  return {
    title: "Increase Enforcement Level",
    message: `This will change the enforcement action from "${currentAction}" to "${newAction}".`,
    severity: "warning",
  };
}

/**
 * Confirmation dialog for enforcement action changes
 * AC-CONFIRM-01: Shows current vs new action
 * AC-CONFIRM-02: Warning message for deny escalation
 * AC-CONFIRM-03: Confirm and Cancel buttons
 * AC-CONFIRM-04: Loading state during update
 */
export function ConfirmEnforcementDialog({
  open,
  onOpenChange,
  constraintKind,
  constraintName,
  currentAction,
  newAction,
  onConfirm,
  onCancel,
  isLoading = false,
}: ConfirmEnforcementDialogProps) {
  if (!newAction) return null;

  const { title, message, severity } = getWarningMessage(currentAction, newAction);
  const currentColors = getEnforcementColors(currentAction);
  const newColors = getEnforcementColors(newAction);

  const SeverityIcon = severity === "danger" ? ShieldAlert : severity === "warning" ? AlertTriangle : ShieldCheck;

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <SeverityIcon
              className={cn(
                "h-5 w-5",
                severity === "danger" && "text-red-500",
                severity === "warning" && "text-amber-500",
                severity === "info" && "text-blue-500"
              )}
            />
            {title}
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                You are about to change the enforcement action for constraint:
              </p>
              <div className="flex items-center justify-center gap-4 py-2">
                <code className="px-2 py-1 rounded bg-muted text-sm font-mono">
                  {constraintKind}/{constraintName}
                </code>
              </div>
              <div className="flex items-center justify-center gap-3 py-2">
                <span
                  className={cn(
                    "px-3 py-1.5 rounded-md font-medium text-sm",
                    currentColors.bg,
                    currentColors.text,
                    currentColors.border,
                    "border"
                  )}
                >
                  {currentAction}
                </span>
                <span className="text-muted-foreground">&rarr;</span>
                <span
                  className={cn(
                    "px-3 py-1.5 rounded-md font-medium text-sm",
                    newColors.bg,
                    newColors.text,
                    newColors.border,
                    "border"
                  )}
                >
                  {newAction}
                </span>
              </div>
              <div
                className={cn(
                  "p-3 rounded-md text-sm",
                  severity === "danger" && "bg-red-50 text-red-800 dark:bg-red-950/30 dark:text-red-400",
                  severity === "warning" && "bg-amber-50 text-amber-800 dark:bg-amber-950/30 dark:text-amber-400",
                  severity === "info" && "bg-blue-50 text-blue-800 dark:bg-blue-950/30 dark:text-blue-400"
                )}
              >
                {message}
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onCancel} disabled={isLoading}>
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={isLoading}
            className={cn(
              severity === "danger" && "bg-red-600 hover:bg-red-700",
              severity === "warning" && "bg-amber-600 hover:bg-amber-700"
            )}
          >
            {isLoading ? "Updating..." : "Confirm Change"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default ConfirmEnforcementDialog;
