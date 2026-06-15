// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ProjectAccessTab } from "./ProjectAccessTab";
import type { Project } from "@/types/project";

vi.mock("@/hooks/useTeams", () => ({
  useTeams: vi.fn(() => ({
    data: {
      items: [
        { name: "platform-eng", oidcGroups: ["g1", "g2"] },
        { name: "payments", oidcGroups: ["g3"] },
        { name: "identity", oidcGroups: ["g4", "g5"] },
      ],
      totalCount: 3,
    },
  })),
}));

const project: Project = {
  name: "payments",
  type: "app",
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
  destinations: [{ namespace: "payments-system" }, { namespace: "payments-dev" }],
  roles: [
    { name: "admin", policies: ["x"], teams: ["platform-eng"], destinations: [] },
    {
      name: "developer",
      policies: ["y"],
      teams: ["payments"],
      destinations: ["payments-system"],
    },
  ],
};

function renderTab(overrides?: Partial<Parameters<typeof ProjectAccessTab>[0]>) {
  const onUpdate = vi.fn().mockResolvedValue(undefined);
  render(
    <MemoryRouter>
      <ProjectAccessTab
        project={project}
        onUpdate={onUpdate}
        isUpdating={false}
        canManage={true}
        {...overrides}
      />
    </MemoryRouter>
  );
  return { onUpdate };
}

describe("ProjectAccessTab", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders one row per bound team with OIDC-group provenance", () => {
    renderTab();
    expect(screen.getByText("platform-eng")).toBeInTheDocument();
    expect(screen.getByText("payments")).toBeInTheDocument();
    // platform-eng has 2 oidc groups, payments has 1
    expect(screen.getByText("via 2 OIDC groups")).toBeInTheDocument();
    expect(screen.getByText("via 1 OIDC group")).toBeInTheDocument();
  });

  it("shows each role's namespace scope (all vs specific)", () => {
    renderTab();
    expect(screen.getByText("All namespaces")).toBeInTheDocument(); // admin role, no destinations
    expect(screen.getByText("payments-system")).toBeInTheDocument(); // developer role
  });

  it("renders the team's role as a pill and the Add team button when manageable", () => {
    renderTab();
    expect(screen.getByRole("button", { name: /Add team/i })).toBeInTheDocument();
    // role pills
    expect(screen.getByRole("button", { name: /^admin/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^developer/i })).toBeInTheDocument();
  });

  it("hides management controls when canManage is false", () => {
    renderTab({ canManage: false });
    expect(screen.queryByRole("button", { name: /Add team/i })).not.toBeInTheDocument();
    // role shown as static text, not a button
    expect(screen.queryByRole("button", { name: /^admin/i })).not.toBeInTheDocument();
    expect(screen.getByText("admin")).toBeInTheDocument();
  });

  it("exposes an overflow menu per team for management", () => {
    renderTab();
    // Radix menu open/close is unreliable under jsdom; the removal transform
    // itself is covered by lib/team-access.test.ts. Here we just assert the
    // per-team management affordance exists.
    expect(
      screen.getByRole("button", { name: /Manage access for platform-eng/i })
    ).toBeInTheDocument();
  });

  it("shows an empty state with Add team when no teams are bound", () => {
    const empty: Project = { ...project, roles: [{ name: "admin", policies: ["x"], teams: [] }] };
    render(
      <MemoryRouter>
        <ProjectAccessTab project={empty} onUpdate={vi.fn()} isUpdating={false} canManage={true} />
      </MemoryRouter>
    );
    expect(screen.getByText("No teams have access")).toBeInTheDocument();
    expect(within(screen.getByText("No teams have access").closest("div")!.parentElement!).getByRole("button", { name: /Add team/i })).toBeInTheDocument();
  });
});
