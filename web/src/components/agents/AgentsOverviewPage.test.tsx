// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { AgentsOverviewPage } from "./AgentsOverviewPage";
import * as agentsApi from "@/api/agents";
import type { AgentsStatusResponse } from "@/api/agents";

vi.mock("@/api/agents");

function renderOverview() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AgentsOverviewPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const readyStatus: AgentsStatusResponse = {
  status: "ready",
  crdPresent: true,
  controllerHealthy: true,
  message: "kagent is installed and healthy",
};

const notInstalledStatus: AgentsStatusResponse = {
  status: "not_installed",
  crdPresent: false,
  controllerHealthy: null,
  message: "kagent Agent CRD (agents.kagent.dev) not found in cluster",
};

const degradedStatus: AgentsStatusResponse = {
  status: "degraded",
  crdPresent: null,
  controllerHealthy: null,
  message: "kagent CRD discovery failed: connection reset",
};

describe("AgentsOverviewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default the ready-state count queries to small payloads; individual
    // tests override to exercise loading/error.
    vi.mocked(agentsApi.listAgents).mockResolvedValue({
      agents: [
        { name: "a1", namespace: "alpha-apps", description: "", createdAt: "" },
        { name: "a2", namespace: "beta-apps", description: "", createdAt: "" },
      ],
    });
    vi.mocked(agentsApi.listModels).mockResolvedValue({
      models: [
        { name: "m1", namespace: "alpha-apps", provider: "openai", model: "gpt-4o" },
      ],
    });
    vi.mocked(agentsApi.listAgentTemplates).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 20,
    });
  });

  it("shows loading skeleton while the status query is pending", () => {
    vi.mocked(agentsApi.getAgentsStatus).mockReturnValue(new Promise(() => {}));

    renderOverview();

    expect(screen.getByTestId("agents-loading")).toBeInTheDocument();
  });

  it("shows onboarding state with install instructions when not_installed", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue(notInstalledStatus);

    renderOverview();

    expect(await screen.findByTestId("agents-onboarding")).toBeInTheDocument();
    expect(screen.getByText("AI Agents not yet available")).toBeInTheDocument();
    expect(screen.getByText(/helm install kagent-crds/)).toBeInTheDocument();
    const docsLink = screen.getByRole("link", { name: /view kagent docs/i });
    expect(docsLink).toHaveAttribute("href", "https://kagent.dev/docs");
    expect(screen.getByTestId("agents-check-details")).toHaveTextContent(
      "Agent CRD (agents.kagent.dev): not found"
    );
    expect(screen.queryByTestId("agents-degraded")).not.toBeInTheDocument();
  });

  it("surfaces a half-installed kagent (CRD found, controller down) in onboarding", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue({
      status: "not_installed",
      crdPresent: true,
      controllerHealthy: false,
      message: "kagent controller responded unhealthy to /health",
    });

    renderOverview();

    const details = await screen.findByTestId("agents-check-details");
    expect(details).toHaveTextContent("Agent CRD (agents.kagent.dev): found");
    expect(details).toHaveTextContent("kagent controller: not responding");
  });

  it("shows degraded state with retry button for degraded status", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue(degradedStatus);

    renderOverview();

    expect(await screen.findByTestId("agents-degraded")).toBeInTheDocument();
    expect(screen.getByText(degradedStatus.message)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("shows degraded state when the fetch itself rejects, and retry refetches", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockRejectedValueOnce(new Error("network down"));

    renderOverview();

    expect(await screen.findByTestId("agents-degraded")).toBeInTheDocument();
    expect(vi.mocked(agentsApi.getAgentsStatus)).toHaveBeenCalledTimes(1);

    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValueOnce(readyStatus);
    await userEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() =>
      expect(vi.mocked(agentsApi.getAgentsStatus)).toHaveBeenCalledTimes(2)
    );
    expect(await screen.findByTestId("agents-overview-ready")).toBeInTheDocument();
  });

  it("renders agent + model counts and both quickstart links in the ready state", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue(readyStatus);

    renderOverview();

    const ready = await screen.findByTestId("agents-overview-ready");

    // Counts come from the list lengths (2 agents, 1 model in beforeEach).
    await waitFor(() =>
      expect(screen.getByTestId("agents-overview-agent-count")).toHaveTextContent("2")
    );
    expect(screen.getByTestId("agents-overview-model-count")).toHaveTextContent("1");

    // Templates count tile renders alongside agents + models (3-up grid).
    expect(screen.getByTestId("agents-overview-template-count")).toHaveTextContent("0");

    // Stat cards are whole-card links into their workspace tab.
    expect(screen.getByTestId("agents-overview-agent-count")).toHaveAttribute("href", "/agents/list");
    expect(screen.getByTestId("agents-overview-model-count")).toHaveAttribute("href", "/agents/models");
    expect(screen.getByTestId("agents-overview-template-count")).toHaveAttribute(
      "href",
      "/agents/templates"
    );

    // Model link is sequenced before the Agent link (first-run dependency).
    const modelLink = within(ready).getByTestId("agents-overview-quickstart-model");
    const agentLink = within(ready).getByTestId("agents-overview-quickstart-agent");
    expect(modelLink).toHaveAttribute("href", "/agents/models");
    // Agents are deployed FROM templates, so the quickstart points at Templates.
    expect(agentLink).toHaveAttribute("href", "/agents/templates");

    // Both blocks exist (2 agents, 1 model) → quickstart is complete and yields
    // the forward CTA into the Agents tab.
    expect(screen.getByText("You're all set")).toBeInTheDocument();
    expect(screen.getByTestId("agents-overview-start")).toHaveAttribute("href", "/agents/list");
  });

  it("shows an incomplete quickstart (no CTA) on a first-run empty workspace", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue(readyStatus);
    vi.mocked(agentsApi.listAgents).mockResolvedValue({ agents: [] });
    vi.mocked(agentsApi.listModels).mockResolvedValue({ models: [] });

    renderOverview();

    await screen.findByTestId("agents-overview-ready");

    await waitFor(() =>
      expect(screen.getByTestId("agents-overview-agent-count")).toHaveTextContent("0")
    );
    // Nothing built yet → onboarding framing, no progress, no "all set" CTA.
    expect(screen.getByText("Quick start")).toBeInTheDocument();
    expect(screen.getByText("0 of 2 complete")).toBeInTheDocument();
    expect(screen.queryByTestId("agents-overview-start")).not.toBeInTheDocument();
  });

  it("degrades a count tile to a dash on a count-query error without rendering degraded", async () => {
    vi.mocked(agentsApi.getAgentsStatus).mockResolvedValue(readyStatus);
    vi.mocked(agentsApi.listModels).mockRejectedValue(new Error("models down"));

    renderOverview();

    await screen.findByTestId("agents-overview-ready");

    // The models tile degrades to a dash …
    await waitFor(() =>
      expect(screen.getByTestId("agents-overview-model-count")).toHaveTextContent("—")
    );
    // … while the agents tile still renders its count, and the presence card
    // is NOT the degraded state (a count error is not a presence failure).
    expect(screen.getByTestId("agents-overview-agent-count")).toHaveTextContent("2");
    expect(screen.queryByTestId("agents-degraded")).not.toBeInTheDocument();
  });
});
