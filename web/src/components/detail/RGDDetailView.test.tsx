// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { RGDDetailView } from "./RGDDetailView";
import type { CatalogRGD } from "@/types/rgd";

// Mock hooks used by RGDDetailView
vi.mock("@/hooks/useRGDs", () => ({
  useRGD: () => ({ data: null, isLoading: false, error: null }),
  useRGDResourceGraph: () => ({ data: null, isLoading: false, error: null }),
}));

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
    version: "v1",
    tags: [],
    category: "database",
    labels: {},
    instances: 2,
    status: "Active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

function renderDetailView(rgd: CatalogRGD, onDeploy?: () => void) {
  return render(
    <MemoryRouter>
      <RGDDetailView rgd={rgd} onBack={vi.fn()} onDeploy={onDeploy} />
    </MemoryRouter>
  );
}

describe("RGDDetailView", () => {
  it("shows Inactive badge when status is not Active", () => {
    renderDetailView(createTestRGD({ status: "Inactive" }));
    // Badge text + Overview Status value both show "Inactive"
    const inactiveElements = screen.getAllByText("Inactive");
    expect(inactiveElements.length).toBeGreaterThanOrEqual(1);
  });

  it("does not show Inactive badge for active RGDs", () => {
    renderDetailView(createTestRGD({ status: "Active" }));
    expect(screen.queryByText("Inactive")).not.toBeInTheDocument();
  });

  it("shows Status field in Overview tab", () => {
    renderDetailView(createTestRGD({ status: "Inactive" }));
    // The Overview tab shows Status as a label in the Details section
    expect(screen.getByText("Status")).toBeInTheDocument();
    // The value should show "Inactive" (in the details section, separate from the badge)
    const statusValues = screen.getAllByText("Inactive");
    // At least 2: one badge + one in Overview details
    expect(statusValues.length).toBeGreaterThanOrEqual(2);
  });

  it("renders Deploy button when onDeploy is provided", () => {
    renderDetailView(createTestRGD({ status: "Active" }), vi.fn());
    expect(screen.getByText("Deploy")).toBeInTheDocument();
  });

  it("does not render Deploy button when onDeploy is undefined", () => {
    renderDetailView(createTestRGD({ status: "Inactive" }));
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });
});
