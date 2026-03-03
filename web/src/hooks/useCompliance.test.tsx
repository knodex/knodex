import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useComplianceSummary,
  useConstraintTemplates,
  useConstraints,
  useViolations,
  useRecentViolations,
  useViolationCount,
  isEnterprise,
} from "./useCompliance";
import * as complianceApi from "@/api/compliance";
import type { ReactNode } from "react";

// Mock the compliance API
vi.mock("@/api/compliance");

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

describe("Compliance hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to enterprise mode for most tests
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  describe("isEnterprise", () => {
    it("returns true when __ENTERPRISE__ is true", () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
      expect(isEnterprise()).toBe(true);
    });

    it("returns false when __ENTERPRISE__ is false", () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;
      expect(isEnterprise()).toBe(false);
    });

    it("returns false when __ENTERPRISE__ is undefined", () => {
      delete (globalThis as Record<string, unknown>).__ENTERPRISE__;
      expect(isEnterprise()).toBe(false);
    });
  });

  describe("useComplianceSummary", () => {
    it("should fetch compliance summary", async () => {
      const mockSummary = {
        totalTemplates: 10,
        totalConstraints: 25,
        totalViolations: 5,
        byEnforcement: { deny: 2, warn: 3, dryrun: 0 },
      };
      vi.mocked(complianceApi.getComplianceSummary).mockResolvedValue(mockSummary);

      const { result } = renderHook(() => useComplianceSummary(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockSummary);
      expect(complianceApi.getComplianceSummary).toHaveBeenCalled();
    });

    it("should handle errors", async () => {
      // Simulate a 402 error so the retry logic doesn't retry
      const error = Object.assign(new Error("Enterprise required"), { status: 402 });
      vi.mocked(complianceApi.getComplianceSummary).mockRejectedValue(error);
      vi.mocked(complianceApi.isEnterpriseRequired).mockReturnValue(true);

      const { result } = renderHook(() => useComplianceSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      }, { timeout: 5000 });

      expect(result.current.error).toBe(error);
    });

    it("should not fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useComplianceSummary(), {
        wrapper: createWrapper(),
      });

      // Wait a bit to ensure the query doesn't run
      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(complianceApi.getComplianceSummary).not.toHaveBeenCalled();
    });
  });

  describe("useConstraintTemplates", () => {
    it("should fetch constraint templates with default params", async () => {
      const mockResponse = {
        items: [
          { name: "k8srequiredlabels", description: "Requires labels", constraintCount: 3 },
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      };
      vi.mocked(complianceApi.listConstraintTemplates).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useConstraintTemplates(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockResponse);
      expect(complianceApi.listConstraintTemplates).toHaveBeenCalledWith(undefined);
    });

    it("should pass params to API", async () => {
      vi.mocked(complianceApi.listConstraintTemplates).mockResolvedValue({
        items: [],
        total: 0,
        page: 2,
        pageSize: 10,
      });

      const params = { page: 2, pageSize: 10 };
      const { result } = renderHook(() => useConstraintTemplates(params), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(complianceApi.listConstraintTemplates).toHaveBeenCalledWith(params);
    });
  });

  describe("useConstraints", () => {
    it("should fetch constraints", async () => {
      const mockResponse = {
        items: [
          {
            name: "require-team-label",
            kind: "K8sRequiredLabels",
            templateName: "k8srequiredlabels",
            enforcementAction: "deny",
            violationCount: 2,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      };
      vi.mocked(complianceApi.listConstraints).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useConstraints(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockResponse);
    });

    it("should filter by templateName", async () => {
      vi.mocked(complianceApi.listConstraints).mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        pageSize: 20,
      });

      const params = { templateName: "k8srequiredlabels" };
      const { result } = renderHook(() => useConstraints(params), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(complianceApi.listConstraints).toHaveBeenCalledWith(params);
    });
  });

  describe("useViolations", () => {
    it("should fetch violations", async () => {
      const mockResponse = {
        items: [
          {
            constraintKind: "K8sRequiredLabels",
            constraintName: "require-team-label",
            enforcementAction: "deny",
            message: "Missing required label: team",
            resource: {
              kind: "Pod",
              name: "test-pod",
              namespace: "default",
              apiVersion: "v1",
            },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      };
      vi.mocked(complianceApi.listViolations).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useViolations(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockResponse);
    });

    it("should filter by constraintName", async () => {
      vi.mocked(complianceApi.listViolations).mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        pageSize: 20,
      });

      const params = { constraintName: "require-team-label" };
      const { result } = renderHook(() => useViolations(params), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(complianceApi.listViolations).toHaveBeenCalledWith(params);
    });
  });

  describe("useRecentViolations", () => {
    it("should fetch recent violations with limit", async () => {
      const mockResponse = {
        items: [
          {
            constraintKind: "K8sRequiredLabels",
            constraintName: "require-team-label",
            enforcementAction: "warn",
            message: "Missing label",
            resource: { kind: "Deployment", name: "my-app", namespace: "default", apiVersion: "apps/v1" },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 5,
      };
      vi.mocked(complianceApi.listViolations).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useRecentViolations(5), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(complianceApi.listViolations).toHaveBeenCalledWith({ pageSize: 5, page: 1 });
    });

    it("should use default limit of 10", async () => {
      vi.mocked(complianceApi.listViolations).mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        pageSize: 10,
      });

      const { result } = renderHook(() => useRecentViolations(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(complianceApi.listViolations).toHaveBeenCalledWith({ pageSize: 10, page: 1 });
    });
  });

  describe("useViolationCount", () => {
    it("should return violation count from summary", async () => {
      vi.mocked(complianceApi.getComplianceSummary).mockResolvedValue({
        totalTemplates: 10,
        totalConstraints: 25,
        totalViolations: 42,
        byEnforcement: { deny: 20, warn: 22 },
      });

      const { result } = renderHook(() => useViolationCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toBe(42);
    });

    it("should return undefined when no data", async () => {
      vi.mocked(complianceApi.getComplianceSummary).mockResolvedValue({
        totalTemplates: 0,
        totalConstraints: 0,
        totalViolations: 0,
        byEnforcement: {},
      });

      const { result } = renderHook(() => useViolationCount(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // 0 is falsy, so it should still return the number
      expect(result.current.data).toBe(0);
    });

    it("should return undefined when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useViolationCount(), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.data).toBeUndefined();
      expect(complianceApi.getComplianceSummary).not.toHaveBeenCalled();
    });
  });
});
