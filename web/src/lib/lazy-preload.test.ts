// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { lazyWithPreload } from "./lazy-preload";

describe("lazyWithPreload", () => {
  it("returns a lazy component with a preload method", () => {
    const factory = vi.fn(() =>
      Promise.resolve({ default: () => null })
    );

    const Component = lazyWithPreload(factory);

    // Should have the standard lazy component shape
    expect(Component).toBeDefined();
    expect(Component.$$typeof).toBeDefined(); // React lazy marker

    // Should expose preload
    expect(typeof Component.preload).toBe("function");
  });

  it("preload() calls the factory function", async () => {
    const factory = vi.fn(() =>
      Promise.resolve({ default: () => null })
    );

    const Component = lazyWithPreload(factory);

    // Factory should not be called until preload or render
    expect(factory).not.toHaveBeenCalled();

    await Component.preload();

    expect(factory).toHaveBeenCalledTimes(1);
  });

  it("preload() returns the module with default export", async () => {
    const FakeComponent = () => null;
    const factory = () => Promise.resolve({ default: FakeComponent });

    const Component = lazyWithPreload(factory);
    const result = await Component.preload();

    expect(result.default).toBe(FakeComponent);
  });

  it("caches the preload promise — factory is called only once", async () => {
    const factory = vi.fn(() =>
      Promise.resolve({ default: () => null })
    );

    const Component = lazyWithPreload(factory);

    const p1 = Component.preload();
    const p2 = Component.preload();

    expect(p1).toBe(p2);
    await p1;

    expect(factory).toHaveBeenCalledTimes(1);
  });
});
