// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { RGDCard } from "./RGDCard";
import type { CatalogRGD } from "@/types/rgd";

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
    version: "v1",
    tags: ["database"],
    category: "database",
    labels: {},
    instances: 3,
    status: "Active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

function renderCard(rgd: CatalogRGD) {
  return render(
    <TooltipProvider>
      <RGDCard rgd={rgd} />
    </TooltipProvider>
  );
}

describe("RGDCard", () => {
  it("does not show Inactive badge for active RGDs", () => {
    renderCard(createTestRGD({ status: "Active" }));
    expect(screen.queryByText("Inactive")).not.toBeInTheDocument();
  });

  it("renders Inactive badge when status is not Active", () => {
    renderCard(createTestRGD({ status: "Inactive" }));
    expect(screen.getByText("Inactive")).toBeInTheDocument();
  });

  it("renders Inactive badge when status is empty", () => {
    renderCard(createTestRGD({ status: "" }));
    expect(screen.getByText("Inactive")).toBeInTheDocument();
  });

  it("applies muted styling for inactive RGDs", () => {
    const { container } = renderCard(createTestRGD({ status: "Inactive" }));
    const card = container.firstChild as HTMLElement;
    expect(card).toHaveClass("opacity-60");
  });

  it("does not apply muted styling for active RGDs", () => {
    const { container } = renderCard(createTestRGD({ status: "Active" }));
    const card = container.firstChild as HTMLElement;
    expect(card).not.toHaveClass("opacity-60");
  });

  it("still shows instance count for inactive RGDs", () => {
    renderCard(createTestRGD({ status: "Inactive", instances: 5 }));
    expect(screen.getByText("5 instances")).toBeInTheDocument();
  });
});
