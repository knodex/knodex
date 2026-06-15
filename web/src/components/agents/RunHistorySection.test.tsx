// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { RunHistorySection } from "./RunHistorySection";
import { parseContextRef } from "./context-ref";
import * as agentRunsApi from "@/api/agent-runs";
import * as agentsApi from "@/api/agents";
import type { AgentRun, AgentRunsResponse } from "@/api/agent-runs";

vi.mock("@/api/agent-runs");
vi.mock("@/api/agents");

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <RunHistorySection />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

function makeRun(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: "run-1",
    actor: "dev@example.com",
    agentType: "helper",
    agentNamespace: "alpha-apps",
    contextRef: "",
    kagentSessionId: "",
    inputSummary: "do the thing",
    recommendationSummary: "",
    actionTaken: "",
    timestamp: "2026-06-06T10:00:00Z",
    status: "completed",
    triggerType: "on_demand",
    ...overrides,
  };
}

function envelope(items: AgentRun[], total = items.length, page = 1, pageSize = 20): AgentRunsResponse {
  return { items, total, page, pageSize };
}

describe("parseContextRef", () => {
  it("parses an instance ref into the GVK-aware instance route", () => {
    const parsed = parseContextRef("instance:apps.example.com/v1alpha1/alpha-apps/WebApp/my-app");
    expect(parsed).toEqual({
      kind: "instance",
      to: "/instances/apps.example.com/v1alpha1/alpha-apps/WebApp/my-app",
      label: "WebApp/my-app",
    });
  });

  it("parses an rgd ref into the catalog route", () => {
    expect(parseContextRef("rgd:webapp")).toEqual({
      kind: "rgd",
      to: "/catalog/webapp",
      label: "webapp",
    });
  });

  it("falls back to plain text for malformed or unknown refs", () => {
    expect(parseContextRef("instance:too/few/segments").kind).toBe("text");
    expect(parseContextRef("instance:a/b/c/d/e/f").kind).toBe("text");
    expect(parseContextRef("rgd:").kind).toBe("text");
    expect(parseContextRef("rgd:has/slash").kind).toBe("text");
    expect(parseContextRef("ticket JIRA-123").kind).toBe("text");
  });
});

describe("RunHistorySection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [] });
  });

  it("renders rows with all five columns (AC #1)", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope([
        makeRun({
          id: "run-1",
          agentType: "helper",
          agentNamespace: "alpha-apps",
          actor: "dev@example.com",
          contextRef: "rgd:webapp",
          status: "completed",
          timestamp: "2026-06-06T10:00:00Z",
        }),
      ])
    );

    renderSection();

    const row = await screen.findByTestId("run-history-row");
    expect(row).toHaveTextContent("helper"); // Agent
    expect(row).toHaveTextContent("alpha-apps"); // namespace subtext (BYOA)
    expect(row).toHaveTextContent("dev@example.com"); // Triggered by
    expect(row).toHaveTextContent("webapp"); // Context
    expect(screen.getByTestId("run-status-badge")).toHaveAttribute("data-status", "completed");
    // Time renders as a locale string of the timestamp.
    expect(row).toHaveTextContent(new Date("2026-06-06T10:00:00Z").toLocaleString());
  });

  it("links an rgd contextRef to the catalog route and an instance ref to the instance route", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope([
        makeRun({ id: "run-1", contextRef: "rgd:webapp" }),
        makeRun({
          id: "run-2",
          contextRef: "instance:apps.example.com/v1alpha1/alpha-apps/WebApp/my-app",
        }),
        makeRun({ id: "run-3", contextRef: "free text note" }),
      ])
    );

    renderSection();

    const rgdLink = await screen.findByRole("link", { name: "webapp" });
    expect(rgdLink).toHaveAttribute("href", "/catalog/webapp");

    const instanceLink = screen.getByRole("link", { name: "WebApp/my-app" });
    expect(instanceLink).toHaveAttribute(
      "href",
      "/instances/apps.example.com/v1alpha1/alpha-apps/WebApp/my-app"
    );

    // Unparseable refs render as plain text — no link.
    expect(screen.getByText("free text note")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "free text note" })).not.toBeInTheDocument();
  });

  it("status filter calls the API with the status param and resets to page 1", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope(
        Array.from({ length: 20 }, (_, i) => makeRun({ id: `run-${i}` })),
        25
      )
    );

    renderSection();
    await screen.findAllByTestId("run-history-row");

    // Move to page 2 first to prove the filter resets it.
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2 })
      )
    );

    await userEvent.click(screen.getByTestId("run-status-filter"));
    await userEvent.click(await screen.findByRole("option", { name: "Failed" }));

    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: "failed", page: 1 })
      )
    );
  });

  it("agent type filter offers installed agents and calls the API with agentType", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({
      agents: [
        { name: "alpha-helper", namespace: "alpha-apps", description: "", createdAt: "" },
      ],
    });
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope([makeRun({ id: "run-1", agentType: "other-agent" })])
    );

    renderSection();
    await screen.findByTestId("run-history-row");

    await userEvent.click(screen.getByTestId("run-agent-filter"));
    // Options = installed agents ∪ agent types in the current page.
    expect(await screen.findByRole("option", { name: "alpha-helper" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "other-agent" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("option", { name: "alpha-helper" }));
    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ agentType: "alpha-helper", page: 1 })
      )
    );
  });

  it("pagination next/prev fires page param changes", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope(
        Array.from({ length: 20 }, (_, i) => makeRun({ id: `run-${i}` })),
        45
      )
    );

    renderSection();
    await screen.findAllByTestId("run-history-row");
    expect(screen.getByText("1 / 3")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2 })
      )
    );

    // Going back lands on the cached page-1 query (fresh within staleTime —
    // no refetch); the UI must reflect page 1 again.
    await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeInTheDocument());
  });

  it("page size change calls the API with the new pageSize and resets to page 1", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(
      envelope(
        Array.from({ length: 20 }, (_, i) => makeRun({ id: `run-${i}` })),
        45
      )
    );

    renderSection();
    await screen.findAllByTestId("run-history-row");

    // Move to page 2 first to prove the size change resets it.
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2 })
      )
    );

    // The page-size select is the pagination combobox (the two filter
    // selects carry testids; the size select shows the current size).
    const sizeTrigger = screen
      .getAllByRole("combobox")
      .find((el) => !el.getAttribute("data-testid"));
    expect(sizeTrigger).toBeDefined();
    await userEvent.click(sizeTrigger!);
    await userEvent.click(await screen.findByRole("option", { name: "50" }));

    await waitFor(() =>
      expect(agentRunsApi.listAgentRuns).toHaveBeenLastCalledWith(
        expect.objectContaining({ pageSize: 50, page: 1 })
      )
    );
  });

  it("shows the empty state when there are no runs (not an error)", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue(envelope([]));

    renderSection();

    expect(await screen.findByTestId("run-history-empty")).toBeInTheDocument();
    expect(screen.getByText("No agent runs yet")).toBeInTheDocument();
    expect(screen.queryByTestId("run-history-error")).not.toBeInTheDocument();
  });

  it("shows the error state when the API fails", async () => {
    vi.mocked(agentRunsApi.listAgentRuns).mockRejectedValue(new Error("boom"));

    renderSection();

    expect(await screen.findByTestId("run-history-error")).toBeInTheDocument();
  });
});
