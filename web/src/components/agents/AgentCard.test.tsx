// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { vi } from "vitest";
import { AgentCard } from "./AgentCard";

describe("AgentCard", () => {
  it("renders an agent with name, namespace, description and AI badge", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
      />
    );

    expect(screen.getByText("alpha-helper")).toBeInTheDocument();
    expect(screen.getByText("alpha-apps")).toBeInTheDocument();
    expect(screen.getByText("Helps with alpha things")).toBeInTheDocument();
    // The hub badge is gone — every card is namespaced now (Story 53.2).
    expect(screen.queryByText("Hub")).not.toBeInTheDocument();
  });

  it("falls back to placeholder copy when the description is empty", () => {
    render(<AgentCard name="quiet-agent" description="" />);

    expect(screen.getByText("No description available")).toBeInTheDocument();
  });

  it("shows the running indicator when running is true (49.4, UX-DR6)", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
        running
      />
    );

    const indicator = screen.getByTestId("agent-running-indicator");
    expect(indicator).toHaveTextContent("Running");
    // The pulsing status dot is the live spinner.
    expect(indicator.querySelector('[data-testid="status-dot"]')).not.toBeNull();
  });

  it("hides the running indicator when running is false/absent", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
      />
    );

    expect(screen.queryByTestId("agent-running-indicator")).not.toBeInTheDocument();
  });

  it("is display-only — no deploy/uninstall action when no `to`/`onEdit` (49.2 AC #1)", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
      />
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("renders as a navigable link with hover affordance when `to` is set (53.2)", () => {
    render(
      <MemoryRouter>
        <AgentCard
          name="rgd-builder"
          namespace="kagent"
          description="Builds RGDs"
          to="/agents/list/kagent/rgd-builder"
        />
      </MemoryRouter>
    );

    const link = screen.getByRole("link", { name: "Open rgd-builder" });
    expect(link).toHaveAttribute("href", "/agents/list/kagent/rgd-builder");
    expect(link).toHaveAttribute("data-testid", "agent-card");
    // The Link variant carries the RGDCard group-hover affordance.
    expect(link.className).toContain("group");
    // Card content still renders inside the link.
    expect(screen.getByText("kagent")).toBeInTheDocument();
  });

  it("stays display-only (no link) when `to` is absent", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
      />
    );

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-card")).toBeInTheDocument();
  });

  it("renders the model badge when a model is provided (50.4)", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
        model={{ provider: "OpenAI", name: "gpt-4.1-mini" }}
      />
    );

    expect(screen.getByTestId("agent-model-badge")).toHaveTextContent("OpenAI · gpt-4.1-mini");
  });

  it("renders no model badge when the model is absent (50.4)", () => {
    render(
      <AgentCard
        name="alpha-helper"
        namespace="alpha-apps"
        description="Helps with alpha things"
      />
    );

    expect(screen.queryByTestId("agent-model-badge")).not.toBeInTheDocument();
  });

  it("renders the edit (change-model) button only when onEdit is set", async () => {
    const onEdit = vi.fn();
    const { rerender } = render(
      <AgentCard name="alpha-helper" namespace="alpha-apps" description="x" />
    );
    expect(screen.queryByTestId("agent-edit-button")).not.toBeInTheDocument();

    rerender(
      <AgentCard name="alpha-helper" namespace="alpha-apps" description="x" onEdit={onEdit} />
    );
    const btn = screen.getByTestId("agent-edit-button");
    expect(btn).toHaveAttribute("aria-label", "Change model for alpha-helper");
    await userEvent.click(btn);
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("edit button on a navigable card opens the editor WITHOUT navigating", async () => {
    const onEdit = vi.fn();
    render(
      <MemoryRouter initialEntries={["/agents/list"]}>
        <Routes>
          <Route
            path="/agents/list"
            element={
              <AgentCard
                name="rgd-builder"
                namespace="kagent"
                description="Builds RGDs"
                to="/agents/list/kagent/rgd-builder"
                onEdit={onEdit}
              />
            }
          />
          <Route path="/agents/list/kagent/rgd-builder" element={<div>CHAT PAGE</div>} />
        </Routes>
      </MemoryRouter>
    );

    await userEvent.click(screen.getByTestId("agent-edit-button"));
    expect(onEdit).toHaveBeenCalledTimes(1);
    // The click was intercepted — the Link did not navigate to the chat page.
    expect(screen.queryByText("CHAT PAGE")).not.toBeInTheDocument();
  });

  it("never resurfaces the removed kagent vendor badge (50.4)", () => {
    render(
      <AgentCard
        name="rgd-builder"
        namespace="kagent"
        description="Builds RGDs"
        model={{ provider: "OpenAI", name: "gpt-4.1-mini" }}
      />
    );

    expect(screen.queryByTestId("agent-ai-badge")).not.toBeInTheDocument();
    expect(screen.queryByText(/Powered by kagent/i)).not.toBeInTheDocument();
  });
});
