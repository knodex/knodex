// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Delete project confirmation dialog with type-to-confirm
 */
import { useState } from "react";
import { AlertTriangle, Loader2 } from "@/lib/icons";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Project } from "@/types/project";

interface DeleteProjectDialogProps {
  project: Project;
  isOpen: boolean;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  isDeleting?: boolean;
  error?: Error | null;
}

export function DeleteProjectDialog({
  project,
  isOpen,
  onConfirm,
  onCancel,
  isDeleting = false,
  error,
}: DeleteProjectDialogProps) {
  const [confirmName, setConfirmName] = useState("");

  const isConfirmValid = confirmName === project.name;

  const handleConfirm = async () => {
    if (!isConfirmValid) return;
    await onConfirm();
  };

  const handleCancel = () => {
    setConfirmName("");
    onCancel();
  };

  const roleCount = project.roles?.length || 0;

  return (
    <AlertDialog open={isOpen} onOpenChange={(open) => !open && handleCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Delete Project
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p>
                Are you sure you want to delete the project{" "}
                <strong className="text-foreground">{project.name}</strong>?
              </p>

              {roleCount > 0 && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md text-sm">
                  <p className="font-medium text-destructive mb-2">
                    Warning: This project contains:
                  </p>
                  <ul className="list-disc list-inside space-y-1 text-muted-foreground">
                    <li>
                      {roleCount} role definition{roleCount !== 1 ? "s" : ""}
                    </li>
                  </ul>
                </div>
              )}

              <p className="text-sm">
                This action cannot be undone. All role bindings and policies
                associated with this project will be permanently deleted.
              </p>

              {error && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
                  <p className="text-sm text-destructive">
                    {error.message || "Failed to delete project"}
                  </p>
                </div>
              )}

              <div className="pt-2">
                <Label htmlFor="confirm-name" className="text-sm">
                  Type <code className="text-destructive">{project.name}</code> to
                  confirm deletion
                </Label>
                <Input
                  id="confirm-name"
                  value={confirmName}
                  onChange={(e) => setConfirmName(e.target.value)}
                  placeholder={project.name}
                  className="mt-2"
                  disabled={isDeleting}
                  autoComplete="off"
                />
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" onClick={handleCancel} disabled={isDeleting}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={!isConfirmValid || isDeleting}
          >
            {isDeleting && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Delete Project
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
