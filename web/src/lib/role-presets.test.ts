// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";

import { isEnterprise } from "@/hooks/useCompliance";

// Mock isEnterprise before importing role-presets
vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: vi.fn(() => false),
}));

import { resolvePresetPolicies, resolvePreset } from "./role-presets";
import type { RolePreset } from "./role-presets";

// As of Story 18.1 the preset CATALOG is server-backed (the static ROLE_PRESETS
// array was retired; defaults live in the Go store and are covered by
// server/internal/roletemplates/store_test.go). These tests cover only the
// client-side RESOLUTION logic this module still owns.

// An admin-shaped template fixture, matching the server default's policy shape.
const adminTemplate: RolePreset = {
  name: "admin",
  label: "Admin",
  description: "Full project management access",
  policies: [
    "p, proj:{project}:{role}, projects, *, {project}, allow",
    "p, proj:{project}:{role}, instances, *, */{project}/*, allow",
    "p, proj:{project}:{role}, rgds, get, *, allow",
    "p, proj:{project}:{role}, repositories, *, {project}/*, allow",
  ],
};

describe("resolvePresetPolicies", () => {
  it("replaces {project} and {role} placeholders", () => {
    const preset: RolePreset = {
      name: "tester",
      label: "Tester",
      description: "Test role",
      policies: ["p, proj:{project}:{role}, instances, *, */{project}/*, allow"],
    };
    const resolved = resolvePresetPolicies(preset, "my-project");
    expect(resolved).toEqual([
      "p, proj:my-project:tester, instances, *, */my-project/*, allow",
    ]);
  });

  it("leaves no unresolved placeholders", () => {
    const resolved = resolvePresetPolicies(adminTemplate, "alpha");
    for (const policy of resolved) {
      expect(policy).not.toContain("{project}");
      expect(policy).not.toContain("{role}");
    }
  });
});

describe("enterprise compliance injection", () => {
  it("injects compliance policy for the admin template when isEnterprise() is true", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(true);
    const resolved = resolvePresetPolicies(adminTemplate, "alpha");
    expect(resolved).toContainEqual(
      "p, proj:alpha:admin, compliance, get, alpha/*, allow"
    );
  });

  it("omits compliance policy when isEnterprise() is false", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(false);
    const resolved = resolvePresetPolicies(adminTemplate, "alpha");
    expect(resolved.some((p) => p.includes("compliance"))).toBe(false);
  });

  it("injects read-only compliance for the developer template when isEnterprise() is true", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(true);
    const developer: RolePreset = {
      name: "developer",
      label: "Developer",
      policies: ["p, proj:{project}:{role}, rgds, get, *, allow"],
    };
    const resolved = resolvePresetPolicies(developer, "alpha");
    // Read-only: get only (no create/update/delete/*) — mirrors ADMIN's get
    // grant but is the only compliance verb a developer receives.
    expect(resolved).toContainEqual(
      "p, proj:alpha:developer, compliance, get, alpha/*, allow"
    );
    expect(
      resolved.some((p) => /compliance,\s*(?:create|update|delete|\*)/.test(p))
    ).toBe(false);
  });

  it("never injects compliance for a template that is neither admin nor developer, even in enterprise mode", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(true);
    const viewer: RolePreset = {
      name: "viewer",
      label: "Viewer",
      policies: ["p, proj:{project}:{role}, rgds, get, *, allow"],
    };
    const resolved = resolvePresetPolicies(viewer, "alpha");
    expect(resolved.some((p) => p.includes("compliance"))).toBe(false);
  });
});

describe("resolvePreset", () => {
  it("returns a ProjectRole with resolved policies and no groups field (teams-only binding)", () => {
    const preset: RolePreset = {
      name: "dev",
      label: "Dev",
      description: "Dev role",
      policies: ["p, proj:{project}:{role}, rgds, get, *, allow"],
    };
    const role = resolvePreset(preset, "alpha");
    expect(role).toEqual({
      name: "dev",
      description: "Dev role",
      policies: ["p, proj:alpha:dev, rgds, get, *, allow"],
    });
    // Legacy roles[].groups[] was removed in epic-10 — resolvePreset must not seed it.
    expect("groups" in role).toBe(false);
  });
});
