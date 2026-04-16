// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useInstanceHistory, useInstanceTimeline, useExportHistory } from "./useHistory";
import * as historyApi from "@/api/history";
import type { ReactNode } from "react";

// Mock the history API
vi.mock("@/api/history");

const mockHistory = {
  instanceId: "test-uid",
  instanceName: "test-instance",
  namespace: "test-ns",
  rgdName: "test-rgd",
  events: [
    {
      id: "event-1",
      timestamp: "2024-01-01T00:00:00Z",
      eventType: "Created" as const,
      status: "Pending",
      user: "test-user",
      deploymentMode: "direct" as const,
      message: "Instance created",
    },
    {
      id: "event-2",
      timestamp: "2024-01-01T00:01:00Z",
      eventType: "Ready" as const,
      status: "Ready",
      message: "Instance is ready",
    },
  ],
  createdAt: "2024-01-01T00:00:00Z",
  currentStatus: "Ready",
  deploymentMode: "direct" as const,
};

const mockTimeline = {
  namespace: "test-ns",
  name: "test-instance",
  timeline: [
    {
      timestamp: "2024-01-01T00:00:00Z",
      eventType: "Created" as const,
      status: "Pending",
      user: "test-user",
      isCompleted: true,
      isCurrent: false,
    },
    {
      timestamp: "2024-01-01T00:01:00Z",
      eventType: "Ready" as const,
      status: "Ready",
      isCompleted: true,
      isCurrent: true,
    },
  ],
};

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

describe("useHistory hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useInstanceHistory", () => {
    it("should fetch history for an instance", async () => {
      vi.mocked(historyApi.getInstanceHistory).mockResolvedValue(mockHistory);

      const { result } = renderHook(
        () => useInstanceHistory("test-ns", "WebApp", "test-instance"),
        { wrapper: createWrapper() }
      );

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockHistory);
      expect(historyApi.getInstanceHistory).toHaveBeenCalledWith(
        "test-ns",
        "WebApp",
        "test-instance"
      );
    });

    it("should fetch for cluster-scoped instances (empty namespace)", async () => {
      vi.mocked(historyApi.getInstanceHistory).mockResolvedValue(mockHistory);

      const { result } = renderHook(
        () => useInstanceHistory("", "WebApp", "test-instance"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(historyApi.getInstanceHistory).toHaveBeenCalledWith(
        "",
        "WebApp",
        "test-instance"
      );
    });

    it("should not fetch when name is empty", async () => {
      const { result } = renderHook(
        () => useInstanceHistory("test-ns", "WebApp", ""),
        { wrapper: createWrapper() }
      );

      expect(result.current.isLoading).toBe(false);
      expect(result.current.fetchStatus).toBe("idle");
      expect(historyApi.getInstanceHistory).not.toHaveBeenCalled();
    });

    it("should handle errors", async () => {
      const error = new Error("Failed to fetch history");
      vi.mocked(historyApi.getInstanceHistory).mockRejectedValue(error);

      const { result } = renderHook(
        () => useInstanceHistory("test-ns", "WebApp", "test-instance"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });
  });

  describe("useInstanceTimeline", () => {
    it("should fetch timeline for an instance", async () => {
      vi.mocked(historyApi.getInstanceTimeline).mockResolvedValue(mockTimeline);

      const { result } = renderHook(
        () => useInstanceTimeline("test-ns", "WebApp", "test-instance"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockTimeline);
      expect(historyApi.getInstanceTimeline).toHaveBeenCalledWith(
        "test-ns",
        "WebApp",
        "test-instance"
      );
    });

    it("should fetch for cluster-scoped instances (empty namespace)", async () => {
      vi.mocked(historyApi.getInstanceTimeline).mockResolvedValue(mockTimeline);

      const { result } = renderHook(
        () => useInstanceTimeline("", "WebApp", "test-instance"),
        { wrapper: createWrapper() }
      );

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(historyApi.getInstanceTimeline).toHaveBeenCalledWith(
        "",
        "WebApp",
        "test-instance"
      );
    });
  });

  describe("useExportHistory", () => {
    it("should export history and trigger download", async () => {
      const mockBlob = new Blob(["test data"], { type: "application/json" });
      vi.mocked(historyApi.exportInstanceHistory).mockResolvedValue(mockBlob);
      vi.mocked(historyApi.downloadHistoryExport).mockImplementation(() => {});

      const { result } = renderHook(() => useExportHistory(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        namespace: "test-ns",
        kind: "WebApp",
        name: "test-instance",
        format: "json",
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(historyApi.exportInstanceHistory).toHaveBeenCalledWith(
        "test-ns",
        "WebApp",
        "test-instance",
        "json"
      );
      expect(historyApi.downloadHistoryExport).toHaveBeenCalledWith(
        mockBlob,
        "test-instance",
        "json"
      );
    });

    it("should handle export errors", async () => {
      const error = new Error("Export failed");
      vi.mocked(historyApi.exportInstanceHistory).mockRejectedValue(error);

      const { result } = renderHook(() => useExportHistory(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        namespace: "test-ns",
        kind: "WebApp",
        name: "test-instance",
        format: "csv",
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });
  });
});
