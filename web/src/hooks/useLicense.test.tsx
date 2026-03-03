import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useLicenseStatus,
  useIsLicensed,
  useIsFeatureEnabled,
  useIsGracePeriod,
  useUpdateLicense,
} from "./useLicense";
import * as licenseApi from "@/api/license";
import type { ReactNode } from "react";
import type { LicenseStatus } from "@/types/license";

// Mock the license API
vi.mock("@/api/license");

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

// Helper to create mock license status
function createMockStatus(overrides: Partial<LicenseStatus> = {}): LicenseStatus {
  return {
    licensed: true,
    enterprise: true,
    status: "valid",
    message: "License is valid",
    license: {
      licenseId: "lic_test_123",
      customer: "Test Customer",
      edition: "enterprise",
      features: ["compliance", "views"],
      maxUsers: 50,
      issuedAt: "2026-01-01T00:00:00Z",
      expiresAt: "2027-01-01T00:00:00Z",
    },
    ...overrides,
  };
}

describe("License hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to enterprise mode for most tests
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  describe("useLicenseStatus", () => {
    it("should fetch license status", async () => {
      const mockStatus = createMockStatus();
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(mockStatus);

      const { result } = renderHook(() => useLicenseStatus(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockStatus);
      expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
    });

    it("should handle errors", async () => {
      const error = Object.assign(new Error("Network error"), { status: 500 });
      vi.mocked(licenseApi.getLicenseStatus).mockRejectedValue(error);

      const { result } = renderHook(() => useLicenseStatus(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      }, { timeout: 5000 });

      expect(result.current.error).toBe(error);
    });

    it("should not retry on 401 auth errors", async () => {
      const authError = Object.assign(new Error("Unauthorized"), { status: 401 });
      vi.mocked(licenseApi.getLicenseStatus).mockRejectedValue(authError);

      const { result } = renderHook(() => useLicenseStatus(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      // Should only be called once (no retries on 401)
      expect(licenseApi.getLicenseStatus).toHaveBeenCalledTimes(1);
    });

    it("should not fetch when not enterprise", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useLicenseStatus(), {
        wrapper: createWrapper(),
      });

      // Wait a bit to ensure the query doesn't run
      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.fetchStatus).toBe("idle");
      expect(licenseApi.getLicenseStatus).not.toHaveBeenCalled();
    });
  });

  describe("useIsLicensed", () => {
    it("should return true when licensed", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ licensed: true })
      );

      const { result } = renderHook(() => useIsLicensed(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current).toBe(true);
      });
    });

    it("should return false when not licensed", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ licensed: false, status: "missing" })
      );

      const { result } = renderHook(() => useIsLicensed(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      // Initially false while loading, stays false after load
      expect(result.current).toBe(false);
    });

    it("should return false when not enterprise build", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useIsLicensed(), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current).toBe(false);
      expect(licenseApi.getLicenseStatus).not.toHaveBeenCalled();
    });

    it("should return false while loading", () => {
      vi.mocked(licenseApi.getLicenseStatus).mockImplementation(
        () => new Promise(() => {}) // Never resolves
      );

      const { result } = renderHook(() => useIsLicensed(), {
        wrapper: createWrapper(),
      });

      expect(result.current).toBe(false);
    });
  });

  describe("useIsFeatureEnabled", () => {
    it("should return true when licensed and feature is in license", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: true,
          license: {
            licenseId: "lic_test",
            customer: "Test",
            edition: "enterprise",
            features: ["compliance", "views"],
            maxUsers: 50,
            issuedAt: "2026-01-01T00:00:00Z",
            expiresAt: "2027-01-01T00:00:00Z",
          },
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("compliance"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current).toBe(true);
      });
    });

    it("should return false when feature is not in license", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: true,
          license: {
            licenseId: "lic_test",
            customer: "Test",
            edition: "enterprise",
            features: ["compliance"], // No views
            maxUsers: 50,
            issuedAt: "2026-01-01T00:00:00Z",
            expiresAt: "2027-01-01T00:00:00Z",
          },
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("views"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });

    it("should return true in grace period when feature is in license", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: true,
          status: "grace_period",
          gracePeriodEnd: "2026-02-10T00:00:00Z",
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("compliance"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current).toBe(true);
      });
    });

    it("should return true in expired read-only mode when feature was in license", async () => {
      // This tests AC-7: expired licenses still allow GET requests (read-only mode)
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: false, // Not licensed anymore
          status: "expired", // But expired (not missing)
          license: {
            licenseId: "lic_test",
            customer: "Test",
            edition: "enterprise",
            features: ["compliance", "views"], // Features were in the license
            maxUsers: 50,
            issuedAt: "2025-01-01T00:00:00Z",
            expiresAt: "2026-01-01T00:00:00Z", // Expired
          },
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("compliance"), {
        wrapper: createWrapper(),
      });

      // Should return true because backend still serves GET requests in read-only mode
      await waitFor(() => {
        expect(result.current).toBe(true);
      });
    });

    it("should return false when expired and feature was not in license", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: false,
          status: "expired",
          license: {
            licenseId: "lic_test",
            customer: "Test",
            edition: "enterprise",
            features: ["compliance"], // views was never in the license
            maxUsers: 50,
            issuedAt: "2025-01-01T00:00:00Z",
            expiresAt: "2026-01-01T00:00:00Z",
          },
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("views"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });

    it("should return false when license is missing", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          licensed: false,
          status: "missing",
          license: undefined,
        })
      );

      const { result } = renderHook(() => useIsFeatureEnabled("compliance"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });

    it("should return false when not enterprise build", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { result } = renderHook(() => useIsFeatureEnabled("compliance"), {
        wrapper: createWrapper(),
      });

      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current).toBe(false);
      expect(licenseApi.getLicenseStatus).not.toHaveBeenCalled();
    });
  });

  describe("useIsGracePeriod", () => {
    it("should return true when in grace period", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "grace_period",
          gracePeriodEnd: "2026-02-10T00:00:00Z",
        })
      );

      const { result } = renderHook(() => useIsGracePeriod(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current).toBe(true);
      });
    });

    it("should return false when license is valid", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ status: "valid" })
      );

      const { result } = renderHook(() => useIsGracePeriod(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });

    it("should return false when license is expired", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ status: "expired", licensed: false })
      );

      const { result } = renderHook(() => useIsGracePeriod(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });

    it("should return false when license is missing", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ status: "missing", licensed: false })
      );

      const { result } = renderHook(() => useIsGracePeriod(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(result.current).toBe(false);
    });
  });

  describe("useUpdateLicense", () => {
    it("should update license and invalidate queries", async () => {
      const newStatus = createMockStatus({
        license: {
          licenseId: "lic_new_456",
          customer: "New Customer",
          edition: "enterprise",
          features: ["compliance", "views"],
          maxUsers: 100,
          issuedAt: "2026-02-01T00:00:00Z",
          expiresAt: "2027-02-01T00:00:00Z",
        },
      });
      vi.mocked(licenseApi.updateLicense).mockResolvedValue(newStatus);

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useUpdateLicense(), { wrapper });

      // Trigger mutation
      result.current.mutate("new-jwt-token");

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(licenseApi.updateLicense).toHaveBeenCalledWith("new-jwt-token");

      // Should invalidate license, compliance, and views queries
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["license"] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["compliance"] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["views"] });
    });

    it("should handle update errors", async () => {
      const error = new Error("Invalid license token");
      vi.mocked(licenseApi.updateLicense).mockRejectedValue(error);

      const { result } = renderHook(() => useUpdateLicense(), {
        wrapper: createWrapper(),
      });

      result.current.mutate("invalid-token");

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });
  });
});
