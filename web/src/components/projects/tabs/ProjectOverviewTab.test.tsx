// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ProjectOverviewTab } from "./ProjectOverviewTab";
import type { Project } from "@/types/project";
import type { Instance } from "@/types/rgd";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

function inst(name: string, namespace: string, health: Instance["health"] = "Healthy"): Instance {
  return {
    name,
    namespace,
    rgdName: "rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "App",
    health,
    conditions: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  } as Instance;
}

const project: Project = {
  name: "alpha",
  type: "app",
  destinations: [{ namespace: "alpha-apps" }],
  roles: [{ name: "r1" }],
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
};

function renderTab(instances: Instance[]) {
  return render(
    <MemoryRouter>
      <ProjectOverviewTab
        project={project}
        instances={instances}
        onUpdate={vi.fn().mockResolvedValue(undefined)}
        isUpdating={false}
        canManage={false}
      />
    </MemoryRouter>
  );
}

function issuesValueEl() {
  const card = screen.getByText("Issues").closest(".overflow-hidden") as HTMLElement;
  // The big number is the only 3xl element in the card.
  return card.querySelector(".text-3xl") as HTMLElement;
}

describe("ProjectOverviewTab — StatCards", () => {
  beforeEach(() => mockNavigate.mockReset());

  it("colors the Issues card with the warning token when issues > 0", () => {
    renderTab([inst("a1", "alpha-apps", "Unhealthy")]);
    const value = issuesValueEl();
    expect(value.textContent).toBe("1");
    expect(value).toHaveStyle({ color: "var(--status-warning)" });
  });

  it("does NOT color the Issues card when there are no issues", () => {
    renderTab([inst("a1", "alpha-apps", "Healthy")]);
    const value = issuesValueEl();
    expect(value.textContent).toBe("0");
    expect(value.style.color).toBe("");
  });

  it("lists destination namespaces with their instance counts", () => {
    renderTab([inst("a1", "alpha-apps"), inst("a2", "alpha-apps")]);
    const ns = screen.getByText("alpha-apps").closest("li") as HTMLElement;
    expect(within(ns).getByText("2")).toBeInTheDocument();
  });

  it("cross-links a recent-instance row to the global instance detail route", () => {
    renderTab([inst("a1", "alpha-apps")]);
    const row = screen.getByRole("button", { name: /View details for a1/i });
    fireEvent.click(row);
    expect(mockNavigate).toHaveBeenCalledWith(
      "/instances/example.com/v1/alpha-apps/App/a1"
    );
  });
});
