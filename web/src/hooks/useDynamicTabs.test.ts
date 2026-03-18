// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createElement } from "react";
import { useDynamicTabs } from "./useDynamicTabs";
import type { Tab, ConditionalTab } from "./useDynamicTabs";

const icon = createElement("span", null, "icon");

const baseTabs: Tab<"a" | "b" | "c" | "d">[] = [
  { id: "a", label: "Tab A", icon },
  { id: "b", label: "Tab B", icon },
];

describe("useDynamicTabs", () => {
  it("returns base tabs when no conditional tabs match", () => {
    const conditionalTabs: ConditionalTab<"a" | "b" | "c" | "d">[] = [
      { condition: false, tab: { id: "c", label: "Tab C", icon } },
    ];
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, conditionalTabs, "a")
    );
    expect(result.current.tabs).toHaveLength(2);
    expect(result.current.tabs.map((t) => t.id)).toEqual(["a", "b"]);
  });

  it("appends conditional tabs when condition is true and no position", () => {
    const conditionalTabs: ConditionalTab<"a" | "b" | "c" | "d">[] = [
      { condition: true, tab: { id: "c", label: "Tab C", icon } },
    ];
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, conditionalTabs, "a")
    );
    expect(result.current.tabs.map((t) => t.id)).toEqual(["a", "b", "c"]);
  });

  it("inserts conditional tab at specified position", () => {
    const conditionalTabs: ConditionalTab<"a" | "b" | "c" | "d">[] = [
      { condition: true, tab: { id: "c", label: "Tab C", icon }, position: 1 },
    ];
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, conditionalTabs, "a")
    );
    expect(result.current.tabs.map((t) => t.id)).toEqual(["a", "c", "b"]);
  });

  it("uses defaultTab as initial activeTab", () => {
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, [], "b")
    );
    expect(result.current.activeTab).toBe("b");
  });

  it("allows setting activeTab", () => {
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, [], "a")
    );
    act(() => {
      result.current.setActiveTab("b");
    });
    expect(result.current.activeTab).toBe("b");
  });

  it("resets to defaultTab when active tab is removed", () => {
    let showC = true;
    const { result, rerender } = renderHook(() =>
      useDynamicTabs(
        baseTabs,
        [{ condition: showC, tab: { id: "c" as const, label: "Tab C", icon } }],
        "a" as "a" | "b" | "c" | "d"
      )
    );

    // Select tab C
    act(() => {
      result.current.setActiveTab("c");
    });
    expect(result.current.activeTab).toBe("c");

    // Remove tab C
    showC = false;
    rerender();

    expect(result.current.activeTab).toBe("a");
  });

  it("handles multiple conditional tabs with mixed positions", () => {
    const conditionalTabs: ConditionalTab<"a" | "b" | "c" | "d">[] = [
      { condition: true, tab: { id: "c", label: "Tab C", icon }, position: 0 },
      { condition: true, tab: { id: "d", label: "Tab D", icon } },
    ];
    const { result } = renderHook(() =>
      useDynamicTabs(baseTabs, conditionalTabs, "a")
    );
    expect(result.current.tabs.map((t) => t.id)).toEqual(["c", "a", "b", "d"]);
  });

  it("falls back to defaultTab when selected tab is not in the resolved list (invalid default)", () => {
    // defaultTab "d" is not in baseTabs and no conditional adds it — activeTab should be "d"
    // but since "d" is also not in tabs, the derived value returns "d" (the fallback itself).
    // The important guarantee: activeTab is always the defaultTab value, never undefined.
    const { result } = renderHook(() =>
      useDynamicTabs(
        baseTabs,
        [],
        "d" as "a" | "b" | "c" | "d"
      )
    );
    // activeTab is defaultTab even when defaultTab isn't in the tabs array
    expect(result.current.activeTab).toBe("d");
    // tabs remains unchanged (only baseTabs)
    expect(result.current.tabs.map((t) => t.id)).toEqual(["a", "b"]);
  });
});
