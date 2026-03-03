/**
 * Tests for ConfirmNamespaceRemovalDialog
 * Tests warning dialog content, instance count display, and action buttons
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmNamespaceRemovalDialog } from "./ConfirmNamespaceRemovalDialog";

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

describe("ConfirmNamespaceRemovalDialog", () => {
  const mockConfirm = vi.fn();
  const mockCancel = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  // AC-4a: Shows namespace being removed
  it("displays the namespace being removed (AC-4a)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    // Namespace appears in both the question text and the code element
    expect(screen.getAllByText("production").length).toBeGreaterThanOrEqual(1);
  });

  // AC-4b: Shows instance count
  it("displays instance count warning when instances exist (AC-4b)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={5}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/5 active instances found/i)).toBeInTheDocument();
  });

  // AC-4c: Shows access loss message
  it("displays access loss warning message (AC-4c)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="staging"
        instanceCount={3}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/project members to lose access/i)).toBeInTheDocument();
  });

  // AC-9: RGD visibility mention
  it("mentions RGD visibility impact (AC-9)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="staging"
        instanceCount={2}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/RGDs scoped to this project may also lose visibility/i)).toBeInTheDocument();
  });

  // AC-4d: Remove Anyway and Cancel buttons
  it("shows Remove Anyway and Cancel buttons (AC-4d)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={1}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByRole("button", { name: /remove anyway/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
  });

  // Confirm action
  it("calls onConfirm when Remove Anyway is clicked", async () => {
    const user = userEvent.setup();
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    await user.click(screen.getByRole("button", { name: /remove anyway/i }));
    expect(mockConfirm).toHaveBeenCalled();
  });

  // Cancel action
  it("calls onCancel when Cancel is clicked", async () => {
    const user = userEvent.setup();
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(mockCancel).toHaveBeenCalled();
  });

  // Loading state
  it("shows loading state while fetching instance count (AC-3.6)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={null}
        isLoadingCount={true}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/checking for affected instances/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /remove anyway/i })).toBeDisabled();
  });

  // No instances - safe removal message
  it("shows safe removal message when no instances found (AC-6.7)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="staging"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/no active instances were found/i)).toBeInTheDocument();
    expect(screen.getByText(/this removal should be safe/i)).toBeInTheDocument();
  });

  // Wildcard pattern - unknown impact warning
  it("shows unknown impact warning for wildcard patterns (H1 fix)", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="dev-*"
        instanceCount={null}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.getByText(/impact unknown/i)).toBeInTheDocument();
    expect(screen.getByText(/cannot be determined for wildcard/i)).toBeInTheDocument();
    // Should NOT show the misleading "safe" message
    expect(screen.queryByText(/this removal should be safe/i)).not.toBeInTheDocument();
  });

  // Not rendered when closed
  it("does not render when isOpen is false", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={false}
        namespace="production"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
      />
    );

    expect(screen.queryByTestId("alert-dialog")).not.toBeInTheDocument();
  });

  // Removing state
  it("disables buttons when isRemoving is true", () => {
    render(
      <ConfirmNamespaceRemovalDialog
        isOpen={true}
        namespace="production"
        instanceCount={0}
        isLoadingCount={false}
        onConfirm={mockConfirm}
        onCancel={mockCancel}
        isRemoving={true}
      />
    );

    expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled();
  });
});
