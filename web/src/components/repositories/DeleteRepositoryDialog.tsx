// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Delete repository configuration confirmation dialog
 * Uses shadcn AlertDialog for accessible destructive action confirmation
 */
import { useState, useEffect } from "react";
import { AlertTriangle, Loader2 } from "@/lib/icons";
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
import { Input } from "@/components/ui/input";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { RepositoryConfig } from "@/types/repository";

interface DeleteRepositoryDialogProps {
  repository: RepositoryConfig;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  isOpen: boolean;
  isDeleting?: boolean;
  error?: Error | null;
}

export function DeleteRepositoryDialog({
  repository,
  onConfirm,
  onCancel,
  isOpen,
  isDeleting = false,
  error = null,
}: DeleteRepositoryDialogProps) {
  const [confirmText, setConfirmText] = useState("");

  // Reset confirmation text when dialog opens/closes
  useEffect(() => {
    if (!isOpen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional reset when dialog closes
      setConfirmText("");
    }
  }, [isOpen]);

  const handleDelete = async () => {
    await onConfirm();
    setConfirmText("");
  };

  const handleOpenChange = (open: boolean) => {
    if (!open && !isDeleting) {
      onCancel();
    }
  };

  const isConfirmed = confirmText === repository.name;

  return (
    <AlertDialog open={isOpen} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="max-w-md">
        <AlertDialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-destructive/10">
              <AlertTriangle className="h-5 w-5 text-destructive" />
            </div>
            <AlertDialogTitle>Delete Repository</AlertDialogTitle>
          </div>
          <AlertDialogDescription className="sr-only">
            Confirm deletion of {repository.name}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-4">
          {error && (
            <Alert variant="destructive" showIcon>
              <AlertTitle>Failed to delete repository</AlertTitle>
              <AlertDescription>
                {error.message || "An error occurred while deleting the repository"}
              </AlertDescription>
            </Alert>
          )}

          <p className="text-sm text-muted-foreground">
            This action <strong>cannot be undone</strong>. This will permanently delete the{" "}
            <strong>{repository.name}</strong> repository configuration.
          </p>

          <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3">
            <p className="text-sm font-medium text-destructive mb-2">Warning:</p>
            <ul className="text-xs text-destructive/80 space-y-1 list-disc list-inside">
              <li>Repository: {repository.repoURL || `${repository.owner}/${repository.repo}`}</li>
              <li>GitOps deployments to this repository will stop working</li>
            </ul>
          </div>

          <div className="space-y-2">
            <label htmlFor="confirmText" className="text-sm font-medium">
              Type <code className="font-mono text-destructive">{repository.name}</code> to
              confirm
            </label>
            <Input
              id="confirmText"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={repository.name}
              disabled={isDeleting}
              className="font-mono"
            />
          </div>
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={!isConfirmed || isDeleting}
          >
            {isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {isDeleting ? "Deleting..." : "Delete Repository"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
