// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo } from "react";

export interface Tab<T extends string> {
  id: T;
  label: string;
  icon: React.ReactNode;
}

export interface ConditionalTab<T extends string> {
  condition: boolean;
  tab: Tab<T>;
  position?: number; // index to insert at; undefined = append. Note: positions are applied sequentially, so earlier inserts shift indices for later ones.
}

export function useDynamicTabs<T extends string>(
  baseTabs: Tab<T>[],
  conditionalTabs: ConditionalTab<T>[],
  defaultTab: T
): {
  tabs: Tab<T>[];
  activeTab: T;
  setActiveTab: (id: T) => void;
} {
  const [selectedTab, setSelectedTab] = useState<T>(defaultTab);

  const tabs = useMemo(() => {
    const result = [...baseTabs];
    const toInsert = conditionalTabs
      .filter((c) => c.condition)
      .sort((a, b) => (a.position ?? Infinity) - (b.position ?? Infinity));
    for (const { tab, position } of toInsert) {
      if (position !== undefined) {
        result.splice(position, 0, tab);
      } else {
        result.push(tab);
      }
    }
    return result;
  }, [baseTabs, conditionalTabs]);

  // Fall back to default if selected tab was removed from the dynamic list
  const activeTab = tabs.some((t) => t.id === selectedTab)
    ? selectedTab
    : defaultTab;

  return { tabs, activeTab, setActiveTab: setSelectedTab };
}
