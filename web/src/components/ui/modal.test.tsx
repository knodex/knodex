// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Modal } from "./modal";

function Harness({
  onClose = vi.fn(),
  open = true,
  width,
  footer,
}: {
  onClose?: () => void;
  open?: boolean;
  width?: "sm" | "md" | "lg" | number;
  footer?: React.ReactNode;
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Sample dialog"
      width={width}
      footer={footer}
    >
      <button>First</button>
      <button>Second</button>
      <button>Third</button>
    </Modal>
  );
}

describe("Modal", () => {
  it("does not render content when open=false", () => {
    render(<Harness open={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders a dialog with role=dialog and aria-modal=true when open", () => {
    render(<Harness />);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("wires aria-labelledby to the title node", () => {
    render(<Harness />);
    const dialog = screen.getByRole("dialog");
    const labelledBy = dialog.getAttribute("aria-labelledby");
    expect(labelledBy).toBeTruthy();
    const title = document.getElementById(labelledBy!);
    expect(title).not.toBeNull();
    expect(title?.textContent).toBe("Sample dialog");
  });

  it("invokes onClose when Escape is pressed", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<Harness onClose={onClose} />);
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("renders a close button (Radix-provided) that fires onClose when clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<Harness onClose={onClose} />);
    const closeBtn = screen.getByRole("button", { name: /close/i });
    await user.click(closeBtn);
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("renders footer content when provided", () => {
    render(
      <Harness footer={<button data-testid="footer-btn">Save</button>} />
    );
    expect(screen.getByTestId("footer-btn")).toBeInTheDocument();
  });

  it("applies max-w class for string widths", () => {
    const { rerender } = render(<Harness width="sm" />);
    expect(screen.getByRole("dialog")).toHaveClass("max-w-sm");
    rerender(<Harness width="md" />);
    expect(screen.getByRole("dialog")).toHaveClass("max-w-lg");
    rerender(<Harness width="lg" />);
    expect(screen.getByRole("dialog")).toHaveClass("max-w-2xl");
  });

  it("applies inline maxWidth for numeric width", () => {
    render(<Harness width={720} />);
    const dialog = screen.getByRole("dialog");
    expect(dialog.style.maxWidth).toBe("720px");
  });

  it("moves focus inside the dialog when opened (Radix focus trap)", async () => {
    render(<Harness />);
    await waitFor(() => {
      const dialog = screen.getByRole("dialog");
      expect(dialog.contains(document.activeElement)).toBe(true);
    });
  });

  it("traps Tab focus inside the dialog (Tab from last focusable cycles back to first)", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const dialog = await screen.findByRole("dialog");

    // Collect the visible focusable elements inside the dialog in DOM order.
    // The dialog contains: First, Second, Third buttons + Radix-provided Close.
    const focusables = Array.from(
      dialog.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )
    );
    expect(focusables.length).toBeGreaterThanOrEqual(2);

    const first = focusables[0];
    const last = focusables[focusables.length - 1];

    // Pin focus on the last focusable, then Tab — focus must remain inside
    // the dialog (Radix focus-scope sentinel cycles back).
    last.focus();
    expect(document.activeElement).toBe(last);
    await user.tab();
    expect(dialog.contains(document.activeElement)).toBe(true);
    // The cycle wraps to the first focusable (Radix's documented behavior).
    expect(document.activeElement).toBe(first);
  });
});
