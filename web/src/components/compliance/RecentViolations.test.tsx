import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RecentViolations } from "./RecentViolations";
import * as useComplianceModule from "@/hooks/useCompliance";

// Mock the useCompliance hook
vi.mock("@/hooks/useCompliance", () => ({
  useRecentViolations: vi.fn(),
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
        <BrowserRouter>{children}</BrowserRouter>
      </QueryClientProvider>
    );
  };
}

describe("RecentViolations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the card with title and description", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 10 },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    expect(screen.getByText("Recent Violations")).toBeInTheDocument();
    expect(
      screen.getByText("Latest policy violations detected by Gatekeeper")
    ).toBeInTheDocument();
  });

  it("displays violations in a table", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: {
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
          {
            constraintKind: "K8sContainerLimits",
            constraintName: "container-limits",
            enforcementAction: "warn",
            message: "Container missing resource limits",
            resource: {
              kind: "Deployment",
              name: "my-app",
              namespace: "production",
              apiVersion: "apps/v1",
            },
          },
        ],
        total: 2,
        page: 1,
        pageSize: 10,
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    // Table headers
    expect(screen.getByText("Resource")).toBeInTheDocument();
    expect(screen.getByText("Constraint")).toBeInTheDocument();
    expect(screen.getByText("Enforcement")).toBeInTheDocument();
    expect(screen.getByText("Message")).toBeInTheDocument();

    // Resource names
    expect(screen.getByText("test-pod")).toBeInTheDocument();
    expect(screen.getByText("my-app")).toBeInTheDocument();

    // Constraint names (as links)
    expect(screen.getByText("require-team-label")).toBeInTheDocument();
    expect(screen.getByText("container-limits")).toBeInTheDocument();

    // Enforcement badges
    expect(screen.getByText("deny")).toBeInTheDocument();
    expect(screen.getByText("warn")).toBeInTheDocument();
  });

  it("shows loading state", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    // Title should still be visible
    expect(screen.getByText("Recent Violations")).toBeInTheDocument();
    // Table headers should be present in loading skeleton
    expect(screen.getAllByText("Resource")).toHaveLength(1);
  });

  it("shows empty state when no violations", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 10 },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    expect(screen.getByText("No Violations")).toBeInTheDocument();
    expect(
      screen.getByText("All resources are compliant with your Gatekeeper policies")
    ).toBeInTheDocument();
  });

  it("shows error state", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch violations"),
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    expect(screen.getByText("Failed to load violations")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch violations")).toBeInTheDocument();
  });

  it("shows View All link when violations exist", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: {
        items: [
          {
            constraintKind: "K8sRequiredLabels",
            constraintName: "test",
            enforcementAction: "deny",
            message: "Test message",
            resource: { kind: "Pod", name: "pod", namespace: "default", apiVersion: "v1" },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 10,
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    const viewAllLink = screen.getByRole("link", { name: /view all/i });
    expect(viewAllLink).toHaveAttribute("href", "/compliance/violations");
  });

  it("does not show View All link when no violations", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 10 },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    expect(screen.queryByRole("link", { name: /view all/i })).not.toBeInTheDocument();
  });

  it("passes limit prop to hook", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: { items: [], total: 0, page: 1, pageSize: 5 },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations limit={5} />, { wrapper: createWrapper() });

    expect(useComplianceModule.useRecentViolations).toHaveBeenCalledWith(5);
  });

  it("links constraint names to constraint detail pages", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: {
        items: [
          {
            constraintKind: "K8sRequiredLabels",
            constraintName: "require-team-label",
            enforcementAction: "deny",
            message: "Test",
            resource: { kind: "Pod", name: "pod", namespace: "default", apiVersion: "v1" },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 10,
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    const constraintLink = screen.getByRole("link", { name: "require-team-label" });
    expect(constraintLink).toHaveAttribute(
      "href",
      "/compliance/constraints/K8sRequiredLabels/require-team-label"
    );
  });

  it("displays resource kind and namespace info", () => {
    vi.mocked(useComplianceModule.useRecentViolations).mockReturnValue({
      data: {
        items: [
          {
            constraintKind: "K8sRequiredLabels",
            constraintName: "test",
            enforcementAction: "deny",
            message: "Test",
            resource: {
              kind: "Deployment",
              name: "my-app",
              namespace: "production",
              apiVersion: "apps/v1",
            },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 10,
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useComplianceModule.useRecentViolations>);

    render(<RecentViolations />, { wrapper: createWrapper() });

    expect(screen.getByText("my-app")).toBeInTheDocument();
    expect(screen.getByText("Deployment in production")).toBeInTheDocument();
  });
});
