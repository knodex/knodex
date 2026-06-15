// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { runsRefetchInterval, RUNNING_REFETCH_INTERVAL_MS } from "./useAgentRuns";
import type { AgentRun, AgentRunsResponse } from "@/api/agent-runs";

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

function queryWith(data?: AgentRunsResponse) {
  return { state: { data } };
}

function envelope(items: AgentRun[]): AgentRunsResponse {
  return { items, total: items.length, page: 1, pageSize: 20 };
}

/**
 * The conditional poll is the ONLY convergence path for non-actors (the
 * WebSocket agent_run_update push is actor-or-admin delivery, Story 49.4
 * Task 3) — these cases pin when it runs and, just as important, when it
 * stops (AC #2 / NFR-A1: no idle polling).
 */
describe("runsRefetchInterval", () => {
  it("polls every 5s while any run on the page is running", () => {
    const query = queryWith(
      envelope([makeRun({ id: "run-1", status: "completed" }), makeRun({ id: "run-2", status: "running" })])
    );
    expect(runsRefetchInterval(query)).toBe(RUNNING_REFETCH_INTERVAL_MS);
  });

  it("stops polling once every run is terminal (completed/failed)", () => {
    const query = queryWith(
      envelope([makeRun({ id: "run-1", status: "completed" }), makeRun({ id: "run-2", status: "failed" })])
    );
    expect(runsRefetchInterval(query)).toBe(false);
  });

  it("does not poll an empty page", () => {
    expect(runsRefetchInterval(queryWith(envelope([])))).toBe(false);
  });

  it("does not poll before the first fetch resolves (no data yet)", () => {
    expect(runsRefetchInterval(queryWith(undefined))).toBe(false);
  });
});
