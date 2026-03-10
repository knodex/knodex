// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComplianceDashboard } from "./ComplianceRoutes";
import * as useComplianceModule from "@/hooks/useCompliance";
import * as useLicenseModule from "@/hooks/useLicense";
import * as complianceApi from "@/api/compliance";

// Mock the compliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useComplianceSummary: vi.fn(),
  isEnterprise: vi.fn(),
}));

// Mock the license hooks
vi.mock("@/hooks/useLicense", () => ({
  useIsFeatureEnabled: vi.fn(),
}));

// Mock the compliance API
vi.mock("@/api/compliance", () => ({
  isEnterpriseRequired: vi.fn(),
  isGatekeeperUnavailable: vi.fn(),
}));

// Mock child components to simplify testing
vi.mock("@/components/compliance", () => ({
  ComplianceSummaryCards: vi.fn(() => (
    <div data-testid="summary-cards">Summary Cards</div>
  )),
  ViolationsByEnforcement: vi.fn(() => (
    <div data-testid="violations-by-enforcement">Violations by Enforcement</div>
  )),
  RecentViolations: vi.fn(() => (
    <div data-testid="recent-violations">Recent Violations</div>
  )),
  EnterpriseRequired: vi.fn(({ feature, description }) => (
    <div data-testid="enterprise-required">
      <span>Enterprise Required: {feature}</span>
      <span data-testid="enterprise-description">{description}</span>
    </div>
  )),
  GatekeeperUnavailable: vi.fn(({ message }) => (
    <div data-testid="gatekeeper-unavailable">Gatekeeper Unavailable: {message}</div>
  )),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe("ComplianceDashboard", () => {
  const mockSummary = {
    totalTemplates: 10,
    totalConstraints: 25,
    totalViolations: 5,
    byEnforcement: { deny: 2, warn: 3, dryrun: 0 },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
    vi.mocked(useLicenseModule.useIsFeatureEnabled).mockReturnValue(true); // Default: feature licensed
    vi.mocked(complianceApi.isEnterpriseRequired).mockReturnValue(false);
    vi.mocked(complianceApi.isGatekeeperUnavailable).mockReturnValue(false);
  });

  describe("Enterprise gating (AC-EE)", () => {
    it("shows enterprise required when not enterprise build (AC-EE-01)", () => {
      vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(false);
      vi.mocked(useLicenseModule.useIsFeatureEnabled).mockReturnValue(false);
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        isError: false,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("enterprise-required")).toBeInTheDocument();
      expect(screen.getByText(/Enterprise Required: Policy Compliance Dashboard/)).toBeInTheDocument();
    });

    it("shows enterprise required when enterprise build but feature not licensed (AC-6)", () => {
      // Enterprise build but compliance feature not in license
      vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
      vi.mocked(useLicenseModule.useIsFeatureEnabled).mockReturnValue(false);
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        isError: false,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      // Should show license gate message
      expect(screen.getByTestId("enterprise-required")).toBeInTheDocument();
      expect(screen.getByText(/does not include the Policy Compliance feature/)).toBeInTheDocument();
    });

    it("shows enterprise required on 402 API response (AC-EE-02)", () => {
      vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
      const error = Object.assign(new Error("Payment Required"), { status: 402 });
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: false,
        error,
        isError: true,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);
      vi.mocked(complianceApi.isEnterpriseRequired).mockReturnValue(true);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("enterprise-required")).toBeInTheDocument();
    });

    it("shows gatekeeper unavailable on 503 response", () => {
      vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
      const error = Object.assign(new Error("Service Unavailable"), { status: 503 });
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: false,
        error,
        isError: true,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);
      vi.mocked(complianceApi.isGatekeeperUnavailable).mockReturnValue(true);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("gatekeeper-unavailable")).toBeInTheDocument();
    });
  });

  describe("Dashboard layout (AC-LAYOUT)", () => {
    beforeEach(() => {
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: mockSummary,
        isLoading: false,
        error: null,
        isError: false,
        isSuccess: true,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);
    });

    it("renders page header with correct title (AC-LAYOUT-02)", () => {
      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(
        screen.getByText("Monitor OPA Gatekeeper policy compliance across your clusters")
      ).toBeInTheDocument();
    });

    it("renders summary cards section (AC-LAYOUT-03)", () => {
      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("summary-cards")).toBeInTheDocument();
    });

    it("renders violations by enforcement breakdown (AC-LAYOUT-04)", () => {
      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("violations-by-enforcement")).toBeInTheDocument();
    });

    it("renders recent violations table (AC-LAYOUT-05)", () => {
      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByTestId("recent-violations")).toBeInTheDocument();
    });
  });

  describe("Loading state", () => {
    it("passes loading state to child components", () => {
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        isError: false,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      // Components should still render (they handle loading internally)
      expect(screen.getByTestId("summary-cards")).toBeInTheDocument();
      expect(screen.getByTestId("violations-by-enforcement")).toBeInTheDocument();
      expect(screen.getByTestId("recent-violations")).toBeInTheDocument();
    });
  });

  describe("Error handling", () => {
    it("shows generic error message for non-402/503 errors", () => {
      vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
      const error = new Error("Network error");
      vi.mocked(useComplianceModule.useComplianceSummary).mockReturnValue({
        data: undefined,
        isLoading: false,
        error,
        isError: true,
        isSuccess: false,
      } as ReturnType<typeof useComplianceModule.useComplianceSummary>);
      vi.mocked(complianceApi.isEnterpriseRequired).mockReturnValue(false);
      vi.mocked(complianceApi.isGatekeeperUnavailable).mockReturnValue(false);

      render(<ComplianceDashboard />, { wrapper: createWrapper() });

      expect(screen.getByText("Failed to Load Compliance Data")).toBeInTheDocument();
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });
});
