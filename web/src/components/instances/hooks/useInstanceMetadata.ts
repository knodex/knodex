// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useRGD } from "@/hooks/useRGDs";
import { useInstanceEvents } from "@/hooks/useHistory";
import { getInstanceUrl } from "../instance-utils";
import type { Instance } from "@/types/rgd";

export function useInstanceMetadata(instance: Instance) {
  const { data: parentRGD } = useRGD(instance.rgdName, instance.rgdNamespace);
  const { data: eventsData } = useInstanceEvents(
    instance.namespace,
    instance.kind,
    instance.name
  );

  const instanceUrl = getInstanceUrl(instance);
  const isGitOps = instance.deploymentMode === "gitops" || instance.deploymentMode === "hybrid";

  const kroState = (instance.status?.state as string) ?? "";
  const isTerminal = kroState === "DELETING" || kroState === "ERROR";
  const isDeleting = kroState === "DELETING";

  const hasSpec = !!(instance.spec && Object.keys(instance.spec).length > 0);

  const externalRefCount = useMemo(() => {
    const extRefObj = (instance.spec?.externalRef as Record<string, unknown>) ?? {};
    return Object.keys(extRefObj).filter(k => {
      const v = extRefObj[k];
      return v && typeof v === "object" && typeof (v as Record<string, unknown>).name === "string";
    }).length;
  }, [instance.spec]);

  const eventsCount = eventsData?.events?.length ?? 0;

  return {
    parentRGD,
    instanceUrl,
    isGitOps,
    kroState,
    isTerminal,
    isDeleting,
    hasSpec,
    externalRefCount,
    eventsCount,
  };
}
