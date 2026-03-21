// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect } from "react";
import { toast } from "sonner";
import { useUpdateSecret } from "@/hooks/useSecrets";
import { ApiError } from "@/api/client";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { KeyValueEditor } from "./KeyValueEditor";
import { createPairId, type KeyValuePair } from "./keyValueTypes";
import type { SecretDetail } from "@/types/secret";

interface EditSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  secret: SecretDetail;
  project: string;
}

export function EditSecretDialog({ open, onOpenChange, secret, project }: EditSecretDialogProps) {
  const [pairs, setPairs] = useState<KeyValuePair[]>([]);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const updateMutation = useUpdateSecret();

  // Initialize pairs from secret data when dialog opens
  useEffect(() => {
    if (open) {
      const initialPairs: KeyValuePair[] = Object.keys(secret.data).map((key) => ({
        id: createPairId(),
        key,
        value: "",
        visible: false,
      }));
      if (initialPairs.length === 0) {
        initialPairs.push({ id: createPairId(), key: "", value: "", visible: false });
      }
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional reset when dialog opens
      setPairs(initialPairs);
      setValidationErrors({});
    }
  }, [open, secret.data]);

  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (!isOpen) {
        setValidationErrors({});
      }
      onOpenChange(isOpen);
    },
    [onOpenChange]
  );

  const validate = useCallback((): boolean => {
    const errors: Record<string, string> = {};

    // For edit, only pairs with a non-empty value count as updates.
    // Pairs with a key but empty value are skipped (unchanged on server).
    const pairsWithValues = pairs.filter((p) => p.key.trim() && p.value);
    // Also check for new keys (key + value both filled in)
    const nonEmptyPairs = pairs.filter((p) => p.key.trim() || p.value.trim());
    if (pairsWithValues.length === 0) {
      errors.keys = "At least one key must have a new value";
    } else {
      const keys = new Set<string>();
      for (const pair of nonEmptyPairs) {
        if (!pair.key.trim()) {
          errors.keys = "All keys must be non-empty";
          break;
        }
        if (keys.has(pair.key.trim())) {
          errors.keys = `Duplicate key: ${pair.key.trim()}`;
          break;
        }
        keys.add(pair.key.trim());
      }
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  }, [pairs]);

  const handleSubmit = useCallback(async () => {
    if (!validate()) return;

    // Only send keys where the user provided a new value.
    // Keys left with empty values are not sent — their server values remain unchanged.
    const data: Record<string, string> = {};
    for (const pair of pairs) {
      if (pair.key.trim() && pair.value) {
        data[pair.key.trim()] = pair.value;
      }
    }

    try {
      await updateMutation.mutateAsync({
        name: secret.name,
        project,
        namespace: secret.namespace,
        data,
      });
      toast.success(`Secret "${secret.name}" updated successfully`);
      handleOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 404) {
          toast.error("Secret not found — it may have been deleted");
        } else if (err.status === 403) {
          toast.error("Permission denied");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to update secret");
      }
    }
  }, [validate, pairs, secret.name, secret.namespace, project, updateMutation, handleOpenChange]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Edit Secret</DialogTitle>
          <DialogDescription>
            Update values for "{secret.name}". Only keys with new values will be updated.
            Leave a value empty to keep it unchanged.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-4 py-2">
            <KeyValueEditor
              pairs={pairs}
              onChange={setPairs}
              errors={validationErrors}
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={updateMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? "Updating..." : "Update"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
