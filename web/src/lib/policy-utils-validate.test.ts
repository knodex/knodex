// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";

import {
  parsePolicyString,
  validatePolicyRule,
  type PolicyRule,
} from "./policy-utils";

vi.mock("@/lib/logger", () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

describe("parsePolicyString error/edge branches", () => {
  it("returns null when the policy has too few parts", () => {
    expect(parsePolicyString("p, proj:a:b, rgds", "a", "b")).toBeNull();
  });

  it("still parses when the subject does not match (warns, does not throw)", () => {
    const rule = parsePolicyString(
      "p, proj:other:role, instances, get, alpha/ns, allow",
      "alpha",
      "developer",
    );
    expect(rule).not.toBeNull();
    expect(rule!.resource).toBe("instances");
    expect(rule!.permission).toBe("allow");
  });

  it("defaults the permission to allow when omitted", () => {
    const rule = parsePolicyString(
      "p, proj:a:b, instances, get, alpha/ns, ",
      "a",
      "b",
    );
    expect(rule!.permission).toBe("allow");
  });

  it("handles a policy without the leading 'p, ' prefix", () => {
    const rule = parsePolicyString(
      "proj:a:b, instances, get, alpha/ns, deny",
      "a",
      "b",
    );
    expect(rule!.permission).toBe("deny");
  });
});

describe("validatePolicyRule", () => {
  const base: PolicyRule = {
    resource: "instances",
    action: "get",
    object: "alpha/ns",
    permission: "allow",
  };

  it("accepts a complete valid rule", () => {
    expect(validatePolicyRule(base)).toBeNull();
  });

  it("requires a resource", () => {
    expect(validatePolicyRule({ ...base, resource: "" })).toBe(
      "Resource is required",
    );
  });

  it("requires an action", () => {
    expect(validatePolicyRule({ ...base, action: "" })).toBe(
      "Action is required",
    );
  });

  it("requires an object pattern", () => {
    expect(validatePolicyRule({ ...base, object: "" })).toBe(
      "Object pattern is required",
    );
  });

  it("rejects an invalid permission", () => {
    expect(
      validatePolicyRule({ ...base, permission: "maybe" as never }),
    ).toBe("Permission must be 'allow' or 'deny'");
  });
});
