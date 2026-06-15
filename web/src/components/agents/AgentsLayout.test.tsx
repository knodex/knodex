// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AgentsLayout } from "./AgentsLayout";
import { getAgentsNavItems, resolveActiveAgentsId } from "./agents-nav";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AgentsLayout>
        <div>panel content</div>
      </AgentsLayout>
    </MemoryRouter>,
  );
}

describe("getAgentsNavItems", () => {
  it("lists exactly Overview, Agents, Templates, Models", () => {
    const items = getAgentsNavItems();
    expect(items.map((i) => i.id)).toEqual(["overview", "agents", "templates", "models"]);
    expect(items.map((i) => i.to)).toEqual([
      "/agents",
      "/agents/list",
      "/agents/templates",
      "/agents/models",
    ]);
  });

  it("marks only Overview as an exact match (so /agents doesn't claim /agents/list)", () => {
    const overview = getAgentsNavItems().find((i) => i.id === "overview");
    expect(overview?.exact).toBe(true);
    expect(overview?.to).toBe("/agents");
  });
});

describe("resolveActiveAgentsId", () => {
  const items = getAgentsNavItems();

  it("resolves Overview only on the exact /agents path", () => {
    expect(resolveActiveAgentsId("/agents", items)).toBe("overview");
  });

  it("resolves /agents/list to the Agents tab, not Overview", () => {
    expect(resolveActiveAgentsId("/agents/list", items)).toBe("agents");
  });

  it("keeps the Agents tab active on a deeper chat route", () => {
    expect(resolveActiveAgentsId("/agents/list/kagent/rgd-builder/chat/abc", items)).toBe("agents");
  });

  it("resolves /agents/models to the Models tab", () => {
    expect(resolveActiveAgentsId("/agents/models", items)).toBe("models");
  });

  it("returns null when nothing matches", () => {
    expect(resolveActiveAgentsId("/instances", items)).toBeNull();
  });
});

describe("AgentsLayout", () => {
  // Navigation moved to the left secondary sidebar (Sidebar.tsx agents sub-nav);
  // AgentsLayout is now a thin content shell, so it must NOT render its own nav.
  it("renders the content panel without an in-content section nav", () => {
    renderAt("/agents");
    expect(screen.getByText("panel content")).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: /agents sections/i })).not.toBeInTheDocument();
  });
});
