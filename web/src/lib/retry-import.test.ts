// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { retryImport } from "./retry-import";

describe("retryImport", () => {
  it("returns the module on first success (no retry)", async () => {
    const fn = vi.fn().mockResolvedValue({ ok: true });
    const result = await retryImport(fn, { delayMs: 0 });
    expect(result).toEqual({ ok: true });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("recovers after one transient failure (default: 1 retry)", async () => {
    const fn = vi
      .fn()
      .mockRejectedValueOnce(new Error("ChunkLoadError"))
      .mockResolvedValue({ ok: true });
    const result = await retryImport(fn, { delayMs: 0 });
    expect(result).toEqual({ ok: true });
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("throws the last error after exhausting retries", async () => {
    const err = new Error("ChunkLoadError");
    const fn = vi.fn().mockRejectedValue(err);
    await expect(retryImport(fn, { delayMs: 0 })).rejects.toBe(err);
    // 1 initial attempt + 1 retry = 2 calls
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("honors a custom retries count", async () => {
    const fn = vi.fn().mockRejectedValue(new Error("nope"));
    await expect(retryImport(fn, { retries: 2, delayMs: 0 })).rejects.toThrow("nope");
    expect(fn).toHaveBeenCalledTimes(3); // 1 + 2 retries
  });

  it("does not retry when retries is 0", async () => {
    const fn = vi.fn().mockRejectedValue(new Error("nope"));
    await expect(retryImport(fn, { retries: 0, delayMs: 0 })).rejects.toThrow("nope");
    expect(fn).toHaveBeenCalledTimes(1);
  });
});
