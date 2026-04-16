// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { RGDMiniCard } from "./RGDMiniCard";
import type { CatalogRGD } from "@/types/rgd";

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD description",
    tags: ["database", "cache", "infra"],
    category: "compute",
    labels: {},
    instances: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderCard(rgd: CatalogRGD, action: React.ReactNode = <button>Action</button>) {
  return render(
    <MemoryRouter>
      <RGDMiniCard rgd={rgd} action={action} />
    </MemoryRouter>
  );
}

describe("RGDMiniCard", () => {
  it("renders title as a link to catalog detail", () => {
    renderCard(createTestRGD());
    const link = screen.getByRole("link", { name: "test-rgd" });
    expect(link).toHaveAttribute("href", "/catalog/test-rgd");
  });

  it("renders title from rgd.title when available", () => {
    renderCard(createTestRGD({ title: "My Custom Title" }));
    expect(screen.getByText("My Custom Title")).toBeInTheDocument();
  });

  it("falls back to rgd.name when title is absent", () => {
    renderCard(createTestRGD({ title: undefined }));
    expect(screen.getByText("test-rgd")).toBeInTheDocument();
  });

  it("renders description", () => {
    renderCard(createTestRGD());
    expect(screen.getByText("A test RGD description")).toBeInTheDocument();
  });

  it("does not render description when absent", () => {
    renderCard(createTestRGD({ description: "" }));
    expect(screen.queryByText("A test RGD description")).not.toBeInTheDocument();
  });

  it("renders first 3 tags", () => {
    renderCard(createTestRGD({ tags: ["a", "b", "c", "d", "e"] }));
    expect(screen.getByText("a")).toBeInTheDocument();
    expect(screen.getByText("b")).toBeInTheDocument();
    expect(screen.getByText("c")).toBeInTheDocument();
    expect(screen.queryByText("d")).not.toBeInTheDocument();
  });

  it("shows overflow count when more than 3 tags", () => {
    renderCard(createTestRGD({ tags: ["a", "b", "c", "d", "e"] }));
    expect(screen.getByText("+2")).toBeInTheDocument();
  });

  it("does not show overflow count for 3 or fewer tags", () => {
    renderCard(createTestRGD({ tags: ["a", "b"] }));
    expect(screen.queryByText(/\+/)).not.toBeInTheDocument();
  });

  it("renders the action slot", () => {
    renderCard(createTestRGD(), <button>Deploy Now</button>);
    expect(screen.getByText("Deploy Now")).toBeInTheDocument();
  });

  it("encodes RGD name in catalog link", () => {
    renderCard(createTestRGD({ name: "my rgd/special" }));
    const link = screen.getByRole("link");
    expect(link.getAttribute("href")).toContain(encodeURIComponent("my rgd/special"));
  });
});
