// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ProjectInstancesTab } from "./ProjectInstancesTab";
import type { Project } from "@/types/project";
import type { Instance } from "@/types/rgd";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

function inst(name: string, namespace: string): Instance {
  return {
    name,
    namespace,
    rgdName: "rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "App",
    health: "Healthy",
    conditions: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  } as Instance;
}

const project: Project = {
  name: "alpha",
  type: "app",
  destinations: [{ namespace: "alpha-apps" }],
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
};

function renderTab(instances: Instance[]) {
  return render(
    <MemoryRouter>
      <ProjectInstancesTab project={project} instances={instances} />
    </MemoryRouter>
  );
}

describe("ProjectInstancesTab", () => {
  beforeEach(() => mockNavigate.mockReset());

  it("renders the empty state when no instances match the project", () => {
    renderTab([inst("other", "unrelated-ns")]);
    expect(screen.getByText(/No instances yet/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /View details/i })).not.toBeInTheDocument();
  });

  it("renders the scoped, health-striped instances table when populated", () => {
    renderTab([inst("a1", "alpha-apps"), inst("ignored", "other-ns")]);
    expect(screen.getByText("a1")).toBeInTheDocument();
    expect(screen.queryByText("ignored")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("instance-row-health-stripe")).toHaveLength(1);
  });

  it("cross-links a row to the global instance detail route", () => {
    renderTab([inst("a1", "alpha-apps")]);
    fireEvent.click(screen.getByRole("button", { name: /View details for a1/i }));
    expect(mockNavigate).toHaveBeenCalledWith(
      "/instances/example.com/v1/alpha-apps/App/a1"
    );
  });
});
