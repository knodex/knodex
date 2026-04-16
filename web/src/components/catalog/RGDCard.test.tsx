// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { RGDCard } from "./RGDCard";
import type { CatalogRGD } from "@/types/rgd";

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
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
  it("renders custom icon img when rgd.icon is set", () => {
    renderCard(createTestRGD({ icon: "argocd" }));
    const img = screen.getByRole("img", { name: "argocd icon" });
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "/api/v1/icons/argocd");
  });

  it("does not render img when rgd.icon is not set", () => {
    renderCard(createTestRGD({ icon: undefined }));
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("falls back to CategoryIcon after img load error", () => {
    renderCard(createTestRGD({ icon: "unknown-brand" }));
    const img = screen.getByRole("img", { name: "unknown-brand icon" });

    // Simulate image load failure (triggers onError → setFailed(true))
    fireEvent.error(img);

    // After error, img is replaced by CategoryIcon (renders an SVG, not <img>)
    expect(screen.queryByRole("img", { name: "unknown-brand icon" })).not.toBeInTheDocument();
  });

  it("renders instance count", () => {
    renderCard(createTestRGD({ instances: 5 }));
    expect(screen.getByText("5 instances")).toBeInTheDocument();
  });
});
