// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useFormContext } from "react-hook-form";
import { Check } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { RESERVED_BASICS_KEYS, type DeployTab } from "@/lib/build-tabs";

interface DeployTabsProps {
  tabs: DeployTab[];
  activeId: string;
  onSelect: (id: string) => void;
  visitedIds: Set<string>;
}

type BadgeState = "error" | "valid" | "untouched";

function ownedRootKeys(tab: DeployTab): string[] {
  switch (tab.kind) {
    case "basics":
      return [...RESERVED_BASICS_KEYS];
    case "general":
      return Object.keys(tab.properties ?? {});
    case "schema":
      return [tab.id.startsWith("rgd-") ? tab.id.slice(4) : tab.id];
    case "review":
    default:
      return [];
  }
}

export function DeployTabs({
  tabs,
  activeId,
  onSelect,
  visitedIds,
}: DeployTabsProps) {
  const { formState } = useFormContext();
  const errorKeys = useMemo(
    () => new Set(Object.keys(formState.errors ?? {})),
    [formState.errors]
  );

  return (
    <div
      role="tablist"
      aria-label="Deploy steps"
      className="inline-flex h-auto w-full items-center justify-start gap-1 rounded-md bg-muted/50 p-1 flex-wrap text-muted-foreground"
    >
      {tabs.map((tab) => {
        const owned = ownedRootKeys(tab);
        const hasError = owned.some((k) => errorKeys.has(k));
        const visited = visitedIds.has(tab.id);
        const state: BadgeState = hasError
          ? "error"
          : visited
            ? "valid"
            : "untouched";
        const isActive = activeId === tab.id;

        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            aria-controls={`deploy-panel-${tab.id}`}
            id={`deploy-tab-${tab.id}`}
            data-testid={`deploy-tab-${tab.id}`}
            data-state={isActive ? "active" : "inactive"}
            onClick={() => onSelect(tab.id)}
            className={cn(
              "inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all",
              "ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
              "disabled:pointer-events-none disabled:opacity-50",
              isActive
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <span className="flex items-center gap-2">
              <span>{tab.label}</span>
              <TabBadge state={state} tabId={tab.id} />
            </span>
          </button>
        );
      })}
    </div>
  );
}

interface TabBadgeProps {
  state: BadgeState;
  tabId: string;
}

function TabBadge({ state, tabId }: TabBadgeProps) {
  if (state === "error") {
    return (
      <span
        data-testid={`deploy-tab-badge-${tabId}`}
        data-state="error"
        aria-label="Tab has validation errors"
        className={cn(
          "inline-flex h-2 w-2 shrink-0 rounded-full",
          "bg-destructive"
        )}
      />
    );
  }
  if (state === "valid") {
    return (
      <span
        data-testid={`deploy-tab-badge-${tabId}`}
        data-state="valid"
        aria-label="Tab is valid"
        className={cn(
          "inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full",
          "bg-emerald-500/20 text-emerald-500"
        )}
      >
        <Check className="h-2.5 w-2.5" />
      </span>
    );
  }
  return (
    <span
      data-testid={`deploy-tab-badge-${tabId}`}
      data-state="untouched"
      aria-label="Tab not yet visited"
      className={cn(
        "inline-flex h-2 w-2 shrink-0 rounded-full",
        "bg-muted-foreground/40"
      )}
    />
  );
}
