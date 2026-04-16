// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StatusIndicator } from "./status-indicator";

describe("StatusIndicator", () => {
  describe("dot variant (default)", () => {
    it("renders a circle dot with no label for healthy status", () => {
      const { container } = render(<StatusIndicator status="healthy" />);
      const dot = container.querySelector("[data-testid='status-dot']");
      expect(dot).toBeTruthy();
      expect(container.textContent).toBe("");
    });

    it("renders without text for each status in dot variant", () => {
      const statuses = ["healthy", "warning", "error", "progressing", "inactive", "unknown"] as const;
      for (const status of statuses) {
        const { container } = render(<StatusIndicator status={status} />);
        expect(container.textContent).toBe("");
      }
    });

    it("uses default variant=dot when variant prop is omitted", () => {
      const { container } = render(<StatusIndicator status="healthy" />);
      expect(container.textContent).toBe("");
    });
  });

  describe("dot colors via CSS tokens", () => {
    it("applies healthy color and glow box-shadow", () => {
      const { container } = render(<StatusIndicator status="healthy" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toContain("var(--status-healthy)");
      expect(dot.style.boxShadow).toContain("var(--status-healthy)");
    });

    it("applies warning color with no glow", () => {
      const { container } = render(<StatusIndicator status="warning" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toContain("var(--status-warning)");
      expect(dot.style.boxShadow).toBeFalsy();
    });

    it("applies error color with no glow", () => {
      const { container } = render(<StatusIndicator status="error" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toContain("var(--status-error)");
      expect(dot.style.boxShadow).toBeFalsy();
    });

    it("applies info color on progressing status", () => {
      const { container } = render(<StatusIndicator status="progressing" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toContain("var(--status-info)");
    });

    it("applies inactive color on inactive status", () => {
      const { container } = render(<StatusIndicator status="inactive" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toContain("var(--status-inactive)");
    });

    it("renders unknown as dashed outline with no fill via inline style", () => {
      const { container } = render(<StatusIndicator status="unknown" />);
      const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
      expect(dot.style.backgroundColor).toBeFalsy();
      expect(dot.style.border).toContain("dashed");
      expect(dot.style.border).toContain("var(--status-inactive)");
    });
  });

  describe("progressing animation", () => {
    it("applies animate-status-pulse class on progressing status", () => {
      const { container } = render(<StatusIndicator status="progressing" />);
      const dot = container.querySelector("[data-testid='status-dot']");
      expect(dot?.className).toContain("animate-status-pulse");
    });

    it("does NOT apply animate-status-pulse on other statuses", () => {
      const { container } = render(<StatusIndicator status="healthy" />);
      const dot = container.querySelector("[data-testid='status-dot']");
      expect(dot?.className).not.toContain("animate-status-pulse");
    });
  });

  describe("dot-label variant", () => {
    it("renders dot + label for healthy", () => {
      render(<StatusIndicator status="healthy" variant="dot-label" />);
      expect(screen.getByText("Healthy")).toBeTruthy();
    });

    it("renders 'Warning' label for warning status", () => {
      render(<StatusIndicator status="warning" variant="dot-label" />);
      expect(screen.getByText("Warning")).toBeTruthy();
    });

    it("renders 'Failed' label for error status", () => {
      render(<StatusIndicator status="error" variant="dot-label" />);
      expect(screen.getByText("Failed")).toBeTruthy();
    });

    it("renders 'Progressing' label for progressing status", () => {
      render(<StatusIndicator status="progressing" variant="dot-label" />);
      expect(screen.getByText("Progressing")).toBeTruthy();
    });

    it("renders 'Inactive' label for inactive status", () => {
      render(<StatusIndicator status="inactive" variant="dot-label" />);
      expect(screen.getByText("Inactive")).toBeTruthy();
    });

    it("renders 'Unknown' label for unknown status", () => {
      render(<StatusIndicator status="unknown" variant="dot-label" />);
      expect(screen.getByText("Unknown")).toBeTruthy();
    });
  });

  describe("dot-count variant", () => {
    it("renders dot + numeric count", () => {
      render(<StatusIndicator status="healthy" variant="dot-count" count={5} />);
      expect(screen.getByText("5")).toBeTruthy();
    });

    it("renders count=0", () => {
      render(<StatusIndicator status="warning" variant="dot-count" count={0} />);
      expect(screen.getByText("0")).toBeTruthy();
    });
  });

  describe("accessibility", () => {
    it("has role=status on root element", () => {
      render(<StatusIndicator status="healthy" />);
      expect(screen.getByRole("status")).toBeTruthy();
    });

    it("has aria-label with status text", () => {
      render(<StatusIndicator status="healthy" />);
      const el = screen.getByRole("status");
      expect(el.getAttribute("aria-label")).toBe("Status: healthy");
    });

    it("has aria-label for error status", () => {
      render(<StatusIndicator status="error" />);
      const el = screen.getByRole("status");
      expect(el.getAttribute("aria-label")).toBe("Status: error");
    });

    it("has aria-label for unknown status", () => {
      render(<StatusIndicator status="unknown" />);
      const el = screen.getByRole("status");
      expect(el.getAttribute("aria-label")).toBe("Status: unknown");
    });
  });

  describe("className prop", () => {
    it("forwards className to root element", () => {
      const { container } = render(<StatusIndicator status="healthy" className="custom-class" />);
      expect(container.firstElementChild?.className).toContain("custom-class");
    });
  });
});
