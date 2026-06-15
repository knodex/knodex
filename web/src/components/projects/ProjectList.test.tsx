// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { ProjectList } from "./ProjectList";
import type { Project } from "@/types/project";
import type { Instance } from "@/types/rgd";

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

const projects: Project[] = [
  {
    name: "alpha",
    type: "app",
    description: "Alpha project",
    destinations: [{ namespace: "alpha-apps" }],
    roles: [{ name: "r1" }, { name: "r2" }],
    resourceVersion: "1",
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "beta",
    type: "app",
    destinations: [{ namespace: "beta-apps" }],
    roles: [{ name: "r1" }],
    resourceVersion: "1",
    createdAt: "2026-01-01T00:00:00Z",
  },
];

const instances: Instance[] = [
  inst("a1", "alpha-apps"),
  inst("a2", "alpha-apps", "Unhealthy"),
  inst("b1", "beta-apps"),
];

describe("ProjectList — card grid + footer", () => {
  it("renders a card per project with the role-count footer affordance", () => {
    render(<ProjectList projects={projects} instances={instances} />);
    const cards = screen.getAllByTestId("project-card");
    expect(cards).toHaveLength(2);
    // E2E relies on a `\d+ role` affordance in the card footer.
    const alpha = cards.find((c) => within(c).queryByText("alpha"))!;
    expect(alpha.textContent).toMatch(/\d+\s+roles?/i);
  });

  it("computes footer counts over ALL visible projects", () => {
    render(<ProjectList projects={projects} instances={instances} />);
    const footer = screen.getByTestId("projects-list-footer");
    // 2 projects · 3 instances (a1,a2,b1) · 3 roles (2 + 1)
    expect(footer.textContent).toMatch(/2\s+projects/);
    expect(footer.textContent).toMatch(/3\s+instances/);
    expect(footer.textContent).toMatch(/3\s+roles/);
  });

  it("recomputes footer counts over the FILTERED set, not the raw list", () => {
    render(<ProjectList projects={projects} instances={instances} />);
    fireEvent.change(screen.getByLabelText("Search projects"), {
      target: { value: "alpha" },
    });
    expect(screen.getAllByTestId("project-card")).toHaveLength(1);
    const footer = screen.getByTestId("projects-list-footer");
    // Only alpha visible: 1 project · 2 instances · 2 roles
    expect(footer.textContent).toMatch(/1\s+project\b/);
    expect(footer.textContent).toMatch(/2\s+instances/);
    expect(footer.textContent).toMatch(/2\s+roles/);
  });

  it("shows the no-match empty state when search matches nothing", () => {
    render(<ProjectList projects={projects} instances={instances} />);
    fireEvent.change(screen.getByLabelText("Search projects"), {
      target: { value: "zzz" },
    });
    expect(screen.queryByTestId("project-card")).not.toBeInTheDocument();
    expect(screen.getByText(/No projects match/i)).toBeInTheDocument();
  });

  it("invokes onClick when a card is activated", () => {
    const onClick = vi.fn();
    render(
      <ProjectList projects={projects} instances={instances} onClick={onClick} />
    );
    fireEvent.click(screen.getAllByTestId("project-card")[0]);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
