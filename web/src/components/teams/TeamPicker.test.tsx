// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { TeamPicker } from "./TeamPicker";
import type { Team } from "@/types/team";

let teams: Team[] = [];

vi.mock("@/hooks/useTeams", () => ({
  useTeams: () => ({ data: { items: teams, totalCount: teams.length }, isLoading: false }),
}));

function renderPicker(selected: string[] = []) {
  const onChange = vi.fn();
  render(
    <MemoryRouter>
      <TeamPicker selected={selected} onChange={onChange} />
    </MemoryRouter>
  );
  return { onChange };
}

beforeEach(() => {
  vi.clearAllMocks();
  teams = [
    { name: "platform-admins", oidcGroups: ["g1"] },
    { name: "alpha-devs", oidcGroups: ["g2"] },
  ];
});

describe("TeamPicker", () => {
  it("binds a team when picked, writing role.teams", async () => {
    const { onChange } = renderPicker([]);
    await userEvent.click(screen.getByRole("combobox"));
    await userEvent.click(screen.getByText("platform-admins"));
    expect(onChange).toHaveBeenCalledWith(["platform-admins"]);
  });

  it("renders bound teams as chips", () => {
    renderPicker(["platform-admins"]);
    const chips = screen.getByTestId("team-picker-chips");
    expect(chips).toHaveTextContent("platform-admins");
  });

  it("shows a hint linking to settings when no teams exist", () => {
    teams = [];
    renderPicker([]);
    const link = screen.getByRole("link", { name: /Create one in Settings/i });
    expect(link).toHaveAttribute("href", "/settings/teams");
  });
});
