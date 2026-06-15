// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock apiClient
vi.mock("./client", () => ({
  default: {
    get: vi.fn(),
  },
}));

import { getAgentsStatus, listAgents } from "./agents";

describe("getAgentsStatus", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls the agents status endpoint and unwraps the response", async () => {
    const { default: apiClient } = await import("./client");
    const payload = {
      status: "ready",
      crdPresent: true,
      controllerHealthy: true,
      message: "kagent is installed and healthy",
    };
    vi.mocked(apiClient.get).mockResolvedValue({ data: payload });

    const result = await getAgentsStatus();

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents/status");
    expect(result).toEqual(payload);
  });

  it("passes through degraded payloads (server never 5xxs for degraded)", async () => {
    const { default: apiClient } = await import("./client");
    const payload = {
      status: "degraded",
      crdPresent: null,
      controllerHealthy: null,
      message: "kagent CRD discovery failed",
    };
    vi.mocked(apiClient.get).mockResolvedValue({ data: payload });

    const result = await getAgentsStatus();

    expect(result.status).toBe("degraded");
    expect(result.crdPresent).toBeNull();
    expect(result.controllerHealthy).toBeNull();
  });

  it("propagates transport errors to the caller (React Query surfaces isError)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(new Error("network down"));

    await expect(getAgentsStatus()).rejects.toThrow("network down");
  });
});

describe("listAgents", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls the agents endpoint and unwraps the single Casbin-scoped list", async () => {
    const { default: apiClient } = await import("./client");
    const payload = {
      agents: [
        {
          name: "alpha-helper",
          namespace: "alpha-apps",
          description: "Helps with alpha things",
          createdAt: "2026-06-01T10:00:00Z",
        },
      ],
    };
    vi.mocked(apiClient.get).mockResolvedValue({ data: payload });

    const result = await listAgents();

    expect(apiClient.get).toHaveBeenCalledWith("/v1/agents");
    expect(result).toEqual(payload);
  });

  it("passes through an empty list (no project access is 200, not an error)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockResolvedValue({ data: { agents: [] } });

    const result = await listAgents();

    expect(result.agents).toEqual([]);
  });

  it("propagates transport errors to the caller (React Query surfaces isError)", async () => {
    const { default: apiClient } = await import("./client");
    vi.mocked(apiClient.get).mockRejectedValue(new Error("network down"));

    await expect(listAgents()).rejects.toThrow("network down");
  });
});
