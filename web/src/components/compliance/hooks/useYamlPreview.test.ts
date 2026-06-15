// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { jsonToYaml } from "./useYamlPreview";
import { cleanParameters, buildMatchRules } from "./constraintUtils";
import type { ConstraintFormValues } from "./useConstraintFormValidation";

// Mock getApiGroupValue
vi.mock("@/api/apiResources", () => ({
  getApiGroupValue: (g: string) => g === "core" ? "" : g,
}));

describe("cleanParameters", () => {
  it("removes undefined values", () => {
    expect(cleanParameters({ a: "hello", b: undefined })).toEqual({ a: "hello" });
  });

  it("removes empty strings", () => {
    expect(cleanParameters({ a: "hello", b: "" })).toEqual({ a: "hello" });
  });

  it("removes NaN values", () => {
    expect(cleanParameters({ a: 1, b: NaN })).toEqual({ a: 1 });
  });

  it("removes empty arrays", () => {
    expect(cleanParameters({ a: [1], b: [] })).toEqual({ a: [1] });
  });

  it("recursively cleans nested objects", () => {
    expect(cleanParameters({ a: { b: "", c: "val" } })).toEqual({ a: { c: "val" } });
  });

  it("removes empty nested objects after cleaning", () => {
    expect(cleanParameters({ a: { b: "" } })).toEqual({});
  });
});

describe("buildMatchRules", () => {
  it("returns null when no kinds and no namespaces", () => {
    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
      matchKinds: [{ apiGroups: [], kinds: [] }],
      matchNamespaces: "",
    };
    expect(buildMatchRules(data)).toBeNull();
  });

  it("builds match rules with kinds", () => {
    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
      matchKinds: [{ apiGroups: ["core"], kinds: ["Pod"] }],
    };
    const result = buildMatchRules(data);
    expect(result?.kinds).toEqual([{ apiGroups: [""], kinds: ["Pod"] }]);
  });

  it("builds match rules with namespaces", () => {
    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
      matchNamespaces: "default, production",
    };
    const result = buildMatchRules(data);
    expect(result?.namespaces).toEqual(["default", "production"]);
  });

  it("builds match rules combining kinds and namespaces", () => {
    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
      matchKinds: [{ apiGroups: ["apps"], kinds: ["Deployment"] }],
      matchNamespaces: "staging",
    };
    const result = buildMatchRules(data);
    expect(result?.kinds).toEqual([{ apiGroups: ["apps"], kinds: ["Deployment"] }]);
    expect(result?.namespaces).toEqual(["staging"]);
  });

  it("filters out match kinds with empty kinds array", () => {
    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
      matchKinds: [
        { apiGroups: [], kinds: ["Pod"] },
        { apiGroups: [], kinds: [] },
      ],
    };
    const result = buildMatchRules(data);
    expect(result?.kinds).toHaveLength(1);
  });
});

describe("jsonToYaml string escaping", () => {
  it("wraps plain strings in double quotes", () => {
    expect(jsonToYaml("hello", 0)).toBe('"hello"');
  });

  it("escapes double-quote characters in strings", () => {
    expect(jsonToYaml('He said "hello"', 0)).toBe('"He said \\"hello\\""');
  });

  it("escapes backslashes in strings", () => {
    expect(jsonToYaml("C:\\Users\\admin", 0)).toBe('"C:\\\\Users\\\\admin"');
  });

  it("escapes both backslashes and double quotes", () => {
    expect(jsonToYaml('path: "C:\\\\dir"', 0)).toBe('"path: \\"C:\\\\\\\\dir\\""');
  });

  it("uses block scalar for multi-line strings", () => {
    const result = jsonToYaml("line1\nline2", 2);
    expect(result).toMatch(/^\|\n/);
    expect(result).toContain("line1");
    expect(result).toContain("line2");
  });

  it("renders numbers without quotes", () => {
    expect(jsonToYaml(42, 0)).toBe("42");
  });

  it("renders booleans without quotes", () => {
    expect(jsonToYaml(true, 0)).toBe("true");
  });
});
