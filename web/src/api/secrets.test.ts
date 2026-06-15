// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock apiClient before importing the API surface — the spec is "the API
// client must hit /api/v1/namespaces/{ns}/secrets[/{name}] with no
// ?project= query and no namespace in the body", which is a contract on
// the underlying axios calls, not on the network.
vi.mock("./client", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    head: vi.fn(),
  },
}));

import apiClient from "./client";
import {
  listSecrets,
  createSecret,
  getSecret,
  updateSecret,
  deleteSecret,
  checkSecretExists,
} from "./secrets";

describe("secrets API client — AC3 URL-shape regression guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.get).mockResolvedValue({ data: { items: [], pageCount: 0, hasMore: false } });
    vi.mocked(apiClient.post).mockResolvedValue({ data: {} });
    vi.mocked(apiClient.put).mockResolvedValue({ data: {} });
    vi.mocked(apiClient.delete).mockResolvedValue({ data: {} });
    vi.mocked(apiClient.head).mockResolvedValue({ data: undefined });
  });

  // ---------------------------------------------------------------------------
  // AC3: the original bug was "project query parameter is required" on a
  // deep-link load with the "All Projects" selector. The fix is at the URL
  // contract layer: the API client must NEVER send ?project= and must use
  // the namespace-keyed path shape. These tests pin the contract so the
  // regression cannot return even if the E2E for it stays skipped.
  // ---------------------------------------------------------------------------

  it("listSecrets calls /v1/secrets with no ?project= parameter", async () => {
    await listSecrets();
    expect(apiClient.get).toHaveBeenCalledWith("/v1/secrets", {
      params: undefined,
    });
    // The params object (undefined here) must never carry a "project" key.
    // We assert absence directly rather than against a non-existent params
    // bag so the test fails loudly if a future regression starts passing
    // `params: { project: ... }`.
    const [, options] = vi.mocked(apiClient.get).mock.calls[0];
    expect(options?.params ?? {}).not.toHaveProperty("project");
  });

  it("listSecrets with namespace filter forwards only namespace/limit/continue", async () => {
    await listSecrets({ namespace: "xxx-shared", limit: 50 });
    expect(apiClient.get).toHaveBeenCalledWith("/v1/secrets", {
      params: { namespace: "xxx-shared", limit: 50 },
    });
    const [, options] = vi.mocked(apiClient.get).mock.calls[0];
    expect(options?.params).not.toHaveProperty("project");
  });

  it("getSecret hits /v1/namespaces/{ns}/secrets/{name} with NO ?project=", async () => {
    await getSecret("my-secret", "xxx-shared");
    expect(apiClient.get).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets/my-secret",
    );
    // No second arg → no query params at all. The original 400 bug was
    // caused by a missing query, so we double-down on the absence here.
    const [, options] = vi.mocked(apiClient.get).mock.calls[0];
    expect(options).toBeUndefined();
  });

  it("getSecret URL-encodes name and namespace", async () => {
    await getSecret("with/slash", "with.dot");
    expect(apiClient.get).toHaveBeenCalledWith(
      "/v1/namespaces/with.dot/secrets/with%2Fslash",
    );
  });

  it("createSecret POSTs to /v1/namespaces/{ns}/secrets with no namespace in body", async () => {
    const body = { name: "new-secret", data: { k: "v" } };
    await createSecret("xxx-shared", body);
    expect(apiClient.post).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets",
      body,
    );
    // The body must NOT carry a namespace field — the server reads it from
    // the URL path, mirroring Instances.
    const [, postedBody] = vi.mocked(apiClient.post).mock.calls[0];
    expect(postedBody).not.toHaveProperty("namespace");
    expect(postedBody).not.toHaveProperty("project");
  });

  it("updateSecret PUTs to namespace-keyed URL with data-only body when no metadata", async () => {
    await updateSecret("my-secret", "xxx-shared", { data: { k: "v2" } });
    expect(apiClient.put).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets/my-secret",
      { data: { k: "v2" } },
    );
    const [, body] = vi.mocked(apiClient.put).mock.calls[0];
    expect(body).not.toHaveProperty("namespace");
    expect(body).not.toHaveProperty("project");
  });

  it("updateSecret includes metadata in body when caller passes it", async () => {
    await updateSecret("my-secret", "xxx-shared", {
      data: { k: "v2" },
      metadata: { rotation: "manual" },
    });
    expect(apiClient.put).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets/my-secret",
      { data: { k: "v2" }, metadata: { rotation: "manual" } },
    );
  });

  it("deleteSecret DELETEs the namespace-keyed URL with no query", async () => {
    await deleteSecret("my-secret", "xxx-shared");
    expect(apiClient.delete).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets/my-secret",
    );
    const [, options] = vi.mocked(apiClient.delete).mock.calls[0];
    expect(options).toBeUndefined();
  });

  it("checkSecretExists HEADs the namespace-keyed URL with no query", async () => {
    await checkSecretExists("my-secret", "xxx-shared");
    expect(apiClient.head).toHaveBeenCalledWith(
      "/v1/namespaces/xxx-shared/secrets/my-secret",
    );
    const [, options] = vi.mocked(apiClient.head).mock.calls[0];
    expect(options).toBeUndefined();
  });

  // ---------------------------------------------------------------------------
  // Defensive cross-check: walk every exported function and assert no call
  // anywhere across the suite passed a "project" parameter. This is the
  // belt-and-suspenders guard the original bug demands: if a future change
  // accidentally reintroduces ?project= in any branch, this catches it.
  // ---------------------------------------------------------------------------
  it("no API call ever forwards a project query parameter (defensive)", async () => {
    await listSecrets();
    await listSecrets({ namespace: "ns" });
    await getSecret("name", "ns");
    await createSecret("ns", { name: "n", data: { k: "v" } });
    await updateSecret("name", "ns", { data: { k: "v" } });
    await deleteSecret("name", "ns");
    await checkSecretExists("name", "ns");

    const allCalls = [
      ...vi.mocked(apiClient.get).mock.calls,
      ...vi.mocked(apiClient.post).mock.calls,
      ...vi.mocked(apiClient.put).mock.calls,
      ...vi.mocked(apiClient.delete).mock.calls,
      ...vi.mocked(apiClient.head).mock.calls,
    ];

    for (const call of allCalls) {
      const url = call[0] as string;
      expect(url).not.toContain("project=");
      expect(url).not.toContain("?project");

      // Inspect each arg following the URL — could be a body (POST/PUT) or
      // an options object (GET/DELETE). Either way it must not contain a
      // "project" key.
      for (let i = 1; i < call.length; i++) {
        const arg = call[i];
        if (arg && typeof arg === "object") {
          expect(arg).not.toHaveProperty("project");
          // Nested params check (for GET/DELETE with axios config)
          const maybeOpts = arg as { params?: unknown };
          if (maybeOpts.params && typeof maybeOpts.params === "object") {
            expect(maybeOpts.params).not.toHaveProperty("project");
          }
        }
      }
    }
  });
});
