// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Delete instance confirmation dialog with type-to-confirm
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
import type { Instance } from "@/types/rgd";

interface DeleteInstanceDialogProps {
  instance: Instance;
  isOpen: boolean;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  isDeleting?: boolean;
  error?: Error | null;
}

export function DeleteInstanceDialog({
  instance,
  isOpen,
  onConfirm,
  onCancel,
  isDeleting = false,
  error,
}: DeleteInstanceDialogProps) {
  const [confirmName, setConfirmName] = useState("");

  const isConfirmValid = confirmName === instance.name;

  const handleConfirm = async () => {
    if (!isConfirmValid) return;
    await onConfirm();
  };

  const handleCancel = () => {
    setConfirmName("");
    onCancel();
  };

  return (
    <AlertDialog open={isOpen} onOpenChange={(open) => !open && handleCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Delete {instance.name}?
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p>
                All resources managed by this instance will be deleted. This
                cannot be undone.
              </p>

              {error && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
                  <p className="text-sm text-destructive">
                    {error.message || "Failed to delete instance"}
                  </p>
                </div>
              )}

              <div className="pt-2">
                <Label htmlFor="confirm-instance-name" className="text-sm">
                  Type{" "}
                  <code className="text-destructive">{instance.name}</code> to
                  confirm deletion
                </Label>
                <Input
                  id="confirm-instance-name"
                  value={confirmName}
                  onChange={(e) => setConfirmName(e.target.value)}
                  placeholder={instance.name}
                  className="mt-2"
                  disabled={isDeleting}
                  autoComplete="off"
                  data-testid="confirm-name-input"
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
            data-testid="confirm-delete-button"
          >
            {isDeleting && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Delete Instance
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
