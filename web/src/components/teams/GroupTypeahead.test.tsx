// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GroupTypeahead } from "./GroupTypeahead";
import type { ObservedGroup } from "@/types/team";

const suggestions: ObservedGroup[] = [
  { name: "alpha-devs", lastSeen: "2026-05-26T12:00:00Z" },
  { name: "alpha-ops", lastSeen: "2026-05-26T11:00:00Z" },
  { name: "beta-team", lastSeen: "2026-05-25T10:00:00Z" },
];

function setup(selected: string[] = []) {
  const onChange = vi.fn();
  render(
    <GroupTypeahead
      selected={selected}
      onChange={onChange}
      suggestions={suggestions}
      canEdit
    />
  );
  return { onChange };
}

describe("GroupTypeahead", () => {
  it("filters suggestions by typed text", async () => {
    setup();
    const input = screen.getByTestId("group-input");
    fireEvent.change(input, { target: { value: "alpha" } });

    expect(screen.getByTestId("group-suggestion-alpha-devs")).toBeInTheDocument();
    expect(screen.getByTestId("group-suggestion-alpha-ops")).toBeInTheDocument();
    expect(
      screen.queryByTestId("group-suggestion-beta-team")
    ).not.toBeInTheDocument();
  });

  it("orders suggestions most-recently-seen first", () => {
    setup();
    fireEvent.focus(screen.getByTestId("group-input"));
    const rendered = screen
      .getByTestId("group-suggestions")
      .querySelectorAll('[data-testid^="group-suggestion-"]');
    expect(rendered[0].getAttribute("data-testid")).toBe(
      "group-suggestion-alpha-devs"
    );
    expect(rendered[1].getAttribute("data-testid")).toBe(
      "group-suggestion-alpha-ops"
    );
    expect(rendered[2].getAttribute("data-testid")).toBe(
      "group-suggestion-beta-team"
    );
  });

  it("adds a chip when a suggestion is clicked", async () => {
    const { onChange } = setup();
    fireEvent.focus(screen.getByTestId("group-input"));
    await userEvent.click(screen.getByTestId("group-suggestion-alpha-devs"));
    expect(onChange).toHaveBeenCalledWith(["alpha-devs"]);
  });

  it("commits a non-suggested typed value (free-text fallback)", async () => {
    const { onChange } = setup();
    const input = screen.getByTestId("group-input");
    fireEvent.change(input, { target: { value: "custom-group-not-observed" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith(["custom-group-not-observed"]);
  });

  it("removes a chip", async () => {
    const { onChange } = setup(["alpha-devs"]);
    await userEvent.click(screen.getByTestId("remove-group-alpha-devs"));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("still accepts raw entry with empty suggestions", () => {
    const onChange = vi.fn();
    render(
      <GroupTypeahead
        selected={[]}
        onChange={onChange}
        suggestions={[]}
        canEdit
      />
    );
    const input = screen.getByTestId("group-input");
    fireEvent.change(input, { target: { value: "lonely-group" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith(["lonely-group"]);
    // No suggestions dropdown when the store is empty.
    expect(screen.queryByTestId("group-suggestions")).not.toBeInTheDocument();
  });

  it("does not suggest already-selected groups", () => {
    setup(["alpha-devs"]);
    fireEvent.focus(screen.getByTestId("group-input"));
    expect(
      screen.queryByTestId("group-suggestion-alpha-devs")
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("group-suggestion-alpha-ops")).toBeInTheDocument();
  });

  it("rejects an empty commit without calling onChange", () => {
    const { onChange } = setup();
    const input = screen.getByTestId("group-input");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).not.toHaveBeenCalled();
  });
});
