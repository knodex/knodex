// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useCopyToClipboard } from "./useCopyToClipboard";

describe("useCopyToClipboard", () => {
  let writeText: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("writes text to the clipboard and sets copied", async () => {
    const { result } = renderHook(() => useCopyToClipboard());

    expect(result.current.copied).toBe(false);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.copy("hello");
    });

    expect(ok).toBe(true);
    expect(writeText).toHaveBeenCalledWith("hello");
    expect(result.current.copied).toBe(true);
  });

  it("auto-resets copied after the default delay", async () => {
    const { result } = renderHook(() => useCopyToClipboard());

    await act(async () => {
      await result.current.copy("hello");
    });
    expect(result.current.copied).toBe(true);

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(result.current.copied).toBe(false);
  });

  it("honors a custom resetDelay", async () => {
    const { result } = renderHook(() =>
      useCopyToClipboard({ resetDelay: 1500 })
    );

    await act(async () => {
      await result.current.copy("hello");
    });

    act(() => {
      vi.advanceTimersByTime(1499);
    });
    expect(result.current.copied).toBe(true);

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.copied).toBe(false);
  });

  it("tracks a keyed copy via copiedKey", async () => {
    const { result } = renderHook(() => useCopyToClipboard());

    await act(async () => {
      await result.current.copy("secret-value", "TOKEN");
    });

    expect(result.current.copiedKey).toBe("TOKEN");

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(result.current.copiedKey).toBe(null);
  });

  it("calls onSuccess with the resolved key", async () => {
    const onSuccess = vi.fn();
    const { result } = renderHook(() => useCopyToClipboard({ onSuccess }));

    await act(async () => {
      await result.current.copy("x", "K");
    });

    expect(onSuccess).toHaveBeenCalledWith("K");
  });

  it("reports failure via onError and returns false when write rejects", async () => {
    const error = new Error("denied");
    writeText.mockRejectedValueOnce(error);
    const onError = vi.fn();
    const { result } = renderHook(() => useCopyToClipboard({ onError }));

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.copy("hello");
    });

    expect(ok).toBe(false);
    expect(onError).toHaveBeenCalledWith(error);
    expect(result.current.copied).toBe(false);
  });

  it("fails gracefully when the clipboard API is unavailable", async () => {
    vi.stubGlobal("navigator", {});
    const onError = vi.fn();
    const { result } = renderHook(() => useCopyToClipboard({ onError }));

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.copy("hello");
    });

    expect(ok).toBe(false);
    expect(onError).toHaveBeenCalled();
  });

  it("clears the reset timer on unmount", async () => {
    const clearSpy = vi.spyOn(globalThis, "clearTimeout");
    const { result, unmount } = renderHook(() => useCopyToClipboard());

    await act(async () => {
      await result.current.copy("hello");
    });

    unmount();
    expect(clearSpy).toHaveBeenCalled();
  });

  it("resolves waitFor-style assertions for copied state", async () => {
    vi.useRealTimers();
    const { result } = renderHook(() => useCopyToClipboard({ resetDelay: 50 }));

    await act(async () => {
      await result.current.copy("hello");
    });
    expect(result.current.copied).toBe(true);

    await waitFor(() => expect(result.current.copied).toBe(false));
  });
});
