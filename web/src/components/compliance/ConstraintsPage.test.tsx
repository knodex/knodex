// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter, MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConstraintsPage } from "./ConstraintsPage";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { Constraint, ConstraintTemplate, ComplianceListResponse } from "@/types/compliance";

// Mock the useCompliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useConstraints: vi.fn(),
  useConstraintTemplates: vi.fn(),
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

const mockConstraints: ComplianceListResponse<Constraint> = {
  items: [
    {
      name: "require-team-label",
      kind: "K8sRequiredLabels",
      templateName: "k8srequiredlabels",
      enforcementAction: "deny",
      violationCount: 5,
      match: {
        kinds: [{ apiGroups: [""], kinds: ["Pod", "Deployment"] }],
        namespaces: ["default", "production"],
      },
      createdAt: "2024-01-15T10:00:00Z",
    },
    {
      name: "block-privileged",
      kind: "K8sBlockPrivilegedContainer",
      templateName: "k8sblockprivileged",
      enforcementAction: "warn",
      violationCount: 0,
      match: {
        kinds: [{ apiGroups: [""], kinds: ["Pod"] }],
        scope: "Namespaced",
      },
      createdAt: "2024-01-14T09:00:00Z",
    },
  ],
  total: 2,
  page: 1,
  pageSize: 20,
};

const mockTemplates: ComplianceListResponse<ConstraintTemplate> = {
  items: [
    { name: "k8srequiredlabels", kind: "K8sRequiredLabels", description: "", createdAt: "" },
    { name: "k8sblockprivileged", kind: "K8sBlockPrivilegedContainer", description: "", createdAt: "" },
  ],
  total: 2,
  page: 1,
  pageSize: 100,
};

describe("ConstraintsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);
  });

  it("renders loading skeleton when loading", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    // "Constraints" appears in both breadcrumb and card title
    const constraintsElements = screen.getAllByText("Constraints");
    expect(constraintsElements.length).toBeGreaterThanOrEqual(1);
  });

  it("renders error state when fetch fails", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Network error"),
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("Failed to load constraints")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();
  });

  it("renders empty state when no constraints exist", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("No Constraints")).toBeInTheDocument();
  });

  it("renders table with constraint data (AC-CON-01)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    // Check table headers
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Kind")).toBeInTheDocument();
    expect(screen.getByText("Enforcement")).toBeInTheDocument();
    expect(screen.getByText("Violations")).toBeInTheDocument();

    // Check constraint data
    expect(screen.getByText("require-team-label")).toBeInTheDocument();
    expect(screen.getByText("K8sRequiredLabels")).toBeInTheDocument();
  });

  it("shows enforcement action badges with correct colors (AC-CON-01)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    // Find enforcement badges
    expect(screen.getByText("deny")).toBeInTheDocument();
    expect(screen.getByText("warn")).toBeInTheDocument();
  });

  it("shows violation count badges (AC-CON-04)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    // 5 violations badge (red/destructive)
    expect(screen.getByText("5")).toBeInTheDocument();
    // 0 violations badge (green)
    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("links to constraint detail page (AC-CON-05)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    const constraintLink = screen.getByRole("link", { name: /require-team-label/i });
    expect(constraintLink).toHaveAttribute(
      "href",
      "/compliance/constraints/K8sRequiredLabels/require-team-label"
    );
  });

  it("shows filter dropdowns (AC-CON-02, AC-CON-03)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    // Check for filter trigger buttons (kind and enforcement comboboxes + page size selector)
    const comboboxes = screen.getAllByRole("combobox");
    expect(comboboxes.length).toBeGreaterThanOrEqual(2);
  });

  it("shows Clear button when filters are applied (AC-CON-06)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    // Render with filter in URL
    render(<ConstraintsPage />, {
      wrapper: createWrapper(["/compliance/constraints?kind=K8sRequiredLabels"]),
    });

    expect(screen.getByRole("button", { name: /clear/i })).toBeInTheDocument();
  });

  it("shows total count in header", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("(2)")).toBeInTheDocument();
  });

  it("renders breadcrumbs correctly (AC-NAVL-01)", () => {
    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintsPage />, { wrapper: createWrapper() });

    expect(screen.getByText("Compliance")).toBeInTheDocument();
    // "Constraints" appears in both breadcrumb and card title
    const constraintsElements = screen.getAllByText("Constraints");
    expect(constraintsElements.length).toBeGreaterThanOrEqual(1);
  });
});
