// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Tests for ProjectDestinationsTab
 * Tests destination list display, add/remove flows, permission gating, and validation
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectDestinationsTab } from "./ProjectDestinationsTab";
import type { Project } from "@/types/project";

// Mock Radix UI Tooltip to avoid portal issues
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children, ...props }: { children: React.ReactNode; asChild?: boolean }) => (
    <span data-testid="tooltip-trigger" {...props}>{children}</span>
  ),
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <span data-testid="tooltip-content">{children}</span>
  ),
  TooltipProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// Mock AlertDialog to avoid portal issues
vi.mock("@/components/ui/alert-dialog", () => ({
  AlertDialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="alert-dialog">{children}</div> : null,
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert-dialog-content">{children}</div>
  ),
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogTitle: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <h2 className={className}>{children}</h2>
  ),
  AlertDialogDescription: ({ children }: { children: React.ReactNode; asChild?: boolean }) => (
    <div>{children}</div>
  ),
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

// Mock react-query useQuery (used for instance count check)
vi.mock("@tanstack/react-query", async () => {
  const actual = await vi.importActual("@tanstack/react-query");
  return {
    ...actual,
    useQuery: vi.fn().mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
    }),
  };
});

// Mock listInstances API call
vi.mock("@/api/rgd", () => ({
  listInstances: vi.fn().mockResolvedValue({ items: [], totalCount: 0 }),
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

const baseProject: Project = {
  name: "test-project",
  description: "A test project",
  destinations: [
    { namespace: "production" },
    { namespace: "staging" },
    { namespace: "dev-*", name: "Dev Wildcard" },
  ],
  roles: [],
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
};

const singleDestProject: Project = {
  ...baseProject,
  destinations: [{ namespace: "production" }],
};

describe("ProjectDestinationsTab", () => {
  const mockOnUpdate = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnUpdate.mockResolvedValue(undefined);
  });

  // AC-1: Destinations tab renders destination list
  it("renders all destinations correctly (AC-1)", () => {
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    expect(screen.getByText("production")).toBeInTheDocument();
    expect(screen.getByText("staging")).toBeInTheDocument();
    // dev-* appears in both the code element and as a wildcard badge, use getAllByText
    expect(screen.getAllByText("dev-*").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("(Dev Wildcard)")).toBeInTheDocument();
  });

  // AC-2: Add destination flow
  it("shows add input and button when canManage is true (AC-2)", () => {
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    expect(screen.getByPlaceholderText(/namespace/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add/i })).toBeInTheDocument();
  });

  it("calls onUpdate when adding a destination (AC-2)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "new-namespace");
    await user.click(screen.getByRole("button", { name: /add/i }));

    expect(mockOnUpdate).toHaveBeenCalledWith({
      destinations: [
        ...baseProject.destinations!,
        { namespace: "new-namespace" },
      ],
    });
  });

  // AC-3: Remove destination opens confirmation dialog
  it("opens confirmation dialog when removing a destination (AC-3)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const removeButtons = screen.getAllByLabelText("Remove destination");
    await user.click(removeButtons[0]); // Click remove on first destination

    // Confirmation dialog should appear
    expect(screen.getByTestId("alert-dialog")).toBeInTheDocument();
    expect(screen.getByText(/remove destination namespace/i)).toBeInTheDocument();
  });

  // AC-5: Minimum one destination
  it("disables remove button when only 1 destination remains (AC-5)", () => {
    render(
      <ProjectDestinationsTab
        project={singleDestProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const removeButtons = screen.getAllByLabelText("Remove destination");
    removeButtons.forEach((btn) => {
      expect(btn).toBeDisabled();
    });
  });

  // AC-6: Permission gating
  it("hides add/remove controls when canManage is false (AC-6)", () => {
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={false}
      />
    );

    // Add input and button should not be visible
    expect(screen.queryByPlaceholderText(/namespace/i)).not.toBeInTheDocument();
    // Remove buttons should not be visible
    expect(screen.queryByLabelText("Remove destination")).not.toBeInTheDocument();
  });

  // Validation: DNS-1123 pattern
  it("shows validation error for invalid namespace pattern (AC-1.6)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "INVALID_NS!");
    await user.click(screen.getByRole("button", { name: /add/i }));

    expect(mockOnUpdate).not.toHaveBeenCalled();
    expect(screen.getByText(/invalid namespace/i)).toBeInTheDocument();
  });

  // Duplicate detection
  it("shows error when adding a duplicate namespace", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "production");
    await user.click(screen.getByRole("button", { name: /add/i }));

    expect(mockOnUpdate).not.toHaveBeenCalled();
    expect(screen.getByText(/already exists/i)).toBeInTheDocument();
  });

  // Empty destinations state
  it("renders empty state when no destinations and cannot manage", () => {
    const emptyProject = { ...baseProject, destinations: [] };
    render(
      <ProjectDestinationsTab
        project={emptyProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={false}
      />
    );

    expect(screen.getByText(/no destinations configured/i)).toBeInTheDocument();
  });

  // Wildcard badge display
  it("shows wildcard badge for wildcard namespaces", () => {
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={false}
      />
    );

    expect(screen.getByText("wildcard")).toBeInTheDocument();
  });

  // Destination count badge
  it("shows destination count in badge", () => {
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={false}
      />
    );

    expect(screen.getByText("3 namespaces")).toBeInTheDocument();
  });

  // Wildcard namespace addition (regression test for regex bug)
  it("allows adding a wildcard namespace pattern like dev-* (AC-2)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "team-alpha-*");
    await user.click(screen.getByRole("button", { name: /add/i }));

    expect(mockOnUpdate).toHaveBeenCalledWith({
      destinations: [
        ...baseProject.destinations!,
        { namespace: "team-alpha-*" },
      ],
    });
  });

  // Error handling: add failure
  it("shows toast error when add destination fails", async () => {
    const user = userEvent.setup();
    mockOnUpdate.mockRejectedValue({ message: "validation failed" });
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "new-ns");
    await user.click(screen.getByRole("button", { name: /add/i }));

    const { toast } = await import("sonner");
    expect(toast.error).toHaveBeenCalled();
  });

  // Prefix wildcard validation (matches server IsWildcard)
  it("allows adding a prefix wildcard pattern like *-prod (AC-2)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "*-prod");
    await user.click(screen.getByRole("button", { name: /add/i }));

    expect(mockOnUpdate).toHaveBeenCalledWith({
      destinations: [
        ...baseProject.destinations!,
        { namespace: "*-prod" },
      ],
    });
  });

  // Enter key submission
  it("submits namespace on Enter key press (AC-2)", async () => {
    const user = userEvent.setup();
    render(
      <ProjectDestinationsTab
        project={baseProject}
        onUpdate={mockOnUpdate}
        isUpdating={false}
        canManage={true}
      />
    );

    const input = screen.getByPlaceholderText(/namespace/i);
    await user.type(input, "test-ns{Enter}");

    expect(mockOnUpdate).toHaveBeenCalledWith({
      destinations: [
        ...baseProject.destinations!,
        { namespace: "test-ns" },
      ],
    });
  });
});
