// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import type { Instance } from "@/types/rgd";
import type { Project } from "@/types/project";
import {
  computeProjectInstanceStats,
  instanceMatchesProject,
  namespaceMatchesPattern,
} from "./project-instances";

function inst(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "i",
    namespace: "alpha-apps",
    rgdName: "rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "App",
    health: "Healthy",
    conditions: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  } as Instance;
}

const project: Pick<Project, "destinations"> = {
  destinations: [{ namespace: "alpha-apps" }, { namespace: "dev-*" }],
};

describe("namespaceMatchesPattern", () => {
  it("matches exact namespaces", () => {
    expect(namespaceMatchesPattern("alpha-apps", "alpha-apps")).toBe(true);
    expect(namespaceMatchesPattern("alpha-apps", "beta-apps")).toBe(false);
  });

  it("matches '*' wildcard against any namespace", () => {
    expect(namespaceMatchesPattern("anything", "*")).toBe(true);
  });

  it("matches trailing-* prefix globs", () => {
    expect(namespaceMatchesPattern("dev-web", "dev-*")).toBe(true);
    expect(namespaceMatchesPattern("prod-web", "dev-*")).toBe(false);
  });

  it("never matches an empty (cluster-scoped) namespace except 'undefined' pattern guard", () => {
    expect(namespaceMatchesPattern("", "alpha-apps")).toBe(false);
    expect(namespaceMatchesPattern("alpha-apps", undefined)).toBe(false);
  });
});

describe("instanceMatchesProject", () => {
  it("matches by exact namespace membership", () => {
    expect(instanceMatchesProject(inst({ namespace: "alpha-apps" }), project)).toBe(true);
  });

  it("matches via a wildcard destination", () => {
    expect(instanceMatchesProject(inst({ namespace: "dev-payments" }), project)).toBe(true);
  });

  it("does not match a non-member namespace (zero-match)", () => {
    expect(instanceMatchesProject(inst({ namespace: "other-ns" }), project)).toBe(false);
  });

  it("does not match when project has no destinations", () => {
    expect(instanceMatchesProject(inst(), { destinations: [] })).toBe(false);
    expect(instanceMatchesProject(inst(), {})).toBe(false);
  });

  it("excludes cluster-scoped instances (empty namespace)", () => {
    expect(instanceMatchesProject(inst({ namespace: "" }), project)).toBe(false);
  });
});

describe("computeProjectInstanceStats", () => {
  const instances: Instance[] = [
    inst({ name: "a", namespace: "alpha-apps", health: "Healthy" }),
    inst({ name: "b", namespace: "dev-x", health: "Progressing" }),
    inst({ name: "c", namespace: "alpha-apps", health: "Degraded" }),
    inst({ name: "d", namespace: "alpha-apps", health: "Unhealthy" }),
    inst({ name: "e", namespace: "unrelated", health: "Healthy" }), // not matched
  ];

  it("aggregates matched counts and merges degraded+unhealthy into issues", () => {
    const stats = computeProjectInstanceStats(project, instances);
    expect(stats.total).toBe(4);
    expect(stats.matched.map((i) => i.name).sort()).toEqual(["a", "b", "c", "d"]);
    expect(stats.healthy).toBe(1);
    expect(stats.progressing).toBe(1);
    expect(stats.issues).toBe(2); // Degraded + Unhealthy merged
  });

  it("counts instances per destination namespace pattern", () => {
    const stats = computeProjectInstanceStats(project, instances);
    expect(stats.byNamespace).toEqual([
      { namespace: "alpha-apps", count: 3 },
      { namespace: "dev-*", count: 1 },
    ]);
  });

  it("returns zeros for a project with no matches", () => {
    const stats = computeProjectInstanceStats(
      { destinations: [{ namespace: "nothing-here" }] },
      instances
    );
    expect(stats.total).toBe(0);
    expect(stats.issues).toBe(0);
    expect(stats.byNamespace).toEqual([{ namespace: "nothing-here", count: 0 }]);
  });
});
