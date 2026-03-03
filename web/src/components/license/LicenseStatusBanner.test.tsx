import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LicenseStatusBanner } from "./LicenseStatusBanner";
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

function renderWithQueryClient(component: React.ReactNode) {
  const Wrapper = createWrapper();
  return render(<Wrapper>{component}</Wrapper>);
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

// Helper to wait for component to settle
async function waitForRender() {
  // Wait for React Query to settle
  await new Promise(resolve => setTimeout(resolve, 50));
}

describe("LicenseStatusBanner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to enterprise mode for most tests
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  describe("when not enterprise build", () => {
    it("should not render anything", async () => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = false;

      const { container } = renderWithQueryClient(<LicenseStatusBanner />);
      await waitForRender();

      expect(container.firstChild).toBeNull();
      expect(licenseApi.getLicenseStatus).not.toHaveBeenCalled();
    });
  });

  describe("when license is valid", () => {
    it("should not render any banner", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ status: "valid" })
      );

      const { container } = renderWithQueryClient(<LicenseStatusBanner />);

      // Wait for data to load using waitFor to avoid act() warnings
      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(container.firstChild).toBeNull();
    });
  });

  describe("when license status is oss", () => {
    it("should not render any banner", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({ status: "oss", licensed: false })
      );

      const { container } = renderWithQueryClient(<LicenseStatusBanner />);

      await waitFor(() => {
        expect(licenseApi.getLicenseStatus).toHaveBeenCalled();
      });

      expect(container.firstChild).toBeNull();
    });
  });

  describe("when license is in grace period", () => {
    it("should render warning banner with grace period end date", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "grace_period",
          gracePeriodEnd: "2026-02-11T00:00:00Z",
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);

      // Wait for the query to settle and component to render
      await screen.findByText("License Expired - Grace Period Active");

      expect(
        screen.getByText(/your enterprise license has expired/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/all features remain available until/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/please renew your license/i)
      ).toBeInTheDocument();
    });

    it("should show 'soon' when gracePeriodEnd is not provided", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "grace_period",
          gracePeriodEnd: undefined,
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);

      await screen.findByText("License Expired - Grace Period Active");
      expect(screen.getByText(/until soon/i)).toBeInTheDocument();
    });
  });

  describe("when license is expired", () => {
    it("should render destructive banner", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "expired",
          licensed: false,
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);

      await screen.findByText("License Expired");

      expect(
        screen.getByText(/your enterprise license has expired and the grace period has ended/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/enterprise features are unavailable/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/please contact your administrator to renew/i)
      ).toBeInTheDocument();
    });
  });

  describe("when license is missing", () => {
    it("should render default banner", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "missing",
          licensed: false,
          license: undefined,
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);

      await screen.findByText("No Enterprise License");

      expect(
        screen.getByText(/no enterprise license is installed/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/enterprise features are unavailable/i)
      ).toBeInTheDocument();
    });
  });

  describe("while loading", () => {
    it("should not render anything", async () => {
      // Never resolve the promise to simulate loading state
      vi.mocked(licenseApi.getLicenseStatus).mockImplementation(
        () => new Promise(() => {})
      );

      const { container } = renderWithQueryClient(<LicenseStatusBanner />);
      await waitForRender();

      // Should not show any banner while loading
      expect(container.querySelector("[role='alert']")).toBeNull();
    });
  });

  describe("alert variants", () => {
    it("should use warning variant for grace period", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "grace_period",
          gracePeriodEnd: "2026-02-11T00:00:00Z",
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);
      await screen.findByText("License Expired - Grace Period Active");

      // The alert should have warning styling (check by role)
      const alert = screen.getByRole("alert");
      expect(alert).toBeInTheDocument();
    });

    it("should use destructive variant for expired", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "expired",
          licensed: false,
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);
      await screen.findByText("License Expired");

      const alert = screen.getByRole("alert");
      expect(alert).toBeInTheDocument();
    });

    it("should use default variant for missing", async () => {
      vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
        createMockStatus({
          status: "missing",
          licensed: false,
          license: undefined,
        })
      );

      renderWithQueryClient(<LicenseStatusBanner />);
      await screen.findByText("No Enterprise License");

      const alert = screen.getByRole("alert");
      expect(alert).toBeInTheDocument();
    });
  });
});
