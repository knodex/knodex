// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CatalogCard } from "./catalog-card";
import { CatalogCardSkeleton } from "./catalog-card-skeleton";
import type { CatalogRGD } from "@/types/rgd";

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test resource graph definition",
    tags: ["database", "postgres", "cloud"],
    category: "database",
    labels: {},
    instances: 3,
    status: "Active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe("CatalogCard", () => {
  it("renders display name and description", () => {
    render(<CatalogCard rgd={createTestRGD({ title: "PostgreSQL DB" })} />);

    expect(screen.getByText("PostgreSQL DB")).toBeInTheDocument();
    expect(screen.getByText("A test resource graph definition")).toBeInTheDocument();
  });

  it("falls back to name when title is not set", () => {
    render(<CatalogCard rgd={createTestRGD({ title: undefined })} />);
    expect(screen.getByText("test-rgd")).toBeInTheDocument();
  });

  it("calls onCardClick when card body is clicked", () => {
    const handleCardClick = vi.fn();
    const rgd = createTestRGD();
    render(<CatalogCard rgd={rgd} onCardClick={handleCardClick} />);

    const card = screen.getByRole("button", {
      name: /view details for test-rgd/i,
    });
    fireEvent.click(card);
    expect(handleCardClick).toHaveBeenCalledWith(rgd);
  });

  it("shows 'No description available' when description is empty", () => {
    render(<CatalogCard rgd={createTestRGD({ description: "" })} />);
    expect(screen.getByText("No description available")).toBeInTheDocument();
  });
});

describe("CatalogCardSkeleton", () => {
  it("renders without errors", () => {
    const { container } = render(<CatalogCardSkeleton />);
    expect(container.firstChild).toBeInTheDocument();
  });

  it("uses animate-token-shimmer animation class", () => {
    const { container } = render(<CatalogCardSkeleton />);
    const shimmerElements = container.querySelectorAll(".animate-token-shimmer");
    expect(shimmerElements.length).toBeGreaterThan(0);
  });
});
