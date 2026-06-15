// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  useRoleTemplates,
  useCreateRoleTemplate,
  useUpdateRoleTemplate,
  useDeleteRoleTemplate,
} from "./useRoleTemplates";
import * as api from "@/api/role-templates";
import type { RoleTemplate } from "@/api/role-templates";

vi.mock("@/api/role-templates");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

const sample: RoleTemplate = {
  name: "developer",
  label: "Developer",
  description: "Deploy and manage instances",
  policies: ["p, proj:{project}:{role}, rgds, get, *, allow"],
};

describe("useRoleTemplates", () => {
  beforeEach(() => vi.clearAllMocks());

  it("fetches and returns the catalog on success", async () => {
    vi.mocked(api.listRoleTemplates).mockResolvedValue([sample]);

    const { result } = renderHook(() => useRoleTemplates(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([sample]);
  });

  it("surfaces an error (e.g. 403 for a non-operator)", async () => {
    vi.mocked(api.listRoleTemplates).mockRejectedValue(new Error("forbidden"));

    const { result } = renderHook(() => useRoleTemplates(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});

describe("role-template mutations", () => {
  beforeEach(() => vi.clearAllMocks());

  it("useCreateRoleTemplate calls the create API", async () => {
    vi.mocked(api.createRoleTemplate).mockResolvedValue(sample);

    const { result } = renderHook(() => useCreateRoleTemplate(), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync(sample);

    expect(api.createRoleTemplate).toHaveBeenCalledWith(sample);
  });

  it("useUpdateRoleTemplate calls the update API with name + body", async () => {
    vi.mocked(api.updateRoleTemplate).mockResolvedValue(sample);

    const { result } = renderHook(() => useUpdateRoleTemplate(), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync({ name: "developer", template: sample });

    expect(api.updateRoleTemplate).toHaveBeenCalledWith("developer", sample);
  });

  it("useDeleteRoleTemplate calls the delete API", async () => {
    vi.mocked(api.deleteRoleTemplate).mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteRoleTemplate(), {
      wrapper: createWrapper(),
    });
    await result.current.mutateAsync("developer");

    expect(api.deleteRoleTemplate).toHaveBeenCalledWith("developer");
  });
});
