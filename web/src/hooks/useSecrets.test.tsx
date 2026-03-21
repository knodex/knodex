// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { useSecretList, useSecret, useSecretExists } from "./useSecrets";
import * as secretsApi from "@/api/secrets";
import type { ReactNode } from "react";

// Mock the secrets API
vi.mock("@/api/secrets");

// Capture original __ENTERPRISE__ value for restore in afterEach
const originalEnterprise = (globalThis as Record<string, unknown>).__ENTERPRISE__;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useSecrets hooks — isEnterprise() guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to enterprise mode
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  describe("useSecretList", () => {
    it("fetches secrets list when enterprise and project provided", async () => {
      const mockResponse = { items: [], totalCount: 0, hasMore: false };
      vi.mocked(secretsApi.listSecrets).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useSecretList("my-project"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(secretsApi.listSecrets).toHaveBeenCalledWith("my-project", undefined);
    });

    it("does NOT fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useSecretList("my-project"), {
        wrapper: createWrapper(),
      });

      // Wait to confirm the query is disabled
      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(secretsApi.listSecrets).not.toHaveBeenCalled();
    });

    it("does NOT fetch when project is empty", async () => {
      const { result } = renderHook(() => useSecretList(""), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(secretsApi.listSecrets).not.toHaveBeenCalled();
    });
  });

  describe("useSecret", () => {
    it("fetches secret when enterprise and all params provided", async () => {
      const mockSecret = {
        name: "my-secret",
        namespace: "default",
        data: { key: "value" },
        createdAt: "2026-01-01T00:00:00Z",
      };
      vi.mocked(secretsApi.getSecret).mockResolvedValue(mockSecret);

      const { result } = renderHook(
        () => useSecret("my-secret", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(secretsApi.getSecret).toHaveBeenCalledWith("my-secret", "my-project", "default");
    });

    it("does NOT fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(
        () => useSecret("my-secret", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(secretsApi.getSecret).not.toHaveBeenCalled();
    });

    it("does NOT fetch when any required param is empty", async () => {
      const { result } = renderHook(
        () => useSecret("", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(secretsApi.getSecret).not.toHaveBeenCalled();
    });
  });

  describe("useSecretExists", () => {
    it("checks existence when enterprise and all params provided", async () => {
      vi.mocked(secretsApi.checkSecretExists).mockResolvedValue(undefined);

      const { result } = renderHook(
        () => useSecretExists("my-secret", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.exists).toBe(true);
      expect(secretsApi.checkSecretExists).toHaveBeenCalledWith("my-secret", "my-project", "default");
    });

    it("returns exists=false on 404", async () => {
      // Create a real AxiosError so `instanceof AxiosError` passes in the hook
      const notFoundError = new AxiosError(
        "Not found",
        "ERR_BAD_REQUEST",
        undefined,
        undefined,
        { status: 404, data: {}, statusText: "Not Found", headers: {}, config: { headers: new AxiosHeaders() } },
      );
      vi.mocked(secretsApi.checkSecretExists).mockRejectedValue(notFoundError);

      const { result } = renderHook(
        () => useSecretExists("missing-secret", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      }, { timeout: 5000 });

      // Verify 404 was handled as exists=false (not as a query error)
      expect(result.current.exists).toBe(false);
      expect(result.current.isError).toBe(false);
    });

    it("does NOT fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(
        () => useSecretExists("my-secret", "my-project", "default"),
        { wrapper: createWrapper() }
      );

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.isLoading).toBe(false);
      expect(secretsApi.checkSecretExists).not.toHaveBeenCalled();
    });

    it("does NOT fetch when any required param is empty", async () => {
      const { result } = renderHook(
        () => useSecretExists("my-secret", "", "default"),
        { wrapper: createWrapper() }
      );

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.isLoading).toBe(false);
      expect(secretsApi.checkSecretExists).not.toHaveBeenCalled();
    });
  });
});
