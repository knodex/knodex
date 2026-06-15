// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { validateGroupId } from "./oidc-groups";

describe("validateGroupId", () => {
  it("rejects empty / whitespace-only input", () => {
    expect(validateGroupId("")).toBe("Group ID cannot be empty");
    expect(validateGroupId("   ")).toBe("Group ID cannot be empty");
  });

  it("rejects leading/trailing whitespace", () => {
    expect(validateGroupId(" devs")).toBe(
      "Group ID cannot have leading/trailing spaces",
    );
    expect(validateGroupId("devs ")).toBe(
      "Group ID cannot have leading/trailing spaces",
    );
  });

  it("rejects overly long ids (> 253 chars)", () => {
    expect(validateGroupId("a".repeat(254))).toBe(
      "Group ID is too long (max 253 characters)",
    );
  });

  it("rejects invalid characters", () => {
    expect(validateGroupId("group!name")).toBe(
      "Group ID contains invalid characters",
    );
    expect(validateGroupId("group name")).toBe(
      "Group ID contains invalid characters",
    );
  });

  it("accepts valid ids (uuid, dashes, dots, @, slashes)", () => {
    expect(validateGroupId("alpha-devs")).toBeNull();
    expect(validateGroupId("00000000-1111-2222-3333-444444444444")).toBeNull();
    expect(validateGroupId("team@example.com")).toBeNull();
    expect(validateGroupId("org/team")).toBeNull();
  });
});
