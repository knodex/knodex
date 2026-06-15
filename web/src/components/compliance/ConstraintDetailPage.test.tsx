// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConstraintDetailPage } from "./ConstraintDetailPage";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { Constraint, Violation } from "@/types/compliance";

// Mock the useCompliance hooks
vi.mock("@/hooks/useCompliance", () => ({
  useConstraint: vi.fn(),
  useUpdateConstraintEnforcement: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
  })),
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
            <Route path="/compliance/constraints/:kind/:name" element={children} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

const mockViolations: Violation[] = [
  {
    constraintName: "require-team-label",
    constraintKind: "K8sRequiredLabels",
    enforcementAction: "deny",
    message: "Missing required label: team",
    resource: {
      kind: "Pod",
      name: "test-pod-1",
      namespace: "default",
      apiGroup: "",
    },
  },
  {
    constraintName: "require-team-label",
    constraintKind: "K8sRequiredLabels",
    enforcementAction: "deny",
    message: "Missing required label: team",
    resource: {
      kind: "Deployment",
      name: "my-deployment",
      namespace: "production",
      apiGroup: "apps",
    },
  },
];

const mockConstraint: Constraint = {
  name: "require-team-label",
  kind: "K8sRequiredLabels",
  templateName: "k8srequiredlabels",
  enforcementAction: "deny",
  violationCount: 2,
  violations: mockViolations,
  match: {
    kinds: [
      { apiGroups: [""], kinds: ["Pod", "ConfigMap"] },
      { apiGroups: ["apps"], kinds: ["Deployment", "StatefulSet"] },
    ],
    namespaces: ["default", "production"],
    scope: "Namespaced",
  },
  parameters: {
    labels: ["team", "owner", "environment"],
  },
  labels: {
    "policy-type": "required-labels",
    team: "platform",
  },
  createdAt: "2024-01-15T10:00:00Z",
};

const mockConstraintNoViolations: Constraint = {
  ...mockConstraint,
  violationCount: 0,
  violations: [],
};

describe("ConstraintDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading skeleton when loading", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Loading state should not show constraint data
    expect(screen.queryByText("require-team-label")).not.toBeInTheDocument();
  });

  it("renders error state when fetch fails", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Constraint not found"),
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/missing"]),
    });

    expect(screen.getByText("Failed to load constraint")).toBeInTheDocument();
    expect(screen.getByText("Constraint not found")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("renders not found state when constraint is null", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/missing"]),
    });

    // "Constraint Not Found" appears in both PageHeader title and ErrorState message
    const notFoundElements = screen.getAllByText("Constraint Not Found");
    expect(notFoundElements.length).toBeGreaterThan(0);
  });

  it("renders constraint details (AC-CON-07)", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Constraint name (appears in breadcrumb, title, and other places)
    const constraintNameElements = screen.getAllByText("require-team-label");
    expect(constraintNameElements.length).toBeGreaterThan(0);
    // Kind
    expect(screen.getByText("K8sRequiredLabels")).toBeInTheDocument();
    // Template link
    expect(screen.getByText("k8srequiredlabels")).toBeInTheDocument();
    // Enforcement action
    expect(screen.getAllByText("deny").length).toBeGreaterThan(0);
  });

  it("shows match rules (AC-CON-08)", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Match Rules section
    expect(screen.getByText("Match Rules")).toBeInTheDocument();
    // Should show kinds (Pod appears multiple times - in MatchRulesDisplay and violation table)
    const podElements = screen.getAllByText(/Pod/);
    expect(podElements.length).toBeGreaterThan(0);
    // Should show namespaces (default appears in multiple places - match rules and violations)
    const defaultElements = screen.getAllByText(/default/);
    expect(defaultElements.length).toBeGreaterThan(0);
  });

  it("shows parameters when present", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Parameters section
    expect(screen.getByText("Parameters")).toBeInTheDocument();
    // Should show parameter values
    expect(screen.getByText(/"team"/)).toBeInTheDocument();
  });

  it("shows violation count badge - with violations", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Should show violation count (destructive badge)
    expect(screen.getByText("2 violations")).toBeInTheDocument();
  });

  it("shows no violations badge - without violations", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraintNoViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Should show no violations badge
    expect(screen.getByText("No violations")).toBeInTheDocument();
  });

  it("lists violations in table", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Violations section should appear (appears in header and sidebar Quick Stats)
    const violationsElements = screen.getAllByText("Violations");
    expect(violationsElements.length).toBeGreaterThan(0);
    // Should show violation resources
    expect(screen.getByText("Pod/test-pod-1")).toBeInTheDocument();
    expect(screen.getByText("Deployment/my-deployment")).toBeInTheDocument();
    // Should show violation messages
    expect(screen.getAllByText("Missing required label: team").length).toBe(2);
  });

  it("hides violations section when no violations", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraintNoViolations,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // No violations table should be rendered
    expect(screen.queryByText("Pod/test-pod-1")).not.toBeInTheDocument();
  });

  it("links to template detail page", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Find the template link specifically - it links to /compliance/templates/
    // (the kind link goes to /compliance/constraints?kind=)
    const allLinks = screen.getAllByRole("link");
    const templateLink = allLinks.find(link =>
      link.getAttribute("href")?.includes("/compliance/templates/")
    );
    expect(templateLink).toHaveAttribute("href", "/compliance/templates/k8srequiredlabels");
  });

  it("links to violations page filtered by constraint", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    const viewAllLink = screen.getByRole("link", { name: /view all violations/i });
    expect(viewAllLink).toHaveAttribute(
      "href",
      "/compliance/violations?constraint=K8sRequiredLabels/require-team-label"
    );
  });

  it("shows labels when present", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    // Should show labels
    expect(screen.getByText(/policy-type: required-labels/)).toBeInTheDocument();
    expect(screen.getByText(/team: platform/)).toBeInTheDocument();
  });

  it("renders breadcrumbs correctly (AC-NAVL-01)", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    expect(screen.getByText("Compliance")).toBeInTheDocument();
    expect(screen.getByText("Constraints")).toBeInTheDocument();
  });

  it("shows Quick Stats sidebar", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    expect(screen.getByText("Quick Stats")).toBeInTheDocument();
    expect(screen.getByText("Target Kinds")).toBeInTheDocument();
    // "Namespaces" appears in Quick Stats sidebar and in MatchRulesDisplay
    const namespacesElements = screen.getAllByText("Namespaces");
    expect(namespacesElements.length).toBeGreaterThan(0);
  });

  it("shows Related links sidebar", () => {
    vi.mocked(useComplianceModule.useConstraint).mockReturnValue({
      data: mockConstraint,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isRefetching: false,
    } as unknown as ReturnType<typeof useComplianceModule.useConstraint>);

    render(<ConstraintDetailPage />, {
      wrapper: createWrapper(["/compliance/constraints/K8sRequiredLabels/require-team-label"]),
    });

    expect(screen.getByText("Related")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /view template/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /similar constraints/i })).toBeInTheDocument();
  });
});
