// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { SessionsListPage } from "./SessionsListPage";
import * as sessionsApi from "@/api/agent-sessions";
import type { AgentSessionSummary, AgentSessionsResponse } from "@/api/agent-sessions";

vi.mock("@/api/agent-sessions");

function makeSession(overrides: Partial<AgentSessionSummary> = {}): AgentSessionSummary {
  return {
    id: "conv-1",
    agentType: "rgd-builder",
    agentNamespace: "kagent",
    firstPrompt: "build me a web app",
    startedAt: "2026-06-06T10:00:00Z",
    lastActivityAt: "2026-06-06T10:05:00Z",
    runCount: 2,
    status: "completed",
    ...overrides,
  };
}

function envelope(items: AgentSessionSummary[]): AgentSessionsResponse {
  return { items, total: items.length, page: 1, pageSize: 20 };
}

function renderPage(initialEntry = "/agents/sessions") {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/agents/sessions" element={<SessionsListPage />} />
          <Route path="/agents/list/:namespace/:name/chat/:sessionId" element={<div>chat: location</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("SessionsListPage", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders a row per session with first prompt, agent, turns and status", async () => {
    vi.mocked(sessionsApi.listAgentSessions).mockResolvedValue(
      envelope([makeSession(), makeSession({ id: "conv-2", firstPrompt: "another", runCount: 1 })])
    );

    renderPage();

    await waitFor(() => expect(screen.getAllByTestId("session-row")).toHaveLength(2));
    expect(screen.getByText("build me a web app")).toBeInTheDocument();
    expect(screen.getByText("another")).toBeInTheDocument();
  });

  it("passes the agentType query param through to the API", async () => {
    vi.mocked(sessionsApi.listAgentSessions).mockResolvedValue(envelope([makeSession()]));

    renderPage("/agents/sessions?agentType=rgd-builder");

    await waitFor(() =>
      expect(sessionsApi.listAgentSessions).toHaveBeenCalledWith(
        expect.objectContaining({ agentType: "rgd-builder" })
      )
    );
  });

  it("shows the empty state when there are no sessions", async () => {
    vi.mocked(sessionsApi.listAgentSessions).mockResolvedValue(envelope([]));

    renderPage();

    await waitFor(() => expect(screen.getByTestId("sessions-empty")).toBeInTheDocument());
    expect(screen.getByText("No conversations yet")).toBeInTheDocument();
  });

  it("shows the error state when the list fails", async () => {
    vi.mocked(sessionsApi.listAgentSessions).mockRejectedValue(new Error("boom"));

    renderPage();

    await waitFor(() => expect(screen.getByTestId("sessions-error")).toBeInTheDocument());
  });

  it("navigates to the replay page on row click", async () => {
    const user = userEvent.setup();
    vi.mocked(sessionsApi.listAgentSessions).mockResolvedValue(envelope([makeSession()]));

    renderPage();

    await waitFor(() => expect(screen.getByTestId("session-row")).toBeInTheDocument());
    await user.click(screen.getByTestId("session-row"));

    await waitFor(() => expect(screen.getByText("chat: location")).toBeInTheDocument());
  });
});
