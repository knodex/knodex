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
import { SecretMetadataFields } from "./SecretMetadataFields";
import type { SecretDetail, SecretMetadata } from "@/types/secret";
import { validateDocsUrl } from "@/lib/url-utils";
import { dateInputToExpiresAt, emptyMetadataValue, expiresAtToDateInput, type SecretMetadataFormValue } from "@/lib/secret-metadata";

interface EditSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  secret: SecretDetail;
}

export function EditSecretDialog({ open, onOpenChange, secret }: EditSecretDialogProps) {
  const [pairs, setPairs] = useState<KeyValuePair[]>([]);
  const [metadata, setMetadata] = useState<SecretMetadataFormValue>(emptyMetadataValue);
  /** Snapshot of the metadata as the dialog opened, used to decide whether
   *  the user touched the section and we need to send `metadata` in the PUT. */
  const [initialMetadata, setInitialMetadata] = useState<SecretMetadataFormValue>(emptyMetadataValue);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const updateMutation = useUpdateSecret();

  // Initialize pairs + metadata from secret when dialog opens
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
      const md: SecretMetadataFormValue = {
        rotation: secret.metadata?.rotation ?? "",
        docsUrl: secret.metadata?.docsUrl ?? "",
        expiresAtDate: expiresAtToDateInput(secret.metadata?.expiresAt),
      };
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional reset when dialog opens
      setPairs(initialPairs);
      setMetadata(md);
      setInitialMetadata(md);
      setValidationErrors({});
    }
  }, [open, secret.data, secret.metadata]);

  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (!isOpen) {
        setValidationErrors({});
      }
      onOpenChange(isOpen);
    },
    [onOpenChange]
  );

  const metadataChanged = useCallback((): boolean => {
    return (
      metadata.rotation !== initialMetadata.rotation ||
      metadata.docsUrl !== initialMetadata.docsUrl ||
      metadata.expiresAtDate !== initialMetadata.expiresAtDate
    );
  }, [metadata, initialMetadata]);

  const validate = useCallback((): boolean => {
    const errors: Record<string, string> = {};

    // For edit, only pairs with a non-empty value count as data updates.
    // Pairs with a key but empty value are skipped (unchanged on server).
    const pairsWithValues = pairs.filter((p) => p.key.trim() && p.value);
    const nonEmptyPairs = pairs.filter((p) => p.key.trim() || p.value.trim());

    // Require at least *some* change — either a new value or a metadata edit.
    // Metadata-only edits are legitimate (e.g., just updating the docs URL).
    if (pairsWithValues.length === 0 && !metadataChanged()) {
      errors.keys = "Enter a new value or change a metadata field to save";
    }

    {
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

    // Metadata is optional. Sanity-check the URL shape only when supplied.
    if (metadata.docsUrl) {
      const msg = validateDocsUrl(metadata.docsUrl);
      if (msg) errors["metadata:docsUrl"] = msg;
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  }, [pairs, metadata, metadataChanged]);

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

    // Only send `metadata` if the user actually touched the section.
    // Omitting the field tells the server to leave existing labels/annotations
    // exactly as they were (matches the contract documented on UpdateSecretRequest).
    let metadataPayload: SecretMetadata | undefined;
    if (metadataChanged()) {
      metadataPayload = {
        rotation: metadata.rotation || undefined,
        docsUrl: metadata.docsUrl.trim() || undefined,
        expiresAt: dateInputToExpiresAt(metadata.expiresAtDate),
      };
    }

    try {
      await updateMutation.mutateAsync({
        name: secret.name,
        namespace: secret.namespace,
        data,
        metadata: metadataPayload,
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
  }, [validate, pairs, secret.name, secret.namespace, metadata, metadataChanged, updateMutation, handleOpenChange]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[600px] max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>Edit Secret</DialogTitle>
          <DialogDescription>
            Update values for "{secret.name}". Only keys with new values will be updated —
            leave a value empty to keep it unchanged. Metadata fields are optional.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <div className="flex-1 space-y-4 overflow-y-auto py-2 pr-1">
            <KeyValueEditor
              pairs={pairs}
              onChange={setPairs}
              errors={validationErrors}
            />

            {/* Optional metadata: rotation policy, docs URL, expiration */}
            <SecretMetadataFields
              value={metadata}
              onChange={setMetadata}
              errors={validationErrors}
              idPrefix="edit-secret-metadata"
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
