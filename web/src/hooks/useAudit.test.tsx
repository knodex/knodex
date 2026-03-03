import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAuditEvents,
  useAuditEvent,
  useAuditStats,
  useAuditConfig,
  useUpdateAuditConfig,
} from "./useAudit";
import * as auditApi from "@/api/audit";
import type { ReactNode } from "react";
import type { AuditEventList, AuditEvent, AuditStats, AuditConfig } from "@/types/audit";

// Mock the audit API
vi.mock("@/api/audit");

// Mock __ENTERPRISE__ global
const originalEnterprise = globalThis.__ENTERPRISE__;

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

const mockEvent: AuditEvent = {
  id: "01HTEST123",
  timestamp: "2026-02-16T10:00:00Z",
  userId: "user-1",
  userEmail: "admin@test.local",
  sourceIP: "10.0.0.1",
  action: "login",
  resource: "auth",
  name: "session",
  requestId: "req-abc",
  result: "success",
};

const mockEventList: AuditEventList = {
  events: [mockEvent],
  total: 1,
  page: 1,
  pageSize: 50,
};

const mockStats: AuditStats = {
  totalEvents: 100,
  eventsToday: 15,
  topUsers: [{ userId: "admin@test.local", count: 10 }],
  deniedAttempts: 3,
  byActionToday: { login: 5, create: 3 },
  byResultToday: { success: 12, denied: 3 },
};

const mockConfig: AuditConfig = {
  enabled: true,
  retentionDays: 90,
  maxStreamLength: 100000,
  excludeActions: [],
  excludeResources: [],
};

describe("Audit hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  describe("useAuditEvents", () => {
    it("should fetch audit events", async () => {
      vi.mocked(auditApi.getAuditEvents).mockResolvedValue(mockEventList);

      const { result } = renderHook(() => useAuditEvents(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockEventList);
      expect(auditApi.getAuditEvents).toHaveBeenCalledWith(undefined);
    });

    it("should pass filter params to API", async () => {
      vi.mocked(auditApi.getAuditEvents).mockResolvedValue(mockEventList);

      const params = { userId: "admin@test.local", page: 2, pageSize: 25 };
      const { result } = renderHook(() => useAuditEvents(params), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(auditApi.getAuditEvents).toHaveBeenCalledWith(params);
    });

    it("should not fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useAuditEvents(), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(auditApi.getAuditEvents).not.toHaveBeenCalled();
    });

    it("should handle 403 errors without retrying", async () => {
      const error = Object.assign(new Error("Forbidden"), {
        response: { status: 403 },
      });
      vi.mocked(auditApi.getAuditEvents).mockRejectedValue(error);

      const { result } = renderHook(() => useAuditEvents(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      // Should only be called once (no retry on 403)
      expect(auditApi.getAuditEvents).toHaveBeenCalledTimes(1);
    });
  });

  describe("useAuditEvent", () => {
    it("should fetch a single event by ID", async () => {
      vi.mocked(auditApi.getAuditEvent).mockResolvedValue(mockEvent);

      const { result } = renderHook(() => useAuditEvent("01HTEST123"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockEvent);
      expect(auditApi.getAuditEvent).toHaveBeenCalledWith("01HTEST123");
    });

    it("should not fetch when id is null", async () => {
      const { result } = renderHook(() => useAuditEvent(null), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(auditApi.getAuditEvent).not.toHaveBeenCalled();
    });
  });

  describe("useAuditStats", () => {
    it("should fetch audit stats", async () => {
      vi.mocked(auditApi.getAuditStats).mockResolvedValue(mockStats);

      const { result } = renderHook(() => useAuditStats(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockStats);
    });

    it("should not fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useAuditStats(), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(auditApi.getAuditStats).not.toHaveBeenCalled();
    });
  });

  describe("useAuditConfig", () => {
    it("should fetch audit config", async () => {
      vi.mocked(auditApi.getAuditConfig).mockResolvedValue(mockConfig);

      const { result } = renderHook(() => useAuditConfig(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockConfig);
    });
  });

  describe("useUpdateAuditConfig", () => {
    it("should update audit config and invalidate queries", async () => {
      const updatedConfig = { ...mockConfig, retentionDays: 30 };
      vi.mocked(auditApi.updateAuditConfig).mockResolvedValue(updatedConfig);

      const { result } = renderHook(() => useUpdateAuditConfig(), {
        wrapper: createWrapper(),
      });

      await result.current.mutateAsync(updatedConfig);

      expect(auditApi.updateAuditConfig).toHaveBeenCalledWith(updatedConfig);
    });
  });
});
