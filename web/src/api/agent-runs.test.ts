// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock apiClient — ApiError must be a real class so the 404→null mapping's
// instanceof check works against errors created in these tests.
vi.mock("./client", () => {
  class ApiError extends Error {
    code: string;
    status: number;
    details?: unknown;
    constructor(code: string, message: string, status: number, details?: unknown) {
      super(message);
      this.code = code;
      this.status = status;
      this.details = details;
    }
  }
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
    },
    ApiError,
  };
});

import { ApiError } from "./client";
import {
  listAgentRuns,
  invokeAgent,
  getAgentRunResult,
  type AgentRun,
  type AgentRunResult,
} from "./agent-runs";

const sampleRun: AgentRun = {
  id: "run-1",
  actor: "dev@example.com",
  agentType: "helper",
  agentNamespace: "alpha-apps",
  contextRef: "rgd:webapp",
  kagentSessionId: "ctx-123",
  inputSummary: "what should I do?",
  recommendationSummary: "scale to 3 replicas",
  actionTaken: "",
  timestamp: "2026-06-06T10:00:00Z",
  completedAt: "2026-06-06T10:00:30Z",
  status: "completed",
  triggerType: "on_demand",
};

describe("listAgentRuns", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls the runs endpoint without params and unwraps the envelope", async () => {
    const { default: apiClient } = await import("./client");
    const payload = { items: [sampleRun], total: 1, page: 1, pageSize: 20 };
    vi.mocked(apiClient.get).mockResolvedValue({ data: payload });

    const result = await listAgentRuns();

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/runs");
    expect(result).toEqual(payload);
  });

  it("builds query params for filters and pagination", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { items: [], total: 0, page: 2, pageSize: 50 },
    });

    await listAgentRuns({ agentType: "helper", status: "failed", page: 2, pageSize: 50 });

    expect(apiClient.get).toHaveBeenCalledWith(
      "/v1/agents/runs?agentType=helper&status=failed&page=2&pageSize=50"
    );
  });

  it("omits unset params from the query string", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
    });

    await listAgentRuns({ status: "running" });

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/runs?status=running");
  });

  it("passes through an empty list (no runs yet is 200, not an error)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { items: [], total: 0, page: 1, pageSize: 20 },
    });

    const result = await listAgentRuns();

    expect(result.items).toEqual([]);
    expect(result.total).toBe(0);
  });

  it("propagates transport errors to the caller (React Query surfaces isError)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(new Error("network down"));

    await expect(listAgentRuns()).rejects.toThrow("network down");
  });
});

describe("invokeAgent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("POSTs the message to the namespaced invoke endpoint and returns the run", async () => {
    const { default: apiClient } = await import("./client");
    const acceptedRun: AgentRun = {
      ...sampleRun,
      agentType: "helper",
      agentNamespace: "alpha-apps",
      status: "running",
      completedAt: undefined,
    };
    vi.mocked(apiClient.post).mockResolvedValue({ data: acceptedRun });

    const result = await invokeAgent("alpha-apps", "helper", "web app with redis");

    expect(apiClient.post).toHaveBeenCalledWith("/v1/agents/alpha-apps/helper/invoke", {
      message: "web app with redis",
    });
    expect(result).toEqual(acceptedRun);
    expect(result.agentNamespace).toBe("alpha-apps");
  });

  it("URL-encodes the namespace and name segments", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.post).mockResolvedValue({ data: sampleRun });

    await invokeAgent("weird/ns", "weird/name", "hi");

    expect(apiClient.post).toHaveBeenCalledWith("/v1/agents/weird%2Fns/weird%2Fname/invoke", {
      message: "hi",
    });
  });

  it("includes conversationId in the body when provided (Story 50.6)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.post).mockResolvedValue({ data: sampleRun });

    await invokeAgent("alpha-apps", "helper", "hi", "conv-xyz");

    expect(apiClient.post).toHaveBeenCalledWith("/v1/agents/alpha-apps/helper/invoke", {
      message: "hi",
      conversationId: "conv-xyz",
    });
  });

  it("includes kagentContextId in the body when provided (session continuity)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.post).mockResolvedValue({ data: sampleRun });

    await invokeAgent("alpha-apps", "helper", "hi", "conv-xyz", "ctx-abc");

    expect(apiClient.post).toHaveBeenCalledWith("/v1/agents/alpha-apps/helper/invoke", {
      message: "hi",
      conversationId: "conv-xyz",
      kagentContextId: "ctx-abc",
    });
  });

  it("omits conversationId from the body when not provided (backward compatible)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.post).mockResolvedValue({ data: sampleRun });

    await invokeAgent("alpha-apps", "helper", "hi");

    expect(apiClient.post).toHaveBeenCalledWith("/v1/agents/alpha-apps/helper/invoke", {
      message: "hi",
    });
  });

  it("propagates errors (404 unknown agent, 503 store down)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.post).mockRejectedValue(new ApiError("NOT_FOUND", "agent not found", 404));

    await expect(invokeAgent("ghost-ns", "ghost", "hi")).rejects.toThrow("agent not found");
  });
});

describe("getAgentRunResult", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const sampleResult: AgentRunResult = {
    runId: "run-1",
    agentNamespace: "",
    status: "completed",
    response: "Here is your spec:\n```yaml\nkind: ResourceGraphDefinition\n```",
    error: "",
    completedAt: "2026-06-06T10:00:30Z",
  };

  it("GETs the result endpoint and returns the payload", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({ data: sampleResult });

    const result = await getAgentRunResult("run-1");

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/runs/run-1/result");
    expect(result).toEqual(sampleResult);
  });

  it("maps a 404 to null (still in-flight — the polling signal)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(
      new ApiError("NOT_FOUND", "agent run result not found", 404)
    );

    await expect(getAgentRunResult("in-flight")).resolves.toBeNull();
  });

  it("propagates non-404 errors", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(
      new ApiError("INTERNAL_ERROR", "boom", 500)
    );

    await expect(getAgentRunResult("run-1")).rejects.toThrow("boom");
  });

  it("propagates plain transport errors (no status)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(new Error("network down"));

    await expect(getAgentRunResult("run-1")).rejects.toThrow("network down");
  });
});
