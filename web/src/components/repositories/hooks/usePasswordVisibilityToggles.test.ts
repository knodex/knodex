// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { usePasswordVisibilityToggles } from "./usePasswordVisibilityToggles";

describe("usePasswordVisibilityToggles", () => {
  it("initializes all fields as hidden", () => {
    const { result } = renderHook(() => usePasswordVisibilityToggles());
    expect(result.current.showPrivateKey).toBe(false);
    expect(result.current.showPassword).toBe(false);
    expect(result.current.showBearerToken).toBe(false);
  });

  it("toggles private key visibility", () => {
    const { result } = renderHook(() => usePasswordVisibilityToggles());

    act(() => {
      result.current.togglePrivateKey();
    });
    expect(result.current.showPrivateKey).toBe(true);

    act(() => {
      result.current.togglePrivateKey();
    });
    expect(result.current.showPrivateKey).toBe(false);
  });

  it("toggles password visibility", () => {
    const { result } = renderHook(() => usePasswordVisibilityToggles());

    act(() => {
      result.current.togglePassword();
    });
    expect(result.current.showPassword).toBe(true);

    act(() => {
      result.current.togglePassword();
    });
    expect(result.current.showPassword).toBe(false);
  });

  it("toggles bearer token visibility", () => {
    const { result } = renderHook(() => usePasswordVisibilityToggles());

    act(() => {
      result.current.toggleBearerToken();
    });
    expect(result.current.showBearerToken).toBe(true);

    act(() => {
      result.current.toggleBearerToken();
    });
    expect(result.current.showBearerToken).toBe(false);
  });

  it("toggles fields independently", () => {
    const { result } = renderHook(() => usePasswordVisibilityToggles());

    act(() => {
      result.current.togglePrivateKey();
    });

    expect(result.current.showPrivateKey).toBe(true);
    expect(result.current.showPassword).toBe(false);
    expect(result.current.showBearerToken).toBe(false);
  });
});
