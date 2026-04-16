// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { PageSkeleton } from "./page-skeleton";
import { Skeleton } from "./skeleton";

describe("PageSkeleton", () => {
  it("renders grid variant by default", () => {
    render(<PageSkeleton />);
    const skeleton = screen.getByTestId("page-skeleton");
    expect(skeleton).toBeInTheDocument();
    const grid = skeleton.querySelector(".grid");
    expect(grid).toBeInTheDocument();
    expect(grid).toHaveClass("sm:grid-cols-2", "lg:grid-cols-3", "xl:grid-cols-4");
  });

  it("renders list variant", () => {
    render(<PageSkeleton variant="list" />);
    const skeleton = screen.getByTestId("page-skeleton");
    expect(skeleton.querySelector(".grid")).not.toBeInTheDocument();
    const listContainer = skeleton.querySelector(".space-y-3");
    expect(listContainer).toBeInTheDocument();
    // Default 8 items
    expect(listContainer!.children.length).toBe(8);
  });

  it("renders detail variant", () => {
    const { container } = render(<PageSkeleton variant="detail" />);
    expect(container.querySelector(".grid.gap-4.sm\\:grid-cols-2")).toBeInTheDocument();
    expect(container.querySelector(".space-y-3")).not.toBeInTheDocument();
  });

  it("applies custom className", () => {
    render(<PageSkeleton className="my-custom-class" />);
    expect(screen.getByTestId("page-skeleton")).toHaveClass("my-custom-class");
  });

  it("renders correct number of card skeletons with cardCount prop", () => {
    render(<PageSkeleton cardCount={4} />);
    const grid = screen.getByTestId("page-skeleton").querySelector(".grid");
    expect(grid!.children.length).toBe(4);
  });

  it("applies animate-token-fade-in to container", () => {
    render(<PageSkeleton />);
    expect(screen.getByTestId("page-skeleton")).toHaveClass("animate-token-fade-in");
  });
});

describe("Skeleton base component", () => {
  it("uses token-shimmer class", () => {
    const { container } = render(<Skeleton />);
    expect(container.firstChild).toHaveClass("animate-token-shimmer");
  });

  it("has reduced-motion handling", () => {
    const { container } = render(<Skeleton />);
    expect(container.firstChild).toHaveClass("motion-reduce:animate-none");
    expect(container.firstChild).toHaveClass("motion-reduce:bg-[rgba(255,255,255,0.04)]");
  });
});
