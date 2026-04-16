// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  sanitizeUrlParam,
  getCatalogFiltersFromURL,
  setCatalogFiltersToURL,
  getInstanceFiltersFromURL,
  setInstanceFiltersToURL,
} from "./url-utils";

describe("sanitizeUrlParam", () => {
  it("removes HTML special characters", () => {
    expect(sanitizeUrlParam("<script>alert('xss')</script>")).toBe(
      "scriptalert(xss)/script"
    );
  });

  it("rejects javascript: protocol prefix to prevent XSS in href contexts", () => {
    expect(sanitizeUrlParam("javascript:alert(1)")).toBe("");
  });

  it("rejects javascript: with mixed case", () => {
    expect(sanitizeUrlParam("JavaScript:alert(1)")).toBe("");
  });

  it("strips equals sign from event handler patterns", () => {
    expect(sanitizeUrlParam("onclick=alert(1)")).toBe("onclickalert(1)");
    expect(sanitizeUrlParam("onmouseover=evil()")).toBe("onmouseoverevil()");
  });

  it("strips angle brackets from data: protocol payloads", () => {
    expect(sanitizeUrlParam("data:text/html,<h1>test</h1>")).toBe(
      "data:text/html,h1test/h1"
    );
  });

  it("trims whitespace", () => {
    expect(sanitizeUrlParam("  test  ")).toBe("test");
  });

  it("limits length to 200 characters", () => {
    const longString = "a".repeat(250);
    expect(sanitizeUrlParam(longString)).toHaveLength(200);
  });

  it("preserves safe characters", () => {
    expect(sanitizeUrlParam("hello-world_123")).toBe("hello-world_123");
  });
});

describe("getCatalogFiltersFromURL", () => {
  const originalWindow = global.window;

  beforeEach(() => {
    // Reset window mock
    Object.defineProperty(global, "window", {
      value: {
        location: {
          search: "",
        },
      },
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(global, "window", {
      value: originalWindow,
      writable: true,
    });
  });

  it("returns default values when no params", () => {
    window.location.search = "";
    const result = getCatalogFiltersFromURL();
    expect(result).toEqual({
      search: "",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "",
    });
  });

  it("parses search query parameter", () => {
    window.location.search = "?q=my-search";
    const result = getCatalogFiltersFromURL();
    expect(result.search).toBe("my-search");
  });

  it("parses tags parameter", () => {
    window.location.search = "?tags=tag1,tag2,tag3";
    const result = getCatalogFiltersFromURL();
    expect(result.tags).toEqual(["tag1", "tag2", "tag3"]);
  });

  it("parses all parameters together", () => {
    window.location.search = "?q=search&tags=a,b&category=database";
    const result = getCatalogFiltersFromURL();
    expect(result).toEqual({
      search: "search",
      tags: ["a", "b"],
      category: "database",
      projectScoped: false,
      producesKind: "",
    });
  });

  it("parses producesKind parameter", () => {
    window.location.search = "?producesKind=ManagedCluster";
    const result = getCatalogFiltersFromURL();
    expect(result.producesKind).toBe("ManagedCluster");
  });

  it("sanitizes producesKind parameter", () => {
    window.location.search = "?producesKind=<script>alert(1)</script>";
    const result = getCatalogFiltersFromURL();
    expect(result.producesKind).not.toContain("<");
    expect(result.producesKind).not.toContain(">");
  });

  it("sanitizes malicious input", () => {
    window.location.search = "?q=<script>alert(1)</script>";
    const result = getCatalogFiltersFromURL();
    expect(result.search).not.toContain("<");
    expect(result.search).not.toContain(">");
  });

  it("limits tags to 20", () => {
    const manyTags = Array.from({ length: 30 }, (_, i) => `tag${i}`).join(",");
    window.location.search = `?tags=${manyTags}`;
    const result = getCatalogFiltersFromURL();
    expect(result.tags).toHaveLength(20);
  });

  it("returns defaults when window is undefined", () => {
    Object.defineProperty(global, "window", {
      value: undefined,
      writable: true,
    });
    const result = getCatalogFiltersFromURL();
    expect(result).toEqual({
      search: "",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "",
    });
  });
});

describe("setCatalogFiltersToURL", () => {
  const originalWindow = global.window;
  let replaceStateMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    replaceStateMock = vi.fn();
    Object.defineProperty(global, "window", {
      value: {
        location: {
          pathname: "/catalog",
        },
        history: {
          replaceState: replaceStateMock,
        },
      },
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(global, "window", {
      value: originalWindow,
      writable: true,
    });
  });

  it("sets all filter parameters", () => {
    setCatalogFiltersToURL({
      search: "test",
      tags: ["a", "b"],
      category: "database",
      projectScoped: false,
      producesKind: "",
    });
    expect(replaceStateMock).toHaveBeenCalledWith(
      {},
      "",
      "/catalog?q=test&category=database&tags=a%2Cb"
    );
  });

  it("sets only pathname when all filters empty", () => {
    setCatalogFiltersToURL({
      search: "",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "",
    });
    expect(replaceStateMock).toHaveBeenCalledWith({}, "", "/catalog");
  });

  it("sanitizes values before setting", () => {
    setCatalogFiltersToURL({
      search: "<script>",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "",
    });
    expect(replaceStateMock).toHaveBeenCalled();
    const url = replaceStateMock.mock.calls[0][2];
    expect(url).not.toContain("<");
    expect(url).not.toContain(">");
  });

  it("sets producesKind parameter", () => {
    setCatalogFiltersToURL({
      search: "",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "ManagedCluster",
    });
    expect(replaceStateMock).toHaveBeenCalledWith(
      {},
      "",
      "/catalog?producesKind=ManagedCluster"
    );
  });

  it("does nothing when window is undefined", () => {
    Object.defineProperty(global, "window", {
      value: undefined,
      writable: true,
    });
    // Should not throw
    setCatalogFiltersToURL({
      search: "test",
      tags: [],
      category: "",
      projectScoped: false,
      producesKind: "",
    });
  });
});

describe("getInstanceFiltersFromURL", () => {
  const originalWindow = global.window;

  beforeEach(() => {
    Object.defineProperty(global, "window", {
      value: {
        location: {
          search: "",
        },
      },
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(global, "window", {
      value: originalWindow,
      writable: true,
    });
  });

  it("returns default values when no params", () => {
    window.location.search = "";
    const result = getInstanceFiltersFromURL();
    expect(result).toEqual({
      search: "",
      rgd: "",
      health: "",
      scope: "",
    });
  });

  it("parses search query parameter", () => {
    window.location.search = "?q=my-instance";
    const result = getInstanceFiltersFromURL();
    expect(result.search).toBe("my-instance");
  });

  it("parses rgd parameter", () => {
    window.location.search = "?rgd=my-database";
    const result = getInstanceFiltersFromURL();
    expect(result.rgd).toBe("my-database");
  });

  it("parses health parameter", () => {
    window.location.search = "?health=Healthy";
    const result = getInstanceFiltersFromURL();
    expect(result.health).toBe("Healthy");
  });

  it("parses all parameters together", () => {
    window.location.search = "?q=test&rgd=my-database&health=Degraded&scope=cluster";
    const result = getInstanceFiltersFromURL();
    expect(result).toEqual({
      search: "test",
      rgd: "my-database",
      health: "Degraded",
      scope: "cluster",
    });
  });

  it("returns defaults when window is undefined", () => {
    Object.defineProperty(global, "window", {
      value: undefined,
      writable: true,
    });
    const result = getInstanceFiltersFromURL();
    expect(result).toEqual({
      search: "",
      rgd: "",
      health: "",
      scope: "",
    });
  });
});

describe("setInstanceFiltersToURL", () => {
  const originalWindow = global.window;
  let replaceStateMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    replaceStateMock = vi.fn();
    Object.defineProperty(global, "window", {
      value: {
        location: {
          pathname: "/instances",
        },
        history: {
          replaceState: replaceStateMock,
        },
      },
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(global, "window", {
      value: originalWindow,
      writable: true,
    });
  });

  it("sets all filter parameters", () => {
    setInstanceFiltersToURL({
      search: "test",
      rgd: "my-database",
      health: "Healthy",
      scope: "namespaced",
    });
    expect(replaceStateMock).toHaveBeenCalledWith(
      {},
      "",
      "/instances?q=test&rgd=my-database&health=Healthy&scope=namespaced"
    );
  });

  it("omits empty parameters", () => {
    setInstanceFiltersToURL({
      search: "",
      rgd: "",
      health: "Healthy",
      scope: "",
    });
    expect(replaceStateMock).toHaveBeenCalledWith(
      {},
      "",
      "/instances?health=Healthy"
    );
  });

  it("sets only pathname when all filters empty", () => {
    setInstanceFiltersToURL({
      search: "",
      rgd: "",
      health: "",
      scope: "",
    });
    expect(replaceStateMock).toHaveBeenCalledWith({}, "", "/instances");
  });

  it("does nothing when window is undefined", () => {
    Object.defineProperty(global, "window", {
      value: undefined,
      writable: true,
    });
    // Should not throw
    setInstanceFiltersToURL({
      search: "test",
      rgd: "",
      health: "",
      scope: "",
    });
  });
});
