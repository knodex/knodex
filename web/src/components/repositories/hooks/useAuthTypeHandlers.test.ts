// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAuthTypeHandlers } from "./useAuthTypeHandlers";

describe("useAuthTypeHandlers", () => {
  const mockSetValue = vi.fn();

  it("provides auth type setters for all three types", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));
    expect(result.current.authTypeSetters.ssh).toBeInstanceOf(Function);
    expect(result.current.authTypeSetters.https).toBeInstanceOf(Function);
    expect(result.current.authTypeSetters["github-app"]).toBeInstanceOf(Function);
  });

  it("sets auth type to ssh", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));

    act(() => {
      result.current.authTypeSetters.ssh();
    });

    expect(mockSetValue).toHaveBeenCalledWith("authType", "ssh");
  });

  it("sets auth type to https", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));

    act(() => {
      result.current.authTypeSetters.https();
    });

    expect(mockSetValue).toHaveBeenCalledWith("authType", "https");
  });

  it("sets auth type to github-app", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));

    act(() => {
      result.current.authTypeSetters["github-app"]();
    });

    expect(mockSetValue).toHaveBeenCalledWith("authType", "github-app");
  });

  it("provides github app type setters", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));
    expect(result.current.githubAppTypeSetters.github).toBeInstanceOf(Function);
    expect(result.current.githubAppTypeSetters["github-enterprise"]).toBeInstanceOf(Function);
  });

  it("sets github app type to github", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));

    act(() => {
      result.current.githubAppTypeSetters.github();
    });

    expect(mockSetValue).toHaveBeenCalledWith("githubAppAuth.appType", "github");
  });

  it("sets github app type to github-enterprise", () => {
    const { result } = renderHook(() => useAuthTypeHandlers(mockSetValue));

    act(() => {
      result.current.githubAppTypeSetters["github-enterprise"]();
    });

    expect(mockSetValue).toHaveBeenCalledWith("githubAppAuth.appType", "github-enterprise");
  });
});
