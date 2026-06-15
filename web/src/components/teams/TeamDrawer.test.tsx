// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamDrawer } from "./TeamDrawer";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe("TeamDrawer — create mode", () => {
  it("renders empty form with 'New team' title", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={vi.fn()}
        isSubmitting={false}
      />
    );
    expect(screen.getByText("New team")).toBeInTheDocument();
    expect(screen.getByTestId("team-name-input")).toHaveValue("");
    expect(screen.getByTestId("team-description-input")).toHaveValue("");
    // Name input is enabled in create mode
    expect(screen.getByTestId("team-name-input")).not.toBeDisabled();
    // GroupTypeahead rendered (not readOnly mode)
    expect(screen.getByTestId("group-typeahead")).toBeInTheDocument();
  });

  it("calls onSubmit with form values when valid create is submitted", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={onSubmit}
        isSubmitting={false}
      />
    );

    fireEvent.change(screen.getByTestId("team-name-input"), {
      target: { value: "platform-admins" },
    });
    // Add a group via free-text + Enter
    fireEvent.change(screen.getByTestId("group-input"), {
      target: { value: "platform-grp" },
    });
    fireEvent.keyDown(screen.getByTestId("group-input"), { key: "Enter" });

    await userEvent.click(screen.getByTestId("team-save-button"));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        name: "platform-admins",
        description: "",
        oidcGroups: ["platform-grp"],
      })
    );
  });

  it("shows validation error when name is empty on create", async () => {
    const onSubmit = vi.fn();
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={onSubmit}
        isSubmitting={false}
      />
    );
    // Add a group so group validation passes, then submit with empty name
    fireEvent.change(screen.getByTestId("group-input"), {
      target: { value: "some-group" },
    });
    fireEvent.keyDown(screen.getByTestId("group-input"), { key: "Enter" });

    await userEvent.click(screen.getByTestId("team-save-button"));

    expect(onSubmit).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByText(/Name is required/i)).toBeInTheDocument()
    );
  });

  it("shows validation error when no groups are added on create", async () => {
    const onSubmit = vi.fn();
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={onSubmit}
        isSubmitting={false}
      />
    );
    fireEvent.change(screen.getByTestId("team-name-input"), {
      target: { value: "my-team" },
    });
    await userEvent.click(screen.getByTestId("team-save-button"));

    expect(onSubmit).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(
        screen.getByText(/At least one OIDC group is required/i)
      ).toBeInTheDocument()
    );
  });
});

describe("TeamDrawer — edit mode", () => {
  it("renders prefilled values with 'Edit team' title and immutable name", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="edit"
        initialValues={{
          name: "alpha",
          description: "Alpha team",
          oidcGroups: ["alpha-devs"],
        }}
        onSubmit={vi.fn()}
        isSubmitting={false}
      />
    );
    expect(screen.getByText("Edit team")).toBeInTheDocument();
    expect(screen.getByTestId("team-name-input")).toHaveValue("alpha");
    // Name is immutable in edit mode
    expect(screen.getByTestId("team-name-input")).toBeDisabled();
    expect(screen.getByTestId("team-description-input")).toHaveValue(
      "Alpha team"
    );
    expect(
      screen.getByText(/Team name is immutable/i)
    ).toBeInTheDocument();
  });

  it("calls onSubmit with updated description in edit mode", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="edit"
        initialValues={{
          name: "alpha",
          description: "",
          oidcGroups: ["alpha-devs"],
        }}
        onSubmit={onSubmit}
        isSubmitting={false}
      />
    );

    fireEvent.change(screen.getByTestId("team-description-input"), {
      target: { value: "Updated description" },
    });
    await userEvent.click(screen.getByTestId("team-save-button"));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        name: "alpha",
        description: "Updated description",
        oidcGroups: ["alpha-devs"],
      })
    );
  });
});

describe("TeamDrawer — readOnlyGroups mode", () => {
  it("hides GroupTypeahead and shows read-only chips when readOnlyGroups=true", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="edit"
        initialValues={{
          name: "alpha",
          description: "",
          oidcGroups: ["alpha-devs", "alpha-ops"],
        }}
        onSubmit={vi.fn()}
        isSubmitting={false}
        readOnlyGroups={true}
      />
    );
    // GroupTypeahead and its input must not be present
    expect(screen.queryByTestId("group-typeahead")).not.toBeInTheDocument();
    expect(screen.queryByTestId("group-input")).not.toBeInTheDocument();
    // Read-only container present
    expect(screen.getByTestId("readonly-groups")).toBeInTheDocument();
    // Each group renders as a chip
    expect(screen.getByTestId("group-chip-alpha-devs")).toBeInTheDocument();
    expect(screen.getByTestId("group-chip-alpha-ops")).toBeInTheDocument();
  });

  it("shows 'No groups assigned' when readOnlyGroups=true and oidcGroups is empty", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="edit"
        initialValues={{ name: "alpha", description: "", oidcGroups: [] }}
        onSubmit={vi.fn()}
        isSubmitting={false}
        readOnlyGroups={true}
      />
    );
    expect(screen.getByText(/No groups assigned/i)).toBeInTheDocument();
  });
});

describe("TeamDrawer — slots", () => {
  it("renders headerSlot instead of default title", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={vi.fn()}
        isSubmitting={false}
        headerSlot={<h2 data-testid="custom-header">Cloud Teams</h2>}
      />
    );
    expect(screen.getByTestId("custom-header")).toBeInTheDocument();
    expect(screen.queryByText("New team")).not.toBeInTheDocument();
  });

  it("renders children after the groups section", () => {
    render(
      <TeamDrawer
        open={true}
        onOpenChange={vi.fn()}
        mode="create"
        onSubmit={vi.fn()}
        isSubmitting={false}
      >
        <div data-testid="roster-panel">Team roster</div>
      </TeamDrawer>
    );
    expect(screen.getByTestId("roster-panel")).toBeInTheDocument();
  });
});
