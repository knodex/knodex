// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { validateInstanceName } from "./validate-instance-name";

describe("validateInstanceName", () => {
  it("requires a name", () => {
    expect(validateInstanceName("")).toBe("Instance name is required");
  });

  it("rejects names longer than 63 characters", () => {
    expect(validateInstanceName("a".repeat(64))).toBe(
      "Must be 63 characters or fewer",
    );
  });

  it("rejects invalid formats", () => {
    expect(validateInstanceName("-bad")).toMatch(/Lowercase letters/);
    expect(validateInstanceName("bad-")).toMatch(/Lowercase letters/);
    expect(validateInstanceName("Bad")).toMatch(/Lowercase letters/);
    expect(validateInstanceName("a_b")).toMatch(/Lowercase letters/);
  });

  it("accepts valid kubernetes-style names", () => {
    expect(validateInstanceName("a")).toBe("");
    expect(validateInstanceName("web-app-1")).toBe("");
    expect(validateInstanceName("a".repeat(63))).toBe("");
  });
});
