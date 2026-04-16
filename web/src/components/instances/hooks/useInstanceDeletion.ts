// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useDeleteInstance } from "@/hooks/useInstances";
import { showSuccessToast } from "@/lib/toast-helpers";
import type { Instance } from "@/types/rgd";

export function useInstanceDeletion(instance: Instance, onDeleted?: () => void) {
  const deleteInstance = useDeleteInstance();

  const handleDelete = useCallback(async () => {
    try {
      await deleteInstance.mutateAsync({
        namespace: instance.namespace,
        kind: instance.kind,
        name: instance.name,
      });
      showSuccessToast(`"${instance.name}" deleted`);
      onDeleted?.();
    } catch {
      // Error is displayed in the dialog via deleteInstance.error
    }
  }, [deleteInstance, instance.namespace, instance.kind, instance.name, onDeleted]);

  return {
    handleDelete,
    deleteInstance,
  };
}
