// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AxiosError, AxiosHeaders } from "axios";
import { TeamsSettings } from "./TeamsSettings";
import type { Team } from "@/types/team";

const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
let teams: Team[] = [];
let canUpdate: boolean | undefined = true;
let teamsError: unknown = null;

function axiosErr(status: number, body: unknown): AxiosError {
  const err = new AxiosError(
    `Request failed with status code ${status}`,
    String(status),
    undefined,
    null,
    {
      data: body,
      status,
      statusText: status === 403 ? "Forbidden" : "Internal Server Error",
      headers: {},
      config: { headers: new AxiosHeaders() },
    },
  );
  return err;
}

vi.mock("@/hooks/useTeams", () => ({
  useTeams: () => ({
    data: { items: teams, totalCount: teams.length },
    isLoading: false,
    error: teamsError,
  }),
  useObservedGroups: () => ({ data: { groups: [] } }),
  useCreateTeam: () => ({ mutateAsync: mockCreate, isPending: false }),
  useUpdateTeam: () => ({ mutateAsync: mockUpdate, isPending: false }),
  useDeleteTeam: () => ({ mutateAsync: mockDelete, isPending: false }),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: () => ({ allowed: canUpdate, isLoading: false, isError: false }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

beforeEach(() => {
  vi.clearAllMocks();
  teams = [];
  canUpdate = true;
  teamsError = null;
});

describe("TeamsSettings", () => {
  it("renders the team list from useTeams", () => {
    teams = [{ name: "alpha", description: "Alpha team", oidcGroups: ["alpha-devs"] }];
    render(<TeamsSettings />);
    expect(screen.getByTestId("team-card-alpha")).toBeInTheDocument();
    expect(screen.getByText("Alpha team")).toBeInTheDocument();
    // Group count is rendered inside the "OIDC groups · N" section header
    // instead of a sidebar badge — assert via the dedicated testid so the
    // markup can keep evolving without churning this test.
    expect(screen.getByTestId("team-group-count-alpha")).toHaveTextContent("1");
  });

  it("shows an empty state when there are no teams", () => {
    teams = [];
    render(<TeamsSettings />);
    expect(screen.getByText(/No teams yet/i)).toBeInTheDocument();
  });

  it("calls createTeam when the create form is submitted", async () => {
    mockCreate.mockResolvedValue({});
    render(<TeamsSettings />);
    await userEvent.click(screen.getByTestId("create-team-button"));

    fireEvent.change(screen.getByTestId("team-name-input"), {
      target: { value: "platform-admins" },
    });
    // Add a group via the typeahead free-text fallback.
    fireEvent.change(screen.getByTestId("group-input"), {
      target: { value: "platform-admins-grp" },
    });
    fireEvent.keyDown(screen.getByTestId("group-input"), { key: "Enter" });

    await userEvent.click(screen.getByTestId("team-save-button"));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        name: "platform-admins",
        description: undefined,
        oidcGroups: ["platform-admins-grp"],
      })
    );
  });

  it("blocks create with no groups (validation error)", async () => {
    render(<TeamsSettings />);
    await userEvent.click(screen.getByTestId("create-team-button"));
    fireEvent.change(screen.getByTestId("team-name-input"), {
      target: { value: "nogroups" },
    });
    await userEvent.click(screen.getByTestId("team-save-button"));
    expect(mockCreate).not.toHaveBeenCalled();
    expect(screen.getByText(/At least one OIDC group is required/i)).toBeInTheDocument();
  });

  it("calls updateTeam from the edit form", async () => {
    mockUpdate.mockResolvedValue({});
    teams = [{ name: "alpha", description: "", oidcGroups: ["alpha-devs"] }];
    render(<TeamsSettings />);
    await userEvent.click(screen.getByTestId("edit-team-alpha"));

    fireEvent.change(screen.getByTestId("team-description-input"), {
      target: { value: "updated desc" },
    });
    await userEvent.click(screen.getByTestId("team-save-button"));

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith({
        name: "alpha",
        request: { description: "updated desc", oidcGroups: ["alpha-devs"] },
      })
    );
  });

  it("calls deleteTeam after confirming", async () => {
    mockDelete.mockResolvedValue(undefined);
    teams = [{ name: "alpha", oidcGroups: ["alpha-devs"] }];
    render(<TeamsSettings />);
    await userEvent.click(screen.getByTestId("delete-team-alpha"));
    await userEvent.click(screen.getByTestId("confirm-delete-team"));
    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith("alpha"));
  });

  it("disables mutation controls for a non-operator", () => {
    canUpdate = false;
    teams = [{ name: "alpha", oidcGroups: ["alpha-devs"] }];
    render(<TeamsSettings />);
    expect(screen.getByTestId("create-team-button")).toBeDisabled();
    expect(screen.getByTestId("edit-team-alpha")).toBeDisabled();
    expect(screen.getByTestId("delete-team-alpha")).toBeDisabled();
  });

  describe("load-error copy", () => {
    it("uses the permission-denied message on 403", () => {
      teamsError = axiosErr(403, { message: "forbidden" });
      render(<TeamsSettings />);
      const banner = screen.getByTestId("teams-error");
      expect(banner).toHaveTextContent(/don't have/i);
      expect(banner).toHaveTextContent(/settings:get/);
      expect(banner).not.toHaveTextContent(/Team CRD/);
    });

    it("uses the permission-denied message on 401", () => {
      teamsError = axiosErr(401, { message: "unauthorized" });
      render(<TeamsSettings />);
      const banner = screen.getByTestId("teams-error");
      expect(banner).toHaveTextContent(/don't have/i);
    });

    it("surfaces server message + CRD hint on 500", () => {
      teamsError = axiosErr(500, { message: "team CRD not installed" });
      render(<TeamsSettings />);
      const banner = screen.getByTestId("teams-error");
      expect(banner).toHaveTextContent(/HTTP 500/);
      expect(banner).toHaveTextContent(/team CRD not installed/);
      expect(banner).toHaveTextContent(/deploy\/crds\/team\.yaml/);
      expect(banner).not.toHaveTextContent(/don't have/i);
    });

    it("falls back to Error.message for non-Axios failures", () => {
      teamsError = new Error("network down");
      render(<TeamsSettings />);
      const banner = screen.getByTestId("teams-error");
      expect(banner).toHaveTextContent(/network down/);
      // Non-Axios → no status code in the message
      expect(banner).not.toHaveTextContent(/HTTP \d/);
    });
  });
});
