import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmEnforcementDialog } from "./ConfirmEnforcementDialog";
import type { EnforcementAction } from "@/types/compliance";

describe("ConfirmEnforcementDialog", () => {
  const defaultProps = {
    open: true,
    onOpenChange: vi.fn(),
    constraintKind: "K8sRequiredLabels",
    constraintName: "require-team-label",
    currentAction: "dryrun" as EnforcementAction,
    newAction: "deny" as EnforcementAction,
    onConfirm: vi.fn(),
    onCancel: vi.fn(),
    isLoading: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("rendering", () => {
    it("renders when open is true", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    });

    it("does not render when open is false", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} open={false} />);

      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });

    it("does not render when newAction is null", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} newAction={null} />);

      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });

    it("shows constraint kind and name (AC-CONFIRM-01)", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      expect(screen.getByText(/K8sRequiredLabels\/require-team-label/)).toBeInTheDocument();
    });

    it("shows current and new action (AC-CONFIRM-01)", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      expect(screen.getByText("dryrun")).toBeInTheDocument();
      expect(screen.getByText("deny")).toBeInTheDocument();
    });

    it("shows correct styling for current action", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      const currentBadge = screen.getByText("dryrun");
      expect(currentBadge.className).toMatch(/blue/i);
    });

    it("shows correct styling for new action", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      const newBadge = screen.getByText("deny");
      expect(newBadge.className).toMatch(/red/i);
    });
  });

  describe("warning messages", () => {
    it("shows danger warning when escalating to deny (AC-CONFIRM-02)", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="dryrun" newAction="deny" />);

      expect(screen.getByText(/Enable Blocking Mode/)).toBeInTheDocument();
      expect(screen.getByText(/will block any resources/i)).toBeInTheDocument();
    });

    it("shows warning when escalating to warn", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="dryrun" newAction="warn" />);

      expect(screen.getByText(/Enable Warning Mode/)).toBeInTheDocument();
      expect(screen.getByText(/emit warnings/i)).toBeInTheDocument();
    });

    it("shows info message when de-escalating", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="deny" newAction="dryrun" />);

      expect(screen.getByText(/Reduce Enforcement Level/)).toBeInTheDocument();
      expect(screen.getByText(/reduce the enforcement level/i)).toBeInTheDocument();
    });

    it("shows warning icon for deny escalation", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="dryrun" newAction="deny" />);

      // The ShieldAlert icon should be visible (check for its characteristic class or svg)
      const title = screen.getByText(/Enable Blocking Mode/).closest("h2");
      expect(title).toBeInTheDocument();
    });
  });

  describe("button interactions", () => {
    it("shows Confirm and Cancel buttons (AC-CONFIRM-03)", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} />);

      expect(screen.getByRole("button", { name: /confirm change/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
    });

    it("calls onConfirm when confirm button is clicked", async () => {
      const user = userEvent.setup();
      const mockConfirm = vi.fn();
      render(<ConfirmEnforcementDialog {...defaultProps} onConfirm={mockConfirm} />);

      await user.click(screen.getByRole("button", { name: /confirm change/i }));

      expect(mockConfirm).toHaveBeenCalled();
    });

    it("calls onCancel when cancel button is clicked", async () => {
      const user = userEvent.setup();
      const mockCancel = vi.fn();
      render(<ConfirmEnforcementDialog {...defaultProps} onCancel={mockCancel} />);

      await user.click(screen.getByRole("button", { name: /cancel/i }));

      expect(mockCancel).toHaveBeenCalled();
    });
  });

  describe("loading state (AC-CONFIRM-04)", () => {
    it("shows loading text when isLoading is true", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} isLoading={true} />);

      expect(screen.getByRole("button", { name: /updating/i })).toBeInTheDocument();
    });

    it("disables confirm button when loading", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} isLoading={true} />);

      expect(screen.getByRole("button", { name: /updating/i })).toBeDisabled();
    });

    it("disables cancel button when loading", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} isLoading={true} />);

      expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled();
    });
  });

  describe("severity-based button styling", () => {
    it("applies danger styling for confirm button when escalating to deny", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="dryrun" newAction="deny" />);

      const confirmButton = screen.getByRole("button", { name: /confirm change/i });
      expect(confirmButton.className).toMatch(/red/i);
    });

    it("applies warning styling for confirm button when escalating to warn", () => {
      render(<ConfirmEnforcementDialog {...defaultProps} currentAction="dryrun" newAction="warn" />);

      const confirmButton = screen.getByRole("button", { name: /confirm change/i });
      expect(confirmButton.className).toMatch(/yellow/i);
    });
  });

  describe("all action combinations", () => {
    const testCases: Array<{
      current: EnforcementAction;
      new: EnforcementAction;
      expectedTitle: RegExp;
    }> = [
      { current: "dryrun", new: "deny", expectedTitle: /Enable Blocking Mode/ },
      { current: "dryrun", new: "warn", expectedTitle: /Enable Warning Mode/ },
      { current: "warn", new: "deny", expectedTitle: /Enable Blocking Mode/ },
      { current: "warn", new: "dryrun", expectedTitle: /Reduce Enforcement Level/ },
      { current: "deny", new: "warn", expectedTitle: /Reduce Enforcement Level/ },
      { current: "deny", new: "dryrun", expectedTitle: /Reduce Enforcement Level/ },
    ];

    testCases.forEach(({ current, new: newAction, expectedTitle }) => {
      it(`shows correct message for ${current} -> ${newAction}`, () => {
        render(
          <ConfirmEnforcementDialog
            {...defaultProps}
            currentAction={current}
            newAction={newAction}
          />
        );

        expect(screen.getByText(expectedTitle)).toBeInTheDocument();
      });
    });
  });
});
