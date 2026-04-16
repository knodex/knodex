// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { usePrefetchAfterIdle } from "./usePrefetchAfterIdle";

describe("usePrefetchAfterIdle", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Provide a requestIdleCallback stub
    vi.stubGlobal("requestIdleCallback", (cb: () => void) => setTimeout(cb, 0));
    vi.stubGlobal("cancelIdleCallback", (id: number) => clearTimeout(id));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("calls preload functions after delay + idle", () => {
    const preload1 = vi.fn(() => Promise.resolve());
    const preload2 = vi.fn(() => Promise.resolve());

    renderHook(() => usePrefetchAfterIdle([preload1, preload2], 2000));

    // Not called immediately
    expect(preload1).not.toHaveBeenCalled();
    expect(preload2).not.toHaveBeenCalled();

    // Advance past delay + idle callback
    vi.runAllTimers();

    expect(preload1).toHaveBeenCalledTimes(1);
    expect(preload2).toHaveBeenCalledTimes(1);
  });

  it("does not call preload functions before delay expires", () => {
    const preload = vi.fn(() => Promise.resolve());

    renderHook(() => usePrefetchAfterIdle([preload], 3000));

    vi.advanceTimersByTime(2999);
    expect(preload).not.toHaveBeenCalled();
  });

  it("only calls preload functions once across re-renders", () => {
    const preload = vi.fn(() => Promise.resolve());

    const { rerender } = renderHook(() => usePrefetchAfterIdle([preload], 1000));

    vi.runAllTimers();
    expect(preload).toHaveBeenCalledTimes(1);

    // Re-render should not trigger again
    rerender();
    vi.runAllTimers();
    expect(preload).toHaveBeenCalledTimes(1);
  });

  it("does nothing when given an empty array", () => {
    renderHook(() => usePrefetchAfterIdle([], 1000));

    vi.advanceTimersByTime(5000);
    // No error thrown, no side effects
  });

  it("cleans up timers on unmount", () => {
    const preload = vi.fn(() => Promise.resolve());

    const { unmount } = renderHook(() => usePrefetchAfterIdle([preload], 2000));

    // Unmount before delay fires
    unmount();

    vi.advanceTimersByTime(5000);
    expect(preload).not.toHaveBeenCalled();
  });

  it("uses default delay of 2000ms when not specified", () => {
    const preload = vi.fn(() => Promise.resolve());

    renderHook(() => usePrefetchAfterIdle([preload]));

    vi.advanceTimersByTime(1999);
    expect(preload).not.toHaveBeenCalled();

    // Advance past delay + idle callback
    vi.runAllTimers();
    expect(preload).toHaveBeenCalledTimes(1);
  });
});
