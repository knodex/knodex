import { describe, it, expect } from "vitest";
import type { Project } from "@/types/project";
import {
  getAllowedNamespaces,
  projectAllowsAllNamespaces,
  filterByProjectNamespaces,
} from "./project-utils";

// Helper to create test projects
function createProject(overrides: Partial<Project> = {}): Project {
  return {
    name: "test-project",
    resourceVersion: "1",
    createdAt: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("getAllowedNamespaces", () => {
  it("returns empty array when project has no destinations", () => {
    const project = createProject({ destinations: undefined });
    expect(getAllowedNamespaces(project)).toEqual([]);
  });

  it("returns empty array when destinations is empty", () => {
    const project = createProject({ destinations: [] });
    expect(getAllowedNamespaces(project)).toEqual([]);
  });

  it("extracts unique namespaces from destinations", () => {
    const project = createProject({
      destinations: [
        { namespace: "ns-a" },
        { namespace: "ns-b" },
        { namespace: "ns-a" }, // duplicate
      ],
    });
    expect(getAllowedNamespaces(project)).toEqual(["ns-a", "ns-b"]);
  });

  it("excludes wildcard namespace", () => {
    const project = createProject({
      destinations: [
        { namespace: "*" },
        { namespace: "ns-a" },
      ],
    });
    expect(getAllowedNamespaces(project)).toEqual(["ns-a"]);
  });

  it("returns empty array when all destinations are wildcards", () => {
    const project = createProject({
      destinations: [{ namespace: "*" }],
    });
    expect(getAllowedNamespaces(project)).toEqual([]);
  });

  it("handles destinations with missing namespace", () => {
    const project = createProject({
      destinations: [
        { name: "no-ns" }, // no namespace
        { namespace: "ns-a" },
      ],
    });
    expect(getAllowedNamespaces(project)).toEqual(["ns-a"]);
  });
});

describe("projectAllowsAllNamespaces", () => {
  it("returns false when project has no destinations", () => {
    const project = createProject({ destinations: undefined });
    expect(projectAllowsAllNamespaces(project)).toBe(false);
  });

  it("returns false when destinations is empty", () => {
    const project = createProject({ destinations: [] });
    expect(projectAllowsAllNamespaces(project)).toBe(false);
  });

  it("returns true when any destination has wildcard namespace", () => {
    const project = createProject({
      destinations: [
        { namespace: "ns-a" },
        { namespace: "*" },
      ],
    });
    expect(projectAllowsAllNamespaces(project)).toBe(true);
  });

  it("returns false when no destination has wildcard namespace", () => {
    const project = createProject({
      destinations: [
        { namespace: "ns-a" },
        { namespace: "ns-b" },
      ],
    });
    expect(projectAllowsAllNamespaces(project)).toBe(false);
  });

  it("returns true when only destination is wildcard", () => {
    const project = createProject({
      destinations: [{ namespace: "*" }],
    });
    expect(projectAllowsAllNamespaces(project)).toBe(true);
  });
});

describe("filterByProjectNamespaces", () => {
  const items = [
    { namespace: "ns-a", name: "item-1" },
    { namespace: "ns-b", name: "item-2" },
    { namespace: "ns-c", name: "item-3" },
    { namespace: "ns-a", name: "item-4" },
  ];

  it("returns all items when project is undefined", () => {
    expect(filterByProjectNamespaces(items, undefined)).toEqual(items);
  });

  it("returns all items when project allows all namespaces", () => {
    const project = createProject({
      destinations: [{ namespace: "*" }],
    });
    expect(filterByProjectNamespaces(items, project)).toEqual(items);
  });

  it("filters items to allowed namespaces", () => {
    const project = createProject({
      destinations: [
        { namespace: "ns-a" },
        { namespace: "ns-b" },
      ],
    });
    const result = filterByProjectNamespaces(items, project);
    expect(result).toHaveLength(3);
    expect(result.map((i) => i.name)).toEqual(["item-1", "item-2", "item-4"]);
  });

  it("returns empty array when project has no destinations", () => {
    const project = createProject({ destinations: [] });
    expect(filterByProjectNamespaces(items, project)).toEqual([]);
  });

  it("returns empty array when no items match allowed namespaces", () => {
    const project = createProject({
      destinations: [{ namespace: "ns-x" }],
    });
    expect(filterByProjectNamespaces(items, project)).toEqual([]);
  });

  it("handles empty items array", () => {
    const project = createProject({
      destinations: [{ namespace: "ns-a" }],
    });
    expect(filterByProjectNamespaces([], project)).toEqual([]);
  });

  it("works with generic type constraint", () => {
    const rgdItems = [
      { namespace: "ns-a", name: "rgd-1", tags: ["tag1"] },
      { namespace: "ns-b", name: "rgd-2", tags: ["tag2"] },
    ];
    const project = createProject({
      destinations: [{ namespace: "ns-a" }],
    });
    const result = filterByProjectNamespaces(rgdItems, project);
    expect(result).toHaveLength(1);
    expect(result[0].name).toBe("rgd-1");
    expect(result[0].tags).toEqual(["tag1"]);
  });

  describe("glob pattern matching", () => {
    const globItems = [
      { namespace: "staging-app", name: "item-1" },
      { namespace: "staging-data", name: "item-2" },
      { namespace: "staging", name: "item-3" },
      { namespace: "dev-app", name: "item-4" },
      { namespace: "prod-app", name: "item-5" },
      { namespace: "knodex-e2e", name: "item-6" },
    ];

    it("filters items using glob pattern with trailing wildcard", () => {
      const project = createProject({
        destinations: [{ namespace: "staging*" }],
      });
      const result = filterByProjectNamespaces(globItems, project);
      expect(result).toHaveLength(3);
      expect(result.map((i) => i.name)).toEqual(["item-1", "item-2", "item-3"]);
    });

    it("matches exact namespace when pattern has no wildcard", () => {
      const project = createProject({
        destinations: [{ namespace: "staging" }],
      });
      const result = filterByProjectNamespaces(globItems, project);
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("item-3");
    });

    it("supports multiple glob patterns", () => {
      const project = createProject({
        destinations: [
          { namespace: "staging*" },
          { namespace: "knodex*" },
        ],
      });
      const result = filterByProjectNamespaces(globItems, project);
      expect(result).toHaveLength(4);
      expect(result.map((i) => i.name)).toEqual([
        "item-1",
        "item-2",
        "item-3",
        "item-6",
      ]);
    });

    it("supports mixed exact and glob patterns", () => {
      const project = createProject({
        destinations: [
          { namespace: "staging*" },
          { namespace: "prod-app" }, // exact match
        ],
      });
      const result = filterByProjectNamespaces(globItems, project);
      expect(result).toHaveLength(4);
      expect(result.map((i) => i.name)).toEqual([
        "item-1",
        "item-2",
        "item-3",
        "item-5",
      ]);
    });

    it("handles patterns with wildcard in the middle", () => {
      const middleWildcardItems = [
        { namespace: "app-staging-v1", name: "item-1" },
        { namespace: "app-prod-v1", name: "item-2" },
        { namespace: "app-dev-v1", name: "item-3" },
      ];
      const project = createProject({
        destinations: [{ namespace: "app-*-v1" }],
      });
      const result = filterByProjectNamespaces(middleWildcardItems, project);
      expect(result).toHaveLength(3);
    });

    it("handles special regex characters in patterns", () => {
      const specialItems = [
        { namespace: "app.test", name: "item-1" },
        { namespace: "appXtest", name: "item-2" },
      ];
      const project = createProject({
        destinations: [{ namespace: "app.test" }],
      });
      const result = filterByProjectNamespaces(specialItems, project);
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("item-1");
    });
  });
});
