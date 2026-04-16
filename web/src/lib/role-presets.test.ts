// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";

import { isEnterprise } from "@/hooks/useCompliance";

// Mock isEnterprise before importing role-presets
vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: vi.fn(() => false),
}));

import {
  ROLE_PRESETS,
  resolvePresetPolicies,
  resolvePreset,
} from "./role-presets";
import type { RolePreset } from "./role-presets";

describe("ROLE_PRESETS", () => {
  it("contains exactly 3 presets: admin, developer, readonly", () => {
    expect(ROLE_PRESETS).toHaveLength(3);
    expect(ROLE_PRESETS.map((p) => p.name)).toEqual([
      "admin",
      "developer",
      "readonly",
    ]);
  });

  it("does not include a platform preset", () => {
    expect(ROLE_PRESETS.find((p) => p.name === "platform")).toBeUndefined();
  });

  it("no preset references clusters resource", () => {
    for (const preset of ROLE_PRESETS) {
      for (const policy of preset.policies) {
        expect(policy).not.toContain("clusters");
      }
    }
  });

  it("no non-admin preset includes secrets access", () => {
    const nonAdmin = ROLE_PRESETS.filter((p) => p.name !== "admin");
    for (const preset of nonAdmin) {
      for (const policy of preset.policies) {
        expect(policy).not.toContain("secrets");
      }
    }
  });

  it("admin preset has no secrets access (secrets are managed outside presets)", () => {
    const admin = ROLE_PRESETS.find((p) => p.name === "admin")!;
    for (const policy of admin.policies) {
      expect(policy).not.toContain("secrets");
    }
  });
});

describe("admin preset", () => {
  const admin = ROLE_PRESETS.find((p) => p.name === "admin")!;

  it("has category-scoped instance policy", () => {
    expect(admin.policies).toContainEqual(
      expect.stringContaining("instances, *, */{project}/*, allow")
    );
  });

  it("retains project management", () => {
    expect(admin.policies).toContainEqual(
      expect.stringContaining("projects, *, {project}, allow")
    );
  });

  it("retains repository access", () => {
    expect(admin.policies).toContainEqual(
      expect.stringContaining("repositories, *, {project}/*, allow")
    );
  });
});

describe("developer preset", () => {
  const developer = ROLE_PRESETS.find((p) => p.name === "developer")!;

  it("has category-scoped instance policy with wildcard action", () => {
    expect(developer.policies).toContainEqual(
      expect.stringContaining("instances, *, */{project}/*, allow")
    );
  });

  it("has rgds get and list", () => {
    expect(developer.policies).toContainEqual(
      expect.stringContaining("rgds, get, *, allow")
    );
    expect(developer.policies).toContainEqual(
      expect.stringContaining("rgds, list, *, allow")
    );
  });

  it("has read-only repository access", () => {
    expect(developer.policies).toContainEqual(
      expect.stringContaining("repositories, get, {project}/*, allow")
    );
    expect(developer.policies).toContainEqual(
      expect.stringContaining("repositories, list, {project}/*, allow")
    );
  });
});

describe("readonly preset", () => {
  const readonly = ROLE_PRESETS.find((p) => p.name === "readonly")!;

  it("has category-scoped instance get and list policies", () => {
    expect(readonly.policies).toContainEqual(
      expect.stringContaining("instances, get, */{project}/*, allow")
    );
    expect(readonly.policies).toContainEqual(
      expect.stringContaining("instances, list, */{project}/*, allow")
    );
  });

  it("has no wildcard instance action", () => {
    const instancePolicies = readonly.policies.filter((p) =>
      p.includes("instances")
    );
    for (const policy of instancePolicies) {
      expect(policy).not.toMatch(/instances, \*, /);
    }
  });

  it("has rgds get and list", () => {
    expect(readonly.policies).toContainEqual(
      expect.stringContaining("rgds, get, *, allow")
    );
    expect(readonly.policies).toContainEqual(
      expect.stringContaining("rgds, list, *, allow")
    );
  });
});

describe("admin preset (enterprise mode)", () => {
  it("includes compliance policy in resolved output when isEnterprise returns true", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(true);
    const admin = ROLE_PRESETS.find((p) => p.name === "admin")!;
    const resolved = resolvePresetPolicies(admin, "alpha");
    expect(resolved).toContainEqual(
      "p, proj:alpha:admin, compliance, get, alpha/*, allow"
    );
  });

  it("excludes compliance policy in resolved output when isEnterprise returns false", () => {
    vi.mocked(isEnterprise).mockReturnValueOnce(false);
    const admin = ROLE_PRESETS.find((p) => p.name === "admin")!;
    const resolved = resolvePresetPolicies(admin, "alpha");
    expect(resolved.some((p) => p.includes("compliance"))).toBe(false);
  });
});

describe("resolvePresetPolicies", () => {
  it("replaces {project} and {role} placeholders", () => {
    const preset: RolePreset = {
      name: "tester",
      label: "Tester",
      description: "Test role",
      policies: [
        "p, proj:{project}:{role}, instances, *, */{project}/*, allow",
      ],
    };
    const resolved = resolvePresetPolicies(preset, "my-project");
    expect(resolved).toEqual([
      "p, proj:my-project:tester, instances, *, */my-project/*, allow",
    ]);
  });

  it("resolves actual developer preset for a real project", () => {
    const developer = ROLE_PRESETS.find((p) => p.name === "developer")!;
    const resolved = resolvePresetPolicies(developer, "alpha");
    expect(resolved).toContainEqual(
      "p, proj:alpha:developer, instances, *, */alpha/*, allow"
    );
    expect(resolved).toContainEqual(
      "p, proj:alpha:developer, rgds, get, *, allow"
    );
    // No unresolved placeholders
    for (const policy of resolved) {
      expect(policy).not.toContain("{project}");
      expect(policy).not.toContain("{role}");
    }
  });
});

describe("resolvePreset", () => {
  it("returns a ProjectRole with resolved policies", () => {
    const preset: RolePreset = {
      name: "dev",
      label: "Dev",
      description: "Dev role",
      policies: [
        "p, proj:{project}:{role}, rgds, get, *, allow",
      ],
    };
    const role = resolvePreset(preset, "alpha");
    expect(role).toEqual({
      name: "dev",
      description: "Dev role",
      policies: ["p, proj:alpha:dev, rgds, get, *, allow"],
      groups: [],
    });
  });
});
