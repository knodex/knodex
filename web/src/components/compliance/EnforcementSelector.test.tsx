// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnforcementSelector } from "./EnforcementSelector";
import type { EnforcementAction } from "@/types/compliance";

// Store the onValueChange callback from the Select component
let selectOnValueChange: ((value: string) => void) | undefined;

// Mock Radix UI Select to avoid portal/pointer event issues in tests
vi.mock("@/components/ui/select", () => ({
  Select: ({ children, value: _value, onValueChange, disabled }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
    disabled?: boolean;
  }) => {
    selectOnValueChange = onValueChange;
    return (
      <div data-testid="select-root" data-disabled={disabled}>
        {children}
      </div>
    );
  },
  SelectTrigger: ({ children, className, ...props }: { children: React.ReactNode; className?: string }) => (
    <button
      role="combobox"
      className={className}
      disabled={props["disabled" as keyof typeof props]}
      data-disabled={props["disabled" as keyof typeof props] ? "" : undefined}
      {...props}
    >
      {children}
    </button>
  ),
  SelectValue: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="select-content">{children}</div>
  ),
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => (
    <div
      role="option"
      data-testid={`option-${value}`}
      onClick={() => selectOnValueChange?.(value)}
    >
      {children}
    </div>
  ),
}));

// Mock the ConfirmEnforcementDialog to simplify testing
vi.mock("./ConfirmEnforcementDialog", () => ({
  ConfirmEnforcementDialog: vi.fn(
    ({
      open,
      constraintKind,
      constraintName,
      currentAction,
      newAction,
      onConfirm,
      onCancel,
      isLoading,
    }) => {
      if (!open) return null;
      return (
        <div data-testid="confirm-dialog">
          <div data-testid="dialog-kind">{constraintKind}</div>
          <div data-testid="dialog-name">{constraintName}</div>
          <div data-testid="dialog-current">{currentAction}</div>
          <div data-testid="dialog-new">{newAction}</div>
          <button data-testid="dialog-confirm" onClick={onConfirm} disabled={isLoading}>
            {isLoading ? "Updating..." : "Confirm"}
          </button>
          <button data-testid="dialog-cancel" onClick={onCancel}>
            Cancel
          </button>
        </div>
      );
    }
  ),
}));

describe("EnforcementSelector", () => {
  const defaultProps = {
    currentAction: "dryrun" as EnforcementAction,
    constraintKind: "K8sRequiredLabels",
    constraintName: "require-team-label",
    onEnforcementChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    selectOnValueChange = undefined;
  });

  describe("rendering", () => {
    it("renders with current action displayed (AC-UI-01)", () => {
      render(<EnforcementSelector {...defaultProps} />);

      expect(screen.getByRole("combobox")).toBeInTheDocument();
      // Text appears both in trigger and dropdown - verify trigger contains it
      expect(screen.getByRole("combobox")).toHaveTextContent("Dry Run");
    });

    it("applies correct color styling for dryrun (AC-UI-02)", () => {
      render(<EnforcementSelector {...defaultProps} currentAction="dryrun" />);

      const trigger = screen.getByRole("combobox");
      expect(trigger.className).toMatch(/blue/i);
    });

    it("applies correct color styling for warn (AC-UI-02)", () => {
      render(<EnforcementSelector {...defaultProps} currentAction="warn" />);

      const trigger = screen.getByRole("combobox");
      expect(trigger.className).toMatch(/yellow/i);
    });

    it("applies correct color styling for deny (AC-UI-02)", () => {
      render(<EnforcementSelector {...defaultProps} currentAction="deny" />);

      const trigger = screen.getByRole("combobox");
      expect(trigger.className).toMatch(/red/i);
    });

    it("renders with canUpdate false shows data-disabled (AC-UI-03)", () => {
      render(<EnforcementSelector {...defaultProps} canUpdate={false} />);

      const selectRoot = screen.getByTestId("select-root");
      expect(selectRoot).toHaveAttribute("data-disabled", "true");
    });

    it("renders with isUpdating true shows data-disabled", () => {
      render(<EnforcementSelector {...defaultProps} isUpdating={true} />);

      const selectRoot = screen.getByTestId("select-root");
      expect(selectRoot).toHaveAttribute("data-disabled", "true");
    });

    it("applies custom className", () => {
      render(<EnforcementSelector {...defaultProps} className="custom-class" />);

      const trigger = screen.getByRole("combobox");
      expect(trigger).toHaveClass("custom-class");
    });
  });

  describe("selection behavior", () => {
    it("shows all enforcement options", () => {
      render(<EnforcementSelector {...defaultProps} />);

      // Check that all options are rendered
      expect(screen.getByTestId("option-dryrun")).toBeInTheDocument();
      expect(screen.getByTestId("option-warn")).toBeInTheDocument();
      expect(screen.getByTestId("option-deny")).toBeInTheDocument();
    });

    it("shows option descriptions", () => {
      render(<EnforcementSelector {...defaultProps} />);

      expect(screen.getByText(/Block resources/)).toBeInTheDocument();
      expect(screen.getByText(/Log warnings/)).toBeInTheDocument();
      expect(screen.getByText(/Audit violations/)).toBeInTheDocument();
    });

    it("opens confirmation dialog when selecting different action", async () => {
      const user = userEvent.setup();
      render(<EnforcementSelector {...defaultProps} />);

      // Click on the Deny option
      await user.click(screen.getByTestId("option-deny"));

      // Dialog should be visible
      expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();
      expect(screen.getByTestId("dialog-kind")).toHaveTextContent("K8sRequiredLabels");
      expect(screen.getByTestId("dialog-name")).toHaveTextContent("require-team-label");
      expect(screen.getByTestId("dialog-current")).toHaveTextContent("dryrun");
      expect(screen.getByTestId("dialog-new")).toHaveTextContent("deny");
    });

    it("does not open dialog when selecting same action", async () => {
      const user = userEvent.setup();
      render(<EnforcementSelector {...defaultProps} currentAction="deny" />);

      // Click on the currently selected Deny option
      await user.click(screen.getByTestId("option-deny"));

      // Dialog should not appear since it's the same action
      expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
    });
  });

  describe("confirmation flow", () => {
    it("calls onEnforcementChange when confirmed", async () => {
      const user = userEvent.setup();
      const mockOnChange = vi.fn().mockResolvedValue(undefined);
      render(<EnforcementSelector {...defaultProps} onEnforcementChange={mockOnChange} />);

      await user.click(screen.getByTestId("option-deny"));

      expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();

      await user.click(screen.getByTestId("dialog-confirm"));

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith("deny");
      });
    });

    it("closes dialog when cancel is clicked", async () => {
      const user = userEvent.setup();
      render(<EnforcementSelector {...defaultProps} />);

      await user.click(screen.getByTestId("option-deny"));

      expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();

      await user.click(screen.getByTestId("dialog-cancel"));

      // Dialog should be hidden
      expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
    });

    it("closes dialog after successful update", async () => {
      const user = userEvent.setup();
      const mockOnChange = vi.fn().mockResolvedValue(undefined);
      render(<EnforcementSelector {...defaultProps} onEnforcementChange={mockOnChange} />);

      await user.click(screen.getByTestId("option-deny"));
      await user.click(screen.getByTestId("dialog-confirm"));

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalled();
      });

      // Dialog should be closed after successful update
      await waitFor(() => {
        expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
      });
    });
  });

  describe("error handling", () => {
    it("keeps dialog open when update fails", async () => {
      const user = userEvent.setup();
      const mockOnChange = vi.fn().mockRejectedValue(new Error("Update failed"));
      render(<EnforcementSelector {...defaultProps} onEnforcementChange={mockOnChange} />);

      await user.click(screen.getByTestId("option-deny"));

      expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();

      await user.click(screen.getByTestId("dialog-confirm"));

      // Dialog should remain visible after error
      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalled();
      });

      // Dialog should still be visible
      expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("resets pending action when dialog is cancelled", async () => {
      const user = userEvent.setup();
      render(<EnforcementSelector {...defaultProps} />);

      // Select deny
      await user.click(screen.getByTestId("option-deny"));
      expect(screen.getByTestId("dialog-new")).toHaveTextContent("deny");

      // Cancel
      await user.click(screen.getByTestId("dialog-cancel"));

      // Select warn
      await user.click(screen.getByTestId("option-warn"));
      expect(screen.getByTestId("dialog-new")).toHaveTextContent("warn");
    });
  });
});
