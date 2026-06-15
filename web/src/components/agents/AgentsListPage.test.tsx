// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Routes, Route, useLocation } from "react-router-dom";
import { AgentsListPage } from "./AgentsListPage";
import * as agentsApi from "@/api/agents";
import * as agentRunsApi from "@/api/agent-runs";
import * as sessionsApi from "@/api/agent-sessions";
import type { InstalledAgent } from "@/api/agents";

vi.mock("@/api/agents");
vi.mock("@/api/agent-runs");
vi.mock("@/api/agent-sessions");
// The list subscribes to agent_runs over WebSocket; stub it out in tests.
vi.mock("@/hooks/useWebSocket", () => ({ useWebSocket: vi.fn() }));

function renderListPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AgentsListPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

/** Surfaces the active pathname so navigation can be asserted. */
function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

/** Renders the page with the chat route wired so navigation can be observed. */
function renderWithRoutes() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/agents/list"]}>
        <Routes>
          <Route path="/agents/list" element={<AgentsListPage />} />
          <Route
            path="/agents/list/:namespace/:name"
            element={<div>chat page</div>}
          />
        </Routes>
        <LocationProbe />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const agents: InstalledAgent[] = [
  {
    name: "alpha-helper",
    namespace: "alpha-apps",
    description: "Helps with alpha things",
    createdAt: "2026-06-01T10:00:00Z",
  },
  {
    name: "beta-helper",
    namespace: "beta-apps",
    description: "Helps with beta things",
    createdAt: "2026-06-02T10:00:00Z",
  },
];

describe("AgentsListPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to the list view; the toggle persists to localStorage.
    localStorage.clear();
    // No running runs and no past sessions by default.
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      pageSize: 100,
    });
    vi.mocked(sessionsApi.listAgentSessions).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      pageSize: 20,
    });
  });

  it("renders one list row per agent with name and namespace (default list view)", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents });

    renderListPage();

    expect(await screen.findByText("alpha-helper")).toBeInTheDocument();
    expect(screen.getByText("beta-helper")).toBeInTheDocument();
    expect(screen.getByText("alpha-apps")).toBeInTheDocument();
    expect(screen.getByText("beta-apps")).toBeInTheDocument();

    const rows = screen.getAllByTestId("agent-row");
    expect(rows).toHaveLength(2);
    // No hub/installed section headings anymore.
    expect(screen.queryByRole("heading", { name: "Hub Agents" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Installed Agents" })).not.toBeInTheDocument();
  });

  it("does NOT render a Create Agent action", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents });

    renderListPage();

    await screen.findByText("alpha-helper");
    expect(screen.queryByTestId("create-agent-button")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /create agent/i })).not.toBeInTheDocument();
  });

  it("exposes each list row's chat route as a keyboard-accessible name link", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [agents[0]] });

    renderListPage();

    const link = await screen.findByRole("link", { name: /open alpha-helper/i });
    expect(link).toHaveAttribute("href", "/agents/list/alpha-apps/alpha-helper");
  });

  it("navigates when a list row is clicked", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [agents[0]] });

    renderWithRoutes();

    const row = await screen.findByTestId("agent-row");
    await userEvent.click(row);

    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/agents/list/alpha-apps/alpha-helper"
      )
    );
  });

  it("toggles to the grid view (rendering cards) and persists the choice", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [agents[0]] });

    renderListPage();

    await screen.findByText("alpha-helper");
    expect(screen.getByTestId("agent-row")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /grid view/i }));

    expect(await screen.findByTestId("agent-card")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-row")).not.toBeInTheDocument();
    expect(localStorage.getItem("knodex.agents.view")).toBe("grid");
  });

  it("honors a persisted grid view on mount", async () => {
    localStorage.setItem("knodex.agents.view", "grid");
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [agents[0]] });

    renderListPage();

    expect(await screen.findByTestId("agent-card")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-row")).not.toBeInTheDocument();
  });

  it("filters agents by name", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents });

    renderListPage();

    await screen.findByText("alpha-helper");
    await userEvent.type(screen.getByTestId("agents-search"), "beta-helper");

    expect(screen.getByText("beta-helper")).toBeInTheDocument();
    expect(screen.queryByText("alpha-helper")).not.toBeInTheDocument();
  });

  it("filters agents by namespace", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents });

    renderListPage();

    await screen.findByText("alpha-helper");
    await userEvent.type(screen.getByTestId("agents-search"), "beta-apps");

    expect(screen.getByText("beta-helper")).toBeInTheDocument();
    expect(screen.queryByText("alpha-helper")).not.toBeInTheDocument();
  });

  it("shows a search empty-state with a clear action when nothing matches", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents });

    renderListPage();

    await screen.findByText("alpha-helper");
    await userEvent.type(screen.getByTestId("agents-search"), "zzz-nomatch");

    expect(await screen.findByText("No matching agents")).toBeInTheDocument();
    // Text query targets the empty-state action (the icon-only input clear
    // button carries the label via aria, with no text node).
    await userEvent.click(screen.getByText("Clear filters"));
    expect(await screen.findByText("alpha-helper")).toBeInTheDocument();
  });

  it("shows an empty-state that deploys from a template (Catalog as fallback) when there are no agents", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [] });

    renderListPage();

    expect(await screen.findByText("No agents available")).toBeInTheDocument();
    // Primary CTA points at the in-section Templates tab (the agent-deploy surface).
    const templatesLink = screen.getByRole("link", { name: /deploy from a template/i });
    expect(templatesLink).toHaveAttribute("href", "/agents/templates");
    // The global Catalog stays available as a secondary fallback.
    const catalogLink = screen.getByRole("link", { name: /browse the full catalog/i });
    expect(catalogLink).toHaveAttribute("href", "/catalog");
  });

  it("shows a retryable error when the fetch fails, and retry refetches", async () => {
    vi.mocked(agentsApi.listAgents).mockRejectedValueOnce(new Error("network down"));

    renderListPage();

    expect(await screen.findByTestId("agents-list-error")).toBeInTheDocument();
    expect(vi.mocked(agentsApi.listAgents)).toHaveBeenCalledTimes(1);

    vi.mocked(agentsApi.listAgents).mockResolvedValueOnce({ agents });
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => expect(vi.mocked(agentsApi.listAgents)).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("alpha-helper")).toBeInTheDocument();
  });

  it("renders the resolved model badge on a list row", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({
      agents: [{ ...agents[0], model: { provider: "Anthropic", name: "claude-sonnet-4" } }],
    });

    renderListPage();

    expect(await screen.findByText("alpha-helper")).toBeInTheDocument();
    expect(screen.getByText("Anthropic · claude-sonnet-4")).toBeInTheDocument();
  });

  it("flags an agent with a running run via the live in-flight indicator", async () => {
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [agents[0]] });
    vi.mocked(agentRunsApi.listAgentRuns).mockResolvedValue({
      items: [
        {
          id: "run-1",
          actor: "dev@example.com",
          agentType: "alpha-helper",
          agentNamespace: "alpha-apps",
          contextRef: "",
          kagentSessionId: "",
          inputSummary: "",
          recommendationSummary: "",
          actionTaken: "",
          timestamp: "2026-06-06T10:00:00Z",
          status: "running",
          triggerType: "on_demand",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    renderListPage();

    expect(await screen.findByTestId("agent-running-indicator")).toBeInTheDocument();
  });
});
