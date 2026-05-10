// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConstraintTemplatesPage } from "./ConstraintTemplatesPage";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { ConstraintTemplate, ComplianceListResponse } from "@/types/compliance";

// Mock the useCompliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useConstraintTemplates: vi.fn(),
  isEnterprise: () => true,
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>{children}</BrowserRouter>
      </QueryClientProvider>
    );
  };
}

const mockTemplates: ComplianceListResponse<ConstraintTemplate> = {
  items: [
    {
      name: "k8srequiredlabels",
      kind: "K8sRequiredLabels",
      description: "Requires resources to contain specified labels",
      createdAt: "2024-01-15T10:00:00Z",
    },
    {
      name: "k8sblockprivilegedcontainer",
      kind: "K8sBlockPrivilegedContainer",
      description: "Blocks privileged containers",
      createdAt: "2024-01-14T09:00:00Z",
    },
  ],
  total: 2,
  page: 1,
  pageSize: 20,
};

describe("ConstraintTemplatesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading skeleton when loading", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    // Should show page header
    expect(screen.getAllByText("Constraint Templates")[0]).toBeInTheDocument();
  });

  it("renders error state when fetch fails", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Failed to fetch"),
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    // Error message uses "Failed to load templates"
    expect(screen.getByText("Failed to load templates")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch")).toBeInTheDocument();
    // Button says "Try Again"
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("renders empty state when no templates exist", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    expect(screen.getByText("No Constraint Templates")).toBeInTheDocument();
  });

  it("renders table with template data (AC-TPL-01)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    // Check table headers
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Kind")).toBeInTheDocument();
    expect(screen.getByText("Description")).toBeInTheDocument();

    // Check template data
    expect(screen.getByText("k8srequiredlabels")).toBeInTheDocument();
    expect(screen.getByText("K8sRequiredLabels")).toBeInTheDocument();
    expect(screen.getByText("Requires resources to contain specified labels")).toBeInTheDocument();

    expect(screen.getByText("k8sblockprivilegedcontainer")).toBeInTheDocument();
    expect(screen.getByText("K8sBlockPrivilegedContainer")).toBeInTheDocument();
  });

  it("links to template detail page (AC-TPL-03)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    // Look for link with template name text
    const templateLinks = screen.getAllByRole("link", { name: /k8srequiredlabels/i });
    // First link is to the detail page
    expect(templateLinks[0]).toHaveAttribute("href", "/compliance/templates/k8srequiredlabels");
  });

  it("shows total count in header", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    expect(screen.getAllByText("Constraint Templates")[0]).toBeInTheDocument();
  });

  it("renders page title correctly (AC-NAVL-01)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    expect(screen.getAllByText("Constraint Templates")[0]).toBeInTheDocument();
  });

  it("links kind to constraints page (AC-TPL-02)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplates).mockReturnValue({
      data: mockTemplates,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplates>);

    render(<ConstraintTemplatesPage />, { wrapper: createWrapper() });

    // Kind should link to constraints filtered by that kind
    const kindLink = screen.getByRole("link", { name: "K8sRequiredLabels" });
    expect(kindLink).toHaveAttribute("href", "/compliance/constraints?kind=K8sRequiredLabels");
  });
});
