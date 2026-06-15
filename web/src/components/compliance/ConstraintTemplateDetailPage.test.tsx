// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConstraintTemplateDetailPage } from "./ConstraintTemplateDetailPage";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { ConstraintTemplate, Constraint, ComplianceListResponse } from "@/types/compliance";

// Mock the useCompliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useConstraintTemplate: vi.fn(),
  useConstraints: vi.fn(),
  useCreateConstraint: () => ({
    mutateAsync: vi.fn(),
    reset: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
  isEnterprise: () => true,
}));

function createWrapper(initialEntries: string[]) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={initialEntries}>
          <Routes>
            <Route path="/compliance/templates/:name" element={children} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

const mockTemplate: ConstraintTemplate = {
  name: "k8srequiredlabels",
  kind: "K8sRequiredLabels",
  description: "Requires resources to contain specified labels with values matching a regex",
  rego: `package k8srequiredlabels

violation[{"msg": msg, "details": {"missing_labels": missing}}] {
  provided := {label | input.review.object.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  count(missing) > 0
  msg := sprintf("Missing required labels: %v", [missing])
}`,
  parameters: {
    labels: {
      type: "array",
      items: { type: "string" },
    },
  },
  labels: {
    "gatekeeper.sh/source": "library",
    "gatekeeper.sh/policy": "required-labels",
  },
  createdAt: "2024-01-15T10:00:00Z",
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
      createdAt: "2024-01-16T10:00:00Z",
    },
    {
      name: "require-owner-label",
      kind: "K8sRequiredLabels",
      templateName: "k8srequiredlabels",
      enforcementAction: "warn",
      violationCount: 0,
      match: {},
      createdAt: "2024-01-17T10:00:00Z",
    },
  ],
  total: 2,
  page: 1,
  pageSize: 100,
};

describe("ConstraintTemplateDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading skeleton when loading", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Loading skeletons should be present
    // We can't easily check for Skeletons, but the page should not show template data
    expect(screen.queryByText("K8sRequiredLabels")).not.toBeInTheDocument();
  });

  it("renders error state when fetch fails", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Template not found"),
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/nonexistent"]),
    });

    expect(screen.getByText("Failed to load template")).toBeInTheDocument();
    expect(screen.getByText("Template not found")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("renders not found state when template is null", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/missing"]),
    });

    // "Template Not Found" appears in both PageHeader title and ErrorState message
    const notFoundElements = screen.getAllByText("Template Not Found");
    expect(notFoundElements.length).toBeGreaterThan(0);
  });

  it("renders template details (AC-TPL-04)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Template name (appears in breadcrumb, title, RegoCodeViewer title)
    const templateNameElements = screen.getAllByText("k8srequiredlabels");
    expect(templateNameElements.length).toBeGreaterThan(0);
    expect(screen.getByText("K8sRequiredLabels")).toBeInTheDocument();

    // Description (appears in subtitle and description section)
    const descriptionElements = screen.getAllByText(/requires resources to contain specified labels/i);
    expect(descriptionElements.length).toBeGreaterThan(0);
  });

  it("renders Rego code with syntax highlighting (AC-TPL-06)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    const { container } = render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Should show Rego Policy section (appears in CardTitle and RegoCodeViewer)
    const regoPolicyElements = screen.getAllByText("Rego Policy");
    expect(regoPolicyElements.length).toBeGreaterThan(0);
    // Should contain Rego code content - text may be split by syntax highlighting
    // Check that it's present in container text
    expect(container.textContent).toContain("package");
    expect(container.textContent).toContain("k8srequiredlabels");
  });

  it("lists constraints using this template (AC-TPL-05)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Should show constraints section
    expect(screen.getByText(/Constraints Using This Template/i)).toBeInTheDocument();
    expect(screen.getByText("require-team-label")).toBeInTheDocument();
    expect(screen.getByText("require-owner-label")).toBeInTheDocument();
  });

  it("links constraints to their detail pages (AC-TPL-02)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    const constraintLink = screen.getByRole("link", { name: /require-team-label/i });
    expect(constraintLink).toHaveAttribute(
      "href",
      "/compliance/constraints/K8sRequiredLabels/require-team-label"
    );
  });

  it("shows parameter schema (AC-TPL-07)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Should show parameters schema section
    expect(screen.getByText("Parameters Schema")).toBeInTheDocument();
    // Should show parameter content (labels appears in multiple places - sidebar constraint names, parameter schema)
    const labelsElements = screen.getAllByText(/labels/);
    expect(labelsElements.length).toBeGreaterThan(0);
  });

  it("shows labels when present", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Should show labels
    expect(screen.getByText(/gatekeeper.sh\/source: library/)).toBeInTheDocument();
  });

  it("renders breadcrumbs correctly (AC-NAVL-01)", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    expect(screen.getByText("Compliance")).toBeInTheDocument();
    expect(screen.getByText("Templates")).toBeInTheDocument();
  });

  it("shows constraint count in sidebar", () => {
    vi.mocked(useComplianceModule.useConstraintTemplate).mockReturnValue({
      data: mockTemplate,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraintTemplate>);

    vi.mocked(useComplianceModule.useConstraints).mockReturnValue({
      data: mockConstraints,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraints>);

    render(<ConstraintTemplateDetailPage />, {
      wrapper: createWrapper(["/compliance/templates/k8srequiredlabels"]),
    });

    // Should show constraint count badge in sidebar (count is 2)
    // "2" appears in multiple places including sidebar
    const twoElements = screen.getAllByText("2");
    expect(twoElements.length).toBeGreaterThan(0);
    // Should show constraints section title
    expect(screen.getByText(/Constraints Using This Template/i)).toBeInTheDocument();
  });
});
