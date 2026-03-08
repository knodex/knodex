// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Delete role confirmation dialog
 * Simpler than DeleteRepositoryDialog - just confirm/cancel, no type-to-confirm
 */
import { AlertTriangle, Loader2 } from "lucide-react";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

interface DeleteRoleDialogProps {
  roleName: string;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  isOpen: boolean;
  isDeleting?: boolean;
  error?: string | null;
}

export function DeleteRoleDialog({
  roleName,
  onConfirm,
  onCancel,
  isOpen,
  isDeleting = false,
  error = null,
}: DeleteRoleDialogProps) {
  const handleOpenChange = (open: boolean) => {
    if (!open && !isDeleting) {
      onCancel();
    }
  };

  return (
    <AlertDialog open={isOpen} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="max-w-md">
        <AlertDialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-destructive/10">
              <AlertTriangle className="h-5 w-5 text-destructive" />
            </div>
            <AlertDialogTitle>Delete Role</AlertDialogTitle>
          </div>
          <AlertDialogDescription className="sr-only">
            Confirm deletion of role {roleName}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete the role{" "}
            <strong className="text-foreground">{roleName}</strong>?
          </p>

          <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3">
            <p className="text-sm font-medium text-destructive mb-2">Warning:</p>
            <ul className="text-xs text-destructive/80 space-y-1 list-disc list-inside">
              <li>This action cannot be undone</li>
              <li>All policy rules for this role will be removed</li>
              <li>Users in OIDC groups assigned to this role will lose access</li>
            </ul>
          </div>

          {error && (
            <p className="text-sm text-destructive font-medium">{error}</p>
          )}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={isDeleting}
          >
            {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {isDeleting ? "Deleting..." : "Delete Role"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
