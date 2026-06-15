// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";

import {
  RGDS_ALL_CATEGORIES,
  rgdsObjectToCategory,
  categoryToRgdsObject,
  parsePolicyString,
  formatPolicyString,
} from "./policy-utils";

describe("rgdsObjectToCategory", () => {
  it('maps the "*" wildcard to the all-categories sentinel', () => {
    expect(rgdsObjectToCategory("*")).toBe(RGDS_ALL_CATEGORIES);
  });

  it("maps an empty object to the all-categories sentinel", () => {
    expect(rgdsObjectToCategory("")).toBe(RGDS_ALL_CATEGORIES);
  });

  it('strips the "/*" suffix to yield the slug', () => {
    expect(rgdsObjectToCategory("databases/*")).toBe("databases");
    expect(rgdsObjectToCategory("uncategorized/*")).toBe("uncategorized");
  });

  it("returns a bare slug verbatim (legacy/non-canonical input)", () => {
    expect(rgdsObjectToCategory("networking")).toBe("networking");
  });
});

describe("categoryToRgdsObject", () => {
  it("maps the all-categories sentinel back to the wildcard", () => {
    expect(categoryToRgdsObject(RGDS_ALL_CATEGORIES)).toBe("*");
  });

  it("maps an empty selection to the wildcard", () => {
    expect(categoryToRgdsObject("")).toBe("*");
  });

  it('wraps a slug as "{slug}/*"', () => {
    expect(categoryToRgdsObject("databases")).toBe("databases/*");
  });
});

describe("rgds object round-trip", () => {
  it("canonicalizes through object → category → object", () => {
    for (const obj of ["*", "databases/*", "infra-networking/*"]) {
      const back = categoryToRgdsObject(rgdsObjectToCategory(obj));
      expect(back).toBe(obj);
    }
  });

  it("survives a full policy-string round-trip with a category scope", () => {
    // A category-scoped rgds policy as a role template would store it. The
    // server expands object "databases/*" to the Casbin object
    // "rgds/databases/*" — see addProjectScopedPolicyFromStringWithDests.
    const subject = { project: "{project}", role: "{role}" };
    const policy =
      "p, proj:{project}:{role}, rgds, get, databases/*, allow";
    const rule = parsePolicyString(policy, subject.project, subject.role);
    expect(rule).not.toBeNull();
    expect(rule!.resource).toBe("rgds");
    expect(rule!.object).toBe("databases/*");
    expect(rgdsObjectToCategory(rule!.object)).toBe("databases");
    expect(formatPolicyString(rule!, subject.project, subject.role)).toBe(
      policy
    );
  });
});
