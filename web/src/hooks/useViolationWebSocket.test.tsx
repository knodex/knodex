import { renderHook, act, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useViolationWebSocket } from "./useViolationWebSocket";
import type { ViolationUpdateData, TemplateUpdateData, ConstraintUpdateData } from "@/types/websocket";

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
  },
}));

// Mock getWebSocketTicket API call
vi.mock("@/api/auth", () => ({
  getWebSocketTicket: vi.fn().mockResolvedValue("mock-ws-ticket-123"),
}));

import { toast } from "sonner";

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
    // Simulate connection opening
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

  // Helper to simulate receiving a message
  simulateMessage(data: unknown) {
    if (this.onmessage) {
      const event = new MessageEvent("message", {
        data: JSON.stringify(data),
      });
      this.onmessage(event);
    }
  }

  // Helper to simulate error
  simulateError() {
    this.onerror?.(new Event("error"));
  }
}

// Store reference to created WebSocket instances
let mockWebSocketInstance: MockWebSocket | null = null;

// Mock global WebSocket
const originalWebSocket = global.WebSocket;
const WebSocketSpy = vi.fn();

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

describe("useViolationWebSocket", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockWebSocketInstance = null;
    WebSocketSpy.mockClear();

    // Create a proper WebSocket mock class that can be instantiated with `new`
    const MockWebSocketClass = class extends MockWebSocket {
      constructor(url: string) {
        super(url);
        mockWebSocketInstance = this;
        WebSocketSpy(url);
      }
    };

    // Assign static properties
    Object.assign(MockWebSocketClass, {
      CONNECTING: 0,
      OPEN: 1,
      CLOSING: 2,
      CLOSED: 3,
    });

    global.WebSocket = MockWebSocketClass as unknown as typeof WebSocket;

    // Mock localStorage for JWT token
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: vi.fn().mockReturnValue("test-jwt-token"),
        setItem: vi.fn(),
        removeItem: vi.fn(),
      },
      writable: true,
    });
  });

  afterEach(() => {
    global.WebSocket = originalWebSocket;
  });

  describe("connection management", () => {
    it("should connect on mount when autoConnect is true", async () => {
      const { wrapper } = createTestWrapper();

      const { result } = renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      expect(result.current.status).toBe("connecting");

      await waitFor(() => {
        expect(result.current.status).toBe("connected");
      });
    });

    it("should not connect on mount when autoConnect is false", async () => {
      const { wrapper } = createTestWrapper();

      const { result } = renderHook(() => useViolationWebSocket({ autoConnect: false }), {
        wrapper,
      });

      expect(result.current.status).toBe("disconnected");
      expect(WebSocketSpy).not.toHaveBeenCalled();
    });

    it("should subscribe to violations after connection", async () => {
      const { wrapper } = createTestWrapper();

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.send).toHaveBeenCalledWith(
          expect.stringContaining('"type":"subscribe"')
        );
      });

      expect(mockWebSocketInstance?.send).toHaveBeenCalledWith(
        JSON.stringify({ type: "subscribe", data: { resourceType: "violations" } })
      );
    });
  });

  describe("cache invalidation on violation_update", () => {
    it("should invalidate violation queries when violation is detected", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const violationUpdate: { type: string; timestamp: string; data: ViolationUpdateData } = {
        type: "violation_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "add",
          constraintKind: "K8sRequiredLabels",
          constraintName: "require-team-label",
          resource: {
            kind: "Pod",
            namespace: "default",
            name: "test-pod",
          },
          message: "Missing required label: team",
          enforcementAction: "deny",
        },
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(violationUpdate);
      });

      await waitFor(() => {
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["violations"] });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "violations"] });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["constraints"] });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "constraints"] });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "summary"] });
      });
    });

    it("should show toast notification for deny violations", async () => {
      const { wrapper } = createTestWrapper();

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const violationUpdate = {
        type: "violation_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "add",
          constraintKind: "K8sRequiredLabels",
          constraintName: "require-team-label",
          resource: {
            kind: "Pod",
            namespace: "default",
            name: "test-pod",
          },
          message: "Missing required label: team",
          enforcementAction: "deny",
        } as ViolationUpdateData,
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(violationUpdate);
      });

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith("Policy Violation Detected", {
          description: "K8sRequiredLabels/require-team-label: Missing required label: team",
          duration: 5000,
        });
      });
    });

    it("should NOT show toast notification for warn violations", async () => {
      const { wrapper } = createTestWrapper();

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const violationUpdate = {
        type: "violation_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "add",
          constraintKind: "K8sRequiredLabels",
          constraintName: "require-team-label",
          resource: {
            kind: "Pod",
            namespace: "default",
            name: "test-pod",
          },
          message: "Missing required label: team",
          enforcementAction: "warn",
        } as ViolationUpdateData,
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(violationUpdate);
      });

      // Give some time for any async operations
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(toast.error).not.toHaveBeenCalled();
    });

    it("should call onViolationDetected callback", async () => {
      const { wrapper } = createTestWrapper();
      const onViolationDetected = vi.fn();

      renderHook(
        () =>
          useViolationWebSocket({
            autoConnect: true,
            onViolationDetected,
          }),
        { wrapper }
      );

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const violationData: ViolationUpdateData = {
        action: "add",
        constraintKind: "K8sRequiredLabels",
        constraintName: "require-team-label",
        resource: {
          kind: "Pod",
          namespace: "default",
          name: "test-pod",
        },
        message: "Missing required label",
        enforcementAction: "deny",
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "violation_update",
          timestamp: new Date().toISOString(),
          data: violationData,
        });
      });

      await waitFor(() => {
        expect(onViolationDetected).toHaveBeenCalledWith(violationData);
      });
    });

    it("should call onViolationResolved callback", async () => {
      const { wrapper } = createTestWrapper();
      const onViolationResolved = vi.fn();

      renderHook(
        () =>
          useViolationWebSocket({
            autoConnect: true,
            onViolationResolved,
          }),
        { wrapper }
      );

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const violationData: ViolationUpdateData = {
        action: "delete",
        constraintKind: "K8sRequiredLabels",
        constraintName: "require-team-label",
        resource: {
          kind: "Pod",
          namespace: "default",
          name: "test-pod",
        },
        message: "Missing required label",
        enforcementAction: "deny",
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "violation_update",
          timestamp: new Date().toISOString(),
          data: violationData,
        });
      });

      await waitFor(() => {
        expect(onViolationResolved).toHaveBeenCalledWith(violationData);
      });
    });
  });

  describe("cache invalidation on template_update", () => {
    it("should invalidate template queries when template is updated", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const templateUpdate: { type: string; timestamp: string; data: TemplateUpdateData } = {
        type: "template_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "add",
          name: "k8srequiredlabels",
          kind: "K8sRequiredLabels",
          description: "Requires specific labels",
        },
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(templateUpdate);
      });

      await waitFor(() => {
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "templates"] });
        expect(invalidateSpy).toHaveBeenCalledWith({
          queryKey: ["compliance", "template", "k8srequiredlabels"],
        });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "summary"] });
      });
    });
  });

  describe("cache invalidation on constraint_update", () => {
    it("should invalidate constraint queries when constraint is updated", async () => {
      const { wrapper, queryClient } = createTestWrapper();
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      const constraintUpdate: { type: string; timestamp: string; data: ConstraintUpdateData } = {
        type: "constraint_update",
        timestamp: new Date().toISOString(),
        data: {
          action: "add",
          kind: "K8sRequiredLabels",
          name: "require-team-label",
          enforcementAction: "deny",
          violationCount: 5,
        },
      };

      act(() => {
        mockWebSocketInstance?.simulateMessage(constraintUpdate);
      });

      await waitFor(() => {
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["constraints"] });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "constraints"] });
        expect(invalidateSpy).toHaveBeenCalledWith({
          queryKey: ["compliance", "constraint", "K8sRequiredLabels", "require-team-label"],
        });
        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance", "summary"] });
      });
    });
  });

  describe("recent update indicator", () => {
    it("should set hasRecentUpdate on violation event", async () => {
      const { wrapper } = createTestWrapper();

      const { result } = renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      expect(result.current.hasRecentUpdate).toBe(false);

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "violation_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "add",
            constraintKind: "K8sRequiredLabels",
            constraintName: "test",
            resource: { kind: "Pod", namespace: "default", name: "test" },
            message: "test",
            enforcementAction: "deny",
          },
        });
      });

      await waitFor(() => {
        expect(result.current.hasRecentUpdate).toBe(true);
      });
    });

    it("should update lastUpdateTime on event", async () => {
      const { wrapper } = createTestWrapper();

      const { result } = renderHook(() => useViolationWebSocket({ autoConnect: true }), {
        wrapper,
      });

      await waitFor(() => {
        expect(mockWebSocketInstance?.readyState).toBe(MockWebSocket.OPEN);
      });

      expect(result.current.lastUpdateTime).toBeNull();

      const beforeTime = new Date();

      act(() => {
        mockWebSocketInstance?.simulateMessage({
          type: "violation_update",
          timestamp: new Date().toISOString(),
          data: {
            action: "add",
            constraintKind: "K8sRequiredLabels",
            constraintName: "test",
            resource: { kind: "Pod", namespace: "default", name: "test" },
            message: "test",
            enforcementAction: "deny",
          },
        });
      });

      await waitFor(() => {
        expect(result.current.lastUpdateTime).not.toBeNull();
      });

      expect(result.current.lastUpdateTime!.getTime()).toBeGreaterThanOrEqual(beforeTime.getTime());
    });
  });
});
