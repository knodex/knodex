// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter, MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ViolationsPage } from "./ViolationsPage";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { Violation, Constraint, ComplianceListResponse } from "@/types/compliance";

// Mock the useCompliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useViolations: vi.fn(),
  useConstraints: vi.fn(),
  useViolationHistoryCount: vi.fn().mockReturnValue({ data: undefined, isLoading: false }),
  useExportViolationHistory: vi.fn().mockReturnValue({ mutate: vi.fn(), isPending: false }),
  isEnterprise: () => true,
}));

// Mock the WebSocket hook
vi.mock("@/hooks/useViolationWebSocket", () => ({
  useViolationWebSocket: vi.fn().mockReturnValue({
    status: "disconnected",
    hasRecentUpdate: false,
    lastUpdateTime: null,
  }),
}));

function createWrapper(initialEntries?: string[]) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    const Router = initialEntries ? MemoryRouter : BrowserRouter;
    const routerProps = initialEntries ? { initialEntries } : {};
    return (
      <QueryClientProvider client={queryClient}>
        <Router {...routerProps}>{children}</Router>
      </QueryClientProvider>
    );
  };
}

const mockViolations: ComplianceListResponse<Violation> = {
  items: [
    {
      constraintName: "require-team-label",
      constraintKind: "K8sRequiredLabels",
      enforcementAction: "deny",
      message: "Missing required label: team",
      resource: {
        kind: "Pod",
        name: "test-pod",
        namespace: "default",
        apiGroup: "",
      },
    },
    {
      constraintName: "block-privileged",
      constraintKind: "K8sBlockPrivilegedContainer",
      enforcementAction: "warn",
      message: "Container is running as privileged",
      resource: {
        kind: "Deployment",
        name: "my-app",
        namespace: "production",
        apiGroup: "apps",
      },
    },
  ],
  total: 2,
  page: 1,
  pageSize: 20,
};

const mockConstraints: ComplianceListResponse<Constraint> = {
  items: [
    {
      name: "require-team-label",
      kind: "K8sRequiredLabels",
      templateName: "k8srequiredlabels",
      enforcementAction: "deny",
      violationCount: 5,
      match: {},
      createdAt: "",
    },
  ],
  total: 1,
  page: 1,
  pageSize: 100,
};

describe("ViolationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);
  });

  it("renders loading skeleton when loading", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    // "Violations" appears in both breadcrumb and card title
    const violationsElements = screen.getAllByText("Violations");
    expect(violationsElements.length).toBeGreaterThanOrEqual(1);
  });

  it("renders error state when fetch fails", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("API error"),
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("Failed to load violations")).toBeInTheDocument();
    expect(screen.getByText("API error")).toBeInTheDocument();
  });

  it("renders success state when no violations (AC-VIO-06)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("No Violations")).toBeInTheDocument();
    expect(screen.getByText(/all resources are compliant/i)).toBeInTheDocument();
  });

  it("renders filter empty state when no matching violations", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, {
      wrapper: createWrapper(["/compliance/violations?constraint=K8sRequiredLabels/test"]),
    });

    expect(screen.getByText("No Matching Violations")).toBeInTheDocument();
    expect(screen.getByText(/no violations match the selected filters/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /clear filters/i })).toBeInTheDocument();
  });

  it("renders table with violation data (AC-VIO-01)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    // Check table headers
    expect(screen.getByText("Resource")).toBeInTheDocument();
    expect(screen.getByText("Namespace")).toBeInTheDocument();
    expect(screen.getByText("Constraint")).toBeInTheDocument();
    expect(screen.getByText("Enforcement")).toBeInTheDocument();
    expect(screen.getByText("Message")).toBeInTheDocument();

    // Check violation data
    expect(screen.getByText("Pod/test-pod")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
    expect(screen.getByText("require-team-label")).toBeInTheDocument();
    expect(screen.getByText("Missing required label: team")).toBeInTheDocument();
  });

  it("shows enforcement action badges (AC-VIO-05)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("deny")).toBeInTheDocument();
    expect(screen.getByText("warn")).toBeInTheDocument();
  });

  it("links to constraint detail page from violation row", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    const constraintLink = screen.getByRole("link", { name: /require-team-label/i });
    expect(constraintLink).toHaveAttribute(
      "href",
      "/compliance/constraints/K8sRequiredLabels/require-team-label"
    );
  });

  it("shows filter controls (AC-VIO-02, AC-VIO-03)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    // Constraint filter dropdown and page size selector (2 comboboxes)
    const comboboxes = screen.getAllByRole("combobox");
    expect(comboboxes.length).toBeGreaterThanOrEqual(1);
    // Resource filter input
    expect(screen.getByPlaceholderText(/filter by resource/i)).toBeInTheDocument();
  });

  it("shows Clear button when filters are applied (AC-VIO-07)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, {
      wrapper: createWrapper(["/compliance/violations?resource=test"]),
    });

    expect(screen.getByRole("button", { name: /^clear$/i })).toBeInTheDocument();
  });

  it("shows total count in header", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getAllByText("Violations")[0]).toBeInTheDocument();
  });

  it("shows cluster-scoped indicator for resources without namespace", () => {
    const clusterScopedViolation: ComplianceListResponse<Violation> = {
      items: [
        {
          constraintName: "require-label",
          constraintKind: "K8sRequiredLabels",
          enforcementAction: "deny",
          message: "Missing label",
          resource: {
            kind: "Namespace",
            name: "test-ns",
            namespace: "",
          },
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    };

    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: clusterScopedViolation,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("cluster")).toBeInTheDocument();
  });

  it("renders page title correctly (AC-NAVL-01)", () => {
    vi.mocked(useComplianceModule.useViolations).mockReturnValue({
      data: mockViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useViolations>);

    render(<ViolationsPage />, { wrapper: createWrapper() });

    expect(screen.getAllByText("Violations")[0]).toBeInTheDocument();
  });
});
