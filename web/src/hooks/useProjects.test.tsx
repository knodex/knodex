// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useProjects,
  useProject,
  useCreateProject,
  useUpdateProject,
  useProjectResources,
  useDeleteProject,
} from "./useProjects";
import * as projectsApi from "@/api/projects";
import type { ReactNode } from "react";

vi.mock("@/api/projects");

function createClientAndWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const removeSpy = vi.spyOn(queryClient, "removeQueries");
  const setDataSpy = vi.spyOn(queryClient, "setQueryData");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, removeSpy, setDataSpy, Wrapper };
}

describe("useProjects hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useProjects", () => {
    it("lists projects", async () => {
      vi.mocked(projectsApi.listProjects).mockResolvedValue({
        projects: [],
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useProjects(), { wrapper: Wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(projectsApi.listProjects).toHaveBeenCalledTimes(1);
    });
  });

  describe("useProject", () => {
    it("does not fetch when name is empty", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useProject(""), { wrapper: Wrapper });

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(projectsApi.getProject).not.toHaveBeenCalled();
    });

    it("fetches a single project by name", async () => {
      vi.mocked(projectsApi.getProject).mockResolvedValue({
        name: "alpha",
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useProject("alpha"), {
        wrapper: Wrapper,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(projectsApi.getProject).toHaveBeenCalledWith("alpha");
    });
  });

  describe("useCreateProject", () => {
    it("invalidates the projects list on success", async () => {
      vi.mocked(projectsApi.createProject).mockResolvedValue({
        name: "alpha",
      } as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useCreateProject(), {
        wrapper: Wrapper,
      });

      result.current.mutate({ name: "alpha" } as never);
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["projects"]);
    });
  });

  describe("useUpdateProject", () => {
    it("updates the cache and invalidates the list", async () => {
      vi.mocked(projectsApi.updateProject).mockResolvedValue({
        name: "alpha",
      } as never);

      const { invalidateSpy, setDataSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useUpdateProject(), {
        wrapper: Wrapper,
      });

      result.current.mutate({ name: "alpha", request: {} as never });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(projectsApi.updateProject).toHaveBeenCalledWith("alpha", {});
      expect(setDataSpy).toHaveBeenCalledWith(["project", "alpha"], {
        name: "alpha",
      });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["projects"]);
    });
  });

  describe("useProjectResources", () => {
    it("does not fetch when projectName is empty", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useProjectResources("", "Ingress"),
        { wrapper: Wrapper },
      );

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(projectsApi.getProjectResources).not.toHaveBeenCalled();
    });

    it("does not fetch when explicitly disabled", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useProjectResources("alpha", "Certificate", false),
        { wrapper: Wrapper },
      );

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(projectsApi.getProjectResources).not.toHaveBeenCalled();
    });

    it("fetches resources for the given kind when enabled", async () => {
      vi.mocked(projectsApi.getProjectResources).mockResolvedValue({
        resources: [],
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useProjectResources("alpha", "Certificate"),
        { wrapper: Wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(projectsApi.getProjectResources).toHaveBeenCalledWith(
        "alpha",
        "Certificate",
      );
    });
  });

  describe("useDeleteProject", () => {
    it("removes the cached project and invalidates the list", async () => {
      vi.mocked(projectsApi.deleteProject).mockResolvedValue(undefined);

      const { invalidateSpy, removeSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useDeleteProject(), {
        wrapper: Wrapper,
      });

      result.current.mutate("alpha");
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(projectsApi.deleteProject).toHaveBeenCalledWith("alpha");
      expect(removeSpy).toHaveBeenCalledWith({ queryKey: ["project", "alpha"] });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["projects"]);
    });
  });
});
