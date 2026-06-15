// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("./client", () => {
  class ApiError extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  }
  return {
    default: { get: vi.fn() },
    ApiError,
  };
});

import { ApiError } from "./client";
import {
  listAgentSessions,
  getAgentSession,
  type AgentSessionSummary,
  type AgentSession,
} from "./agent-sessions";

const summary: AgentSessionSummary = {
  id: "conv-1",
  agentType: "rgd-builder",
  agentNamespace: "",
  firstPrompt: "build me a web app",
  startedAt: "2026-06-06T10:00:00Z",
  lastActivityAt: "2026-06-06T10:05:00Z",
  runCount: 2,
  status: "completed",
};

describe("listAgentSessions", () => {
  beforeEach(() => vi.clearAllMocks());

  it("calls the sessions endpoint without params and unwraps the envelope", async () => {
    const { default: apiClient } = await import("./client");
    const payload = { items: [summary], total: 1, page: 1, pageSize: 20 };
    vi.mocked(apiClient.get).mockResolvedValue({ data: payload });

    const result = await listAgentSessions();

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/sessions");
    expect(result).toEqual(payload);
  });

  it("builds query params for agentType and pagination", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { items: [], total: 0, page: 2, pageSize: 50 },
    });

    await listAgentSessions({ agentType: "rgd-builder", page: 2, pageSize: 50 });

    expect(apiClient.get).toHaveBeenCalledWith(
      "/v1/agents/sessions?agentType=rgd-builder&page=2&pageSize=50"
    );
  });

  it("omits unset params from the query string", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
    });

    await listAgentSessions({ agentType: "rgd-builder" });

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/sessions?agentType=rgd-builder");
  });

  it("propagates transport errors", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(new Error("network down"));

    await expect(listAgentSessions()).rejects.toThrow("network down");
  });
});

describe("getAgentSession", () => {
  beforeEach(() => vi.clearAllMocks());

  const session: AgentSession = {
    id: "conv-1",
    agentType: "rgd-builder",
    agentNamespace: "",
    runs: [],
  };

  it("GETs the session endpoint (url-encoded id) and returns the payload", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({ data: session });

    const result = await getAgentSession("conv 1/x");

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/sessions/conv%201%2Fx");
    expect(result).toEqual(session);
  });

  it("returns null on a 404 (brand-new or not-visible session) instead of throwing", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(
      new ApiError("NOT_FOUND", "agent session not found", 404)
    );

    await expect(getAgentSession("ghost")).resolves.toBeNull();
  });

  it("propagates a non-404 error to the caller", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(
      new ApiError("INTERNAL", "boom", 500)
    );

    await expect(getAgentSession("conv-1")).rejects.toThrow("boom");
  });
});
