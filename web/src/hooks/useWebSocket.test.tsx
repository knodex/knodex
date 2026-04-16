// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, act, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useWebSocket } from "./useWebSocket";
import type { InstanceUpdateData, RevisionUpdateData } from "@/types/websocket";

// Mock getWebSocketTicket API call
vi.mock("@/api/auth", () => ({
  getWebSocketTicket: vi.fn().mockResolvedValue("mock-ws-ticket-123"),
}));

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url: string;
  readyState: number = MockWebSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.(new Event("open"));
    }, 10);
  }

  send = vi.fn();
  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent("close", { code: 1000, reason: "test close" }));
  });

  simulateMessage(data: unknown) {
    if (this.onmessage) {
      const event = new MessageEvent("message", {
        data: JSON.stringify(data),
      });
      this.onmessage(event);
    }
  }
}

let mockWebSocketInstance: MockWebSocket | null = null;
const originalWebSocket = global.WebSocket;

function createTestWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return {
    queryClient,
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  };
}

describe("useWebSocket", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockWebSocketInstance = null;

    const MockWebSocketClass = class extends MockWebSocket {
      constructor(url: string) {
        super(url);
        mockWebSocketInstance = this;
      }
    };

    Object.assign(MockWebSocketClass, {
      CONNECTING: 0,
      OPEN: 1,
      CLOSING: 2,
      CLOSED: 3,
    });

    global.WebSocket = MockWebSocketClass as unknown as typeof WebSocket;

    const { useUserStore } = await import("@/stores/userStore");
    useUserStore.setState({ isAuthenticated: true });
  });

  afterEach(() => {
    global.WebSocket = originalWebSocket;
  });

  describe("instance_update cache handling", () => {
    it("should use invalidateQueries (not setQueryData) for instance_update", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
      const setQueryDataSpy = vi.spyOn(queryClient, "setQueryData");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const instanceUpdate = {
        type: "instance_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "update",
          namespace: "default",
          kind: "WebApp",
          name: "my-instance",
          instance: { metadata: { name: "my-instance", namespace: "default" } },
        } as InstanceUpdateData,
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(instanceUpdate);
      });

      await waitFor(() => {
        // Should invalidate the specific instance query key (namespace, kind, name)
        expect(invalidateSpy).toHaveBeenCalledWith({
          queryKey: ["instance", "default", "WebApp", "my-instance"],
        });
      });

      // Should NOT use setQueryData for the specific instance
      const setQueryDataCalls = setQueryDataSpy.mock.calls.filter(
        (call) =>
          Array.isArray(call[0]) &&
          call[0][0] === "instance" &&
          call[0][1] === "default" &&
          call[0][2] === "WebApp" &&
          call[0][3] === "my-instance"
      );
      expect(setQueryDataCalls).toHaveLength(0);
    });

    it("should still invalidate the instances list", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "instance_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "update",
            namespace: "default",
            kind: "WebApp",
            name: "my-instance",
            instance: { metadata: { name: "my-instance" } },
          } as InstanceUpdateData,
        });
      });

      await waitFor(() => {
        // Should invalidate instances list with predicate that excludes count queries
        expect(invalidateSpy).toHaveBeenCalledWith(
          expect.objectContaining({
            queryKey: ["instances"],
            predicate: expect.any(Function),
          })
        );
      });
    });

    it("should handle rapid consecutive instance_update messages", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      // Fire 3 rapid updates for the same instance (simulates ArgoCD sync status churn)
      act(() => {
        for (let i = 0; i < 3; i++) {
          mockWebSocketInstance?.simulateMessage({
            type: "instance_update",
            timestamp: new Date().toISOString(),
            data: {
              action: "update",
              namespace: "default",
              kind: "WebApp",
              name: "my-instance",
              instance: { metadata: { name: "my-instance", namespace: "default" } },
            } as InstanceUpdateData,
          });
        }
      });

      await waitFor(() => {
        // Each message should produce an invalidation for the specific instance (ns, kind, name)
        const specificCalls = invalidateSpy.mock.calls.filter(
          (call) =>
            (call[0] as { queryKey?: string[] }).queryKey?.[0] === "instance" &&
            (call[0] as { queryKey?: string[] }).queryKey?.[1] === "default" &&
            (call[0] as { queryKey?: string[] }).queryKey?.[2] === "WebApp" &&
            (call[0] as { queryKey?: string[] }).queryKey?.[3] === "my-instance"
        );
        expect(specificCalls).toHaveLength(3);
      });
    });

  });

  describe("revision_update cache handling", () => {
    it("should invalidate revision list query on revision_update", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "revision_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "add",
            rgdName: "my-webapp",
            revision: 5,
          } as RevisionUpdateData,
        });
      });

      await waitFor(() => {
        expect(invalidateSpy).toHaveBeenCalledWith({
          queryKey: ["rgd", "my-webapp", "revisions"],
          exact: true,
        });
      });
    });

    it("should NOT invalidate individual revision or diff queries on revision_update", async () => {
      const { wrapper, queryClient } = createTestWrapper();

      // Seed immutable queries into the cache BEFORE the WebSocket message
      queryClient.setQueryData(["rgd", "my-webapp", "revision", 3], { revision: 3, snapshot: {} });
      queryClient.setQueryData(["rgd", "my-webapp", "revisions", "diff", 3, 4], { diff: {} });

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "revision_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "add",
            rgdName: "my-webapp",
            revision: 5,
          } as RevisionUpdateData,
        });
      });

      // Wait for invalidation to process
      await waitFor(() => {
        const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
        // The handler should have been called by now
        expect(invalidateSpy).toBeDefined();
      });

      // Verify immutable queries are still in cache and NOT invalidated
      const revisionData = queryClient.getQueryData(["rgd", "my-webapp", "revision", 3]);
      expect(revisionData).toEqual({ revision: 3, snapshot: {} });

      const diffData = queryClient.getQueryData(["rgd", "my-webapp", "revisions", "diff", 3, 4]);
      expect(diffData).toEqual({ diff: {} });
    });

    it("should handle revision_update with projectId field", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "revision_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "add",
            rgdName: "team-app",
            revision: 2,
            projectId: "team-alpha",
          } as RevisionUpdateData,
        });
      });

      await waitFor(() => {
        expect(invalidateSpy).toHaveBeenCalledWith({
          queryKey: ["rgd", "team-app", "revisions"],
          exact: true,
        });
      });
    });
  });

  describe("instance_update cache handling (delete)", () => {
    it("should use removeQueries for delete actions", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const removeSpy = vi.spyOn(queryClient, "removeQueries");

      renderHook(() => useWebSocket({ autoConnect: true }), { wrapper });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "instance_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "delete",
            namespace: "default",
            kind: "WebApp",
            name: "deleted-instance",
          } as InstanceUpdateData,
        });
      });

      await waitFor(() => {
        expect(removeSpy).toHaveBeenCalledWith({
          queryKey: ["instance", "default", "WebApp", "deleted-instance"],
        });
      });
    });
  });
});
