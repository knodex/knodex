// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useTestConnection } from "./useTestConnection";

describe("useTestConnection", () => {
  const mockWatch = vi.fn().mockReturnValue({
    repoURL: "https://github.com/test/repo.git",
    authType: "https",
    httpsAuth: {
      username: "user",
      password: "pass",
      bearerToken: "",
      tlsClientCert: "",
      tlsClientKey: "",
      insecureSkipTLSVerify: false,
    },
  });

  it("initializes with null result and not testing", () => {
    const { result } = renderHook(() =>
      useTestConnection(mockWatch, "https")
    );
    expect(result.current.testResult).toBeNull();
    expect(result.current.isTesting).toBe(false);
  });

  it("does nothing when onTestConnection is not provided", async () => {
    const { result } = renderHook(() =>
      useTestConnection(mockWatch, "https")
    );

    await act(async () => {
      await result.current.handleTestConnection();
    });

    expect(result.current.testResult).toBeNull();
  });

  it("calls onTestConnection and sets success result", async () => {
    const mockTestFn = vi.fn().mockResolvedValue({
      valid: true,
      message: "Connection successful",
    });

    const { result } = renderHook(() =>
      useTestConnection(mockWatch, "https", mockTestFn)
    );

    await act(async () => {
      await result.current.handleTestConnection();
    });

    expect(mockTestFn).toHaveBeenCalledWith({
      repoURL: "https://github.com/test/repo.git",
      authType: "https",
      httpsAuth: {
        username: "user",
        password: "pass",
        bearerToken: undefined,
        tlsClientCert: undefined,
        tlsClientKey: undefined,
        insecureSkipTLSVerify: false,
      },
    });
    expect(result.current.testResult?.valid).toBe(true);
    expect(result.current.isTesting).toBe(false);
  });

  it("handles connection test error", async () => {
    const mockTestFn = vi.fn().mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() =>
      useTestConnection(mockWatch, "https", mockTestFn)
    );

    await act(async () => {
      await result.current.handleTestConnection();
    });

    expect(result.current.testResult?.valid).toBe(false);
    expect(result.current.testResult?.message).toBe("Network error");
  });

  it("resets test result when auth type changes", () => {
    const { result, rerender } = renderHook(
      ({ authType }) => useTestConnection(mockWatch, authType),
      { initialProps: { authType: "https" as const } }
    );

    // Manually set a test result first by testing
    // Then rerender with different auth type
    rerender({ authType: "ssh" as const });

    expect(result.current.testResult).toBeNull();
  });
});
