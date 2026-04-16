// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import {
  Activity,
  Clock,
  Code,
  Puzzle,
  Link2,
  Boxes,
  Zap,
} from "@/lib/icons";
import { useRGDList } from "@/hooks/useRGDs";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab, ConditionalTab } from "@/hooks/useDynamicTabs";
import type { Instance } from "@/types/rgd";
import { createElement } from "react";

export type InstanceTabId = "status" | "addons" | "deployment-history" | "external-refs" | "spec" | "children" | "events";

const BASE_TABS: Tab<InstanceTabId>[] = [
  { id: "status", label: "Status", icon: createElement(Activity, { className: "h-4 w-4" }) },
  { id: "deployment-history", label: "Deployment History", icon: createElement(Clock, { className: "h-4 w-4" }) },
];

export function useInstanceTabs(
  instance: Instance,
  eventsCount: number,
  externalRefCount: number,
  hasSpec: boolean,
) {
  // Fetch add-ons count for tab visibility (React Query deduplicates with InstanceAddOns)
  const { data: addOnsData } = useRGDList(
    instance.kind ? { extendsKind: instance.kind, pageSize: 100 } : undefined
  );
  const addOnsCount = addOnsData?.totalCount ?? 0;

  // Build conditional tabs
  const conditionalTabs = useMemo<ConditionalTab<InstanceTabId>[]>(() => [
    {
      condition: addOnsCount > 0,
      tab: { id: "addons", label: `Add-ons (${addOnsCount})`, icon: createElement(Puzzle, { className: "h-4 w-4" }) },
      position: 1,
    },
    {
      condition: externalRefCount > 0,
      tab: { id: "external-refs", label: `External References (${externalRefCount})`, icon: createElement(Link2, { className: "h-4 w-4" }) },
      position: 2,
    },
    {
      condition: true,
      tab: { id: "children", label: "Resources", icon: createElement(Boxes, { className: "h-4 w-4" }) },
    },
    {
      condition: hasSpec,
      tab: { id: "spec", label: "Spec", icon: createElement(Code, { className: "h-4 w-4" }) },
    },
    // Events tab — always shown, includes instance + child resource K8s events
    {
      condition: true,
      tab: { id: "events", label: eventsCount > 0 ? `Events (${eventsCount})` : "Events", icon: createElement(Zap, { className: "h-4 w-4" }) },
      position: 1, // After Deployment History (base tab index 1)
    },
  ], [addOnsCount, externalRefCount, hasSpec, eventsCount]);

  const { tabs, activeTab, setActiveTab } = useDynamicTabs(BASE_TABS, conditionalTabs, "status" as InstanceTabId);

  return {
    tabs,
    activeTab,
    setActiveTab,
  };
}
