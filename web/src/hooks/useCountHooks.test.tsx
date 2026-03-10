// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useRGDCount } from "./useRGDs";
import { useInstanceCount } from "./useInstances";
import * as rgdApi from "@/api/rgd";
import type { ReactNode } from "react";

// Mock the RGD API
vi.mock("@/api/rgd");

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

describe("Count hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useRGDCount", () => {
    it("should fetch RGD count", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 5 });

      const { result } = renderHook(() => useRGDCount(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual({ count: 5 });
      expect(rgdApi.getRGDCount).toHaveBeenCalledTimes(1);
    });

    it("should return count of 0", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 0 });

      const { result } = renderHook(() => useRGDCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.count).toBe(0);
    });

    it("should handle errors", async () => {
      const error = new Error("Failed to fetch RGD count");
      vi.mocked(rgdApi.getRGDCount).mockRejectedValue(error);

      const { result } = renderHook(() => useRGDCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });

    it("should have correct query key for cache management", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 10 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useRGDCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify the query is cached with the correct key
      const cachedData = queryClient.getQueryData(["rgds", "count"]);
      expect(cachedData).toEqual({ count: 10 });
    });
  });

  describe("useInstanceCount", () => {
    it("should fetch instance count", async () => {
      vi.mocked(rgdApi.getInstanceCount).mockResolvedValue({ count: 15 });

      const { result } = renderHook(() => useInstanceCount(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual({ count: 15 });
      expect(rgdApi.getInstanceCount).toHaveBeenCalledTimes(1);
    });

    it("should return count of 0", async () => {
      vi.mocked(rgdApi.getInstanceCount).mockResolvedValue({ count: 0 });

      const { result } = renderHook(() => useInstanceCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.count).toBe(0);
    });

    it("should handle errors", async () => {
      const error = new Error("Failed to fetch instance count");
      vi.mocked(rgdApi.getInstanceCount).mockRejectedValue(error);

      const { result } = renderHook(() => useInstanceCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });

    it("should have correct query key for cache management", async () => {
      vi.mocked(rgdApi.getInstanceCount).mockResolvedValue({ count: 25 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useInstanceCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify the query is cached with the correct key
      const cachedData = queryClient.getQueryData(["instances", "count"]);
      expect(cachedData).toEqual({ count: 25 });
    });
  });

  describe("WebSocket count push behavior", () => {
    it("does not refetch RGD count on window focus when data exists", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 5 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useRGDCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Simulate window focus event
      window.dispatchEvent(new Event("focus"));

      // Wait a bit to ensure no additional fetch happens
      await new Promise((resolve) => setTimeout(resolve, 100));

      // Should only have been called once (initial fetch)
      expect(rgdApi.getRGDCount).toHaveBeenCalledTimes(1);
    });

    it("does not refetch instance count on window focus when data exists", async () => {
      vi.mocked(rgdApi.getInstanceCount).mockResolvedValue({ count: 10 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useInstanceCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Simulate window focus event
      window.dispatchEvent(new Event("focus"));

      // Wait a bit to ensure no additional fetch happens
      await new Promise((resolve) => setTimeout(resolve, 100));

      // Should only have been called once (initial fetch)
      expect(rgdApi.getInstanceCount).toHaveBeenCalledTimes(1);
    });

    it("fetches on mount when no cached data exists (initial load)", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 42 });

      const { result } = renderHook(() => useRGDCount(), {
        wrapper: createWrapper(),
      });

      // Should start loading (cache is empty)
      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual({ count: 42 });
      expect(rgdApi.getRGDCount).toHaveBeenCalledTimes(1);
    });

    it("accepts direct cache writes via setQueryData (WebSocket pattern)", async () => {
      vi.mocked(rgdApi.getRGDCount).mockResolvedValue({ count: 5 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useRGDCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(5);
      });

      // Simulate WebSocket counts_update writing directly to cache
      queryClient.setQueryData(["rgds", "count"], { count: 99 });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(99);
      });

      // Should NOT have triggered a refetch
      expect(rgdApi.getRGDCount).toHaveBeenCalledTimes(1);
    });
  });

  describe("Count hooks query invalidation", () => {
    it("should allow invalidating RGD count independently", async () => {
      vi.mocked(rgdApi.getRGDCount)
        .mockResolvedValueOnce({ count: 5 })
        .mockResolvedValueOnce({ count: 6 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useRGDCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(5);
      });

      // Invalidate the count query
      await queryClient.invalidateQueries({ queryKey: ["rgds", "count"] });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(6);
      });

      expect(rgdApi.getRGDCount).toHaveBeenCalledTimes(2);
    });

    it("should allow invalidating instance count independently", async () => {
      vi.mocked(rgdApi.getInstanceCount)
        .mockResolvedValueOnce({ count: 10 })
        .mockResolvedValueOnce({ count: 12 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useInstanceCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(10);
      });

      // Invalidate the count query
      await queryClient.invalidateQueries({ queryKey: ["instances", "count"] });

      await waitFor(() => {
        expect(result.current.data?.count).toBe(12);
      });

      expect(rgdApi.getInstanceCount).toHaveBeenCalledTimes(2);
    });
  });
});
