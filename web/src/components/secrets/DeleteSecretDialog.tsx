// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { AlertTriangle, Loader2 } from "lucide-react";
import { useDeleteSecret } from "@/hooks/useSecrets";
import { ApiError } from "@/api/client";
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

interface DeleteSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  secretName: string;
  secretNamespace: string;
  project: string;
  /** If true, navigate to /secrets after deletion. Default true (detail view behavior). */
  navigateOnDelete?: boolean;
}

export function DeleteSecretDialog({
  open,
  onOpenChange,
  secretName,
  secretNamespace,
  project,
  navigateOnDelete = true,
}: DeleteSecretDialogProps) {
  const navigate = useNavigate();
  const deleteMutation = useDeleteSecret();
  const [postDeleteWarnings, setPostDeleteWarnings] = useState<string[]>([]);

  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (!isOpen && !deleteMutation.isPending) {
        setPostDeleteWarnings([]);
        onOpenChange(false);
      }
    },
    [onOpenChange, deleteMutation.isPending]
  );

  const secretsListUrl = `/secrets?project=${encodeURIComponent(project)}`;

  const handleDelete = useCallback(async () => {
    try {
      const result = await deleteMutation.mutateAsync({
        name: secretName,
        project,
        namespace: secretNamespace,
      });

      if (result.warnings && result.warnings.length > 0) {
        // Show warnings inside the dialog before navigating
        setPostDeleteWarnings(result.warnings);
        toast.warning(`Secret "${secretName}" deleted with warnings`);
      } else {
        toast.success(`Secret "${secretName}" deleted successfully`);
        onOpenChange(false);
        if (navigateOnDelete) navigate(secretsListUrl);
      }
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 404) {
          toast.error("Secret not found — it may have already been deleted");
          onOpenChange(false);
          if (navigateOnDelete) navigate(secretsListUrl);
        } else if (err.status === 403) {
          toast.error("Permission denied");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to delete secret");
      }
    }
  }, [secretName, secretNamespace, project, deleteMutation, onOpenChange, navigate, navigateOnDelete, secretsListUrl]);

  const handleDismissWarnings = useCallback(() => {
    setPostDeleteWarnings([]);
    onOpenChange(false);
    if (navigateOnDelete) navigate(secretsListUrl);
  }, [onOpenChange, navigate, navigateOnDelete, secretsListUrl]);

  // Post-deletion warnings view — shown after DELETE returns warnings
  if (postDeleteWarnings.length > 0) {
    return (
      <AlertDialog open={open} onOpenChange={() => handleDismissWarnings()}>
        <AlertDialogContent className="max-w-md">
          <AlertDialogHeader>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-amber-500/10">
                <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
              </div>
              <AlertDialogTitle>Secret Deleted — Warnings</AlertDialogTitle>
            </div>
            <AlertDialogDescription className="sr-only">
              Secret {secretName} was deleted with warnings
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Secret <strong>{secretName}</strong> was deleted, but the following warnings were returned:
            </p>
            <div className="bg-amber-500/10 border border-amber-500/20 rounded-md p-3 space-y-1">
              {postDeleteWarnings.map((warning, i) => (
                <p key={i} className="text-sm text-amber-700 dark:text-amber-400">
                  {warning}
                </p>
              ))}
            </div>
          </div>

          <AlertDialogFooter>
            <Button onClick={handleDismissWarnings}>OK</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    );
  }

  // Standard confirmation view
  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="max-w-md">
        <AlertDialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-destructive/10">
              <AlertTriangle className="h-5 w-5 text-destructive" />
            </div>
            <AlertDialogTitle>Delete Secret</AlertDialogTitle>
          </div>
          <AlertDialogDescription className="sr-only">
            Confirm deletion of secret {secretName}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            This action <strong>cannot be undone</strong>. This will permanently delete the
            secret <strong>{secretName}</strong> from namespace <strong>{secretNamespace}</strong>.
          </p>

          <div className="bg-amber-500/10 border border-amber-500/20 rounded-md p-3">
            <p className="text-sm font-medium text-amber-700 dark:text-amber-400 mb-1">
              Warning:
            </p>
            <p className="text-xs text-amber-600 dark:text-amber-500">
              Instances referencing this secret may be affected. Any warnings will be shown after deletion.
            </p>
          </div>
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleteMutation.isPending}>Cancel</AlertDialogCancel>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {deleteMutation.isPending ? "Deleting..." : "Delete Secret"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
