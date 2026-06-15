// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useCreateSecret,
  useUpdateSecret,
  useDeleteSecret,
  useSecretList,
} from "./useSecrets";
import * as secretsApi from "@/api/secrets";
import type { ReactNode } from "react";

vi.mock("@/api/secrets");

function createClientAndWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, Wrapper };
}

describe("useSecrets mutation hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useSecretList pagination key", () => {
    it("forwards namespace, limit and continue to listSecrets", async () => {
      vi.mocked(secretsApi.listSecrets).mockResolvedValue({
        items: [],
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () =>
          useSecretList({ namespace: "xxx-shared", limit: 25, continue: "tok" }),
        { wrapper: Wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(secretsApi.listSecrets).toHaveBeenCalledWith({
        namespace: "xxx-shared",
        limit: 25,
        continue: "tok",
      });
    });
  });

  describe("useCreateSecret", () => {
    it("creates a secret and invalidates list + namespace caches", async () => {
      vi.mocked(secretsApi.createSecret).mockResolvedValue(undefined as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useCreateSecret(), { wrapper: Wrapper });

      result.current.mutate({
        namespace: "xxx-shared",
        name: "my-secret",
        data: { token: "abc" },
      } as never);
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(secretsApi.createSecret).toHaveBeenCalledWith("xxx-shared", {
        name: "my-secret",
        data: { token: "abc" },
      });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["secrets"]);
      expect(keys).toContainEqual(["secret", "xxx-shared"]);
    });
  });

  describe("useUpdateSecret", () => {
    it("updates a secret and invalidates the specific entry", async () => {
      vi.mocked(secretsApi.updateSecret).mockResolvedValue(undefined as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useUpdateSecret(), { wrapper: Wrapper });

      result.current.mutate({
        name: "my-secret",
        namespace: "xxx-shared",
        data: { token: "xyz" },
      } as never);
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(secretsApi.updateSecret).toHaveBeenCalledWith(
        "my-secret",
        "xxx-shared",
        { data: { token: "xyz" } },
      );
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["secrets"]);
      expect(keys).toContainEqual(["secret", "xxx-shared", "my-secret"]);
    });
  });

  describe("useDeleteSecret", () => {
    it("deletes a secret and invalidates the caches", async () => {
      vi.mocked(secretsApi.deleteSecret).mockResolvedValue(undefined as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useDeleteSecret(), { wrapper: Wrapper });

      result.current.mutate({ name: "my-secret", namespace: "xxx-shared" });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(secretsApi.deleteSecret).toHaveBeenCalledWith(
        "my-secret",
        "xxx-shared",
      );
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["secrets"]);
      expect(keys).toContainEqual(["secret", "xxx-shared", "my-secret"]);
    });
  });
});
