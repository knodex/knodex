// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AdvancedConfigToggle, useAdvancedConfigToggle } from "./AdvancedConfigToggle";
import { renderHook, act } from "@testing-library/react";
import type { AdvancedSection } from "@/types/rgd";

describe("AdvancedConfigToggle", () => {
  const mockAdvancedSection: AdvancedSection = {
    path: "advanced",
    affectedProperties: [
      "advanced.replicas",
      "advanced.resources",
      "advanced.resources.limits",
      "advanced.resources.limits.memory",
      "advanced.resources.limits.cpu",
    ],
  };

  it("renders nothing when advancedSection is null", () => {
    const { container } = render(
      <AdvancedConfigToggle
        advancedSection={null}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when advancedSection has no affected properties", () => {
    const emptySection: AdvancedSection = {
      path: "advanced",
      affectedProperties: [],
    };

    const { container } = render(
      <AdvancedConfigToggle
        advancedSection={emptySection}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders toggle button when advancedSection has properties", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    expect(screen.getByRole("button")).toBeInTheDocument();
    expect(screen.getByText(/Show Advanced Configuration/i)).toBeInTheDocument();
  });

  it("shows correct option count (leaf properties only)", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    // Should count only leaf properties: replicas, memory, cpu (not resources or resources.limits)
    expect(screen.getByText(/3 options/i)).toBeInTheDocument();
  });

  it("calls onToggle when button is clicked", () => {
    const onToggle = vi.fn();

    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={false}
        onToggle={onToggle}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("hides children when collapsed", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div data-testid="advanced-content">Advanced Content</div>
      </AdvancedConfigToggle>
    );

    expect(screen.queryByTestId("advanced-content")).not.toBeInTheDocument();
  });

  it("shows children when expanded", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={true}
        onToggle={vi.fn()}
      >
        <div data-testid="advanced-content">Advanced Content</div>
      </AdvancedConfigToggle>
    );

    expect(screen.getByTestId("advanced-content")).toBeInTheDocument();
  });

  it("shows 'Hide' text when expanded", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={true}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    expect(screen.getByText(/Hide Advanced Configuration/i)).toBeInTheDocument();
  });

  it("shows secure defaults message when expanded", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={true}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    expect(
      screen.getByText(/These settings have secure defaults/i)
    ).toBeInTheDocument();
  });

  it("has correct aria attributes", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={false}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(button).toHaveAttribute("aria-controls", "advanced-config-section");
  });

  it("updates aria-expanded when expanded", () => {
    render(
      <AdvancedConfigToggle
        advancedSection={mockAdvancedSection}
        isExpanded={true}
        onToggle={vi.fn()}
      >
        <div>Children</div>
      </AdvancedConfigToggle>
    );

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-expanded", "true");
  });
});

describe("useAdvancedConfigToggle", () => {
  it("starts collapsed by default", () => {
    const { result } = renderHook(() => useAdvancedConfigToggle());
    expect(result.current.isExpanded).toBe(false);
  });

  it("starts expanded when initialExpanded is true", () => {
    const { result } = renderHook(() => useAdvancedConfigToggle(true));
    expect(result.current.isExpanded).toBe(true);
  });

  it("toggles state correctly", () => {
    const { result } = renderHook(() => useAdvancedConfigToggle());

    expect(result.current.isExpanded).toBe(false);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.isExpanded).toBe(true);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.isExpanded).toBe(false);
  });

  it("expand sets state to true", () => {
    const { result } = renderHook(() => useAdvancedConfigToggle());

    act(() => {
      result.current.expand();
    });
    expect(result.current.isExpanded).toBe(true);

    // Should stay true even if called again
    act(() => {
      result.current.expand();
    });
    expect(result.current.isExpanded).toBe(true);
  });

  it("collapse sets state to false", () => {
    const { result } = renderHook(() => useAdvancedConfigToggle(true));

    act(() => {
      result.current.collapse();
    });
    expect(result.current.isExpanded).toBe(false);

    // Should stay false even if called again
    act(() => {
      result.current.collapse();
    });
    expect(result.current.isExpanded).toBe(false);
  });
});
