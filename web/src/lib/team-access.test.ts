// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import type { ProjectRole } from "@/types/project";
import {
  deriveTeamBindings,
  assignTeamToRole,
  removeTeamFromProject,
  boundTeamNames,
  scopeLabel,
} from "./team-access";

const roles: ProjectRole[] = [
  { name: "admin", policies: ["x"], teams: ["platform-eng"], destinations: [] },
  {
    name: "developer",
    policies: ["y"],
    teams: ["payments"],
    destinations: ["payments-system", "payments-dev"],
  },
  { name: "readonly", policies: ["z"], teams: [], destinations: ["payments-system"] },
];

describe("deriveTeamBindings", () => {
  it("produces one binding per team, sorted by team name, with the role's scope", () => {
    const bindings = deriveTeamBindings(roles);
    expect(bindings.map((b) => b.team)).toEqual(["payments", "platform-eng"]);

    const payments = bindings.find((b) => b.team === "payments")!;
    expect(payments.primaryRole).toBe("developer");
    expect(payments.roleNames).toEqual(["developer"]);
    expect(payments.destinations).toEqual(["payments-system", "payments-dev"]);

    const platform = bindings.find((b) => b.team === "platform-eng")!;
    expect(platform.primaryRole).toBe("admin");
    expect(platform.destinations).toEqual([]); // all namespaces
  });

  it("surfaces a team bound to multiple roles via roleNames (needs consolidation)", () => {
    const multi: ProjectRole[] = [
      { name: "admin", teams: ["dup"] },
      { name: "developer", teams: ["dup"] },
    ];
    const [binding] = deriveTeamBindings(multi);
    expect(binding.team).toBe("dup");
    expect(binding.roleNames).toEqual(["admin", "developer"]);
    expect(binding.primaryRole).toBe("admin");
  });

  it("returns nothing for roles with no team bindings", () => {
    expect(deriveTeamBindings([{ name: "admin", teams: [] }])).toEqual([]);
    expect(deriveTeamBindings([{ name: "admin" }])).toEqual([]);
  });
});

describe("assignTeamToRole", () => {
  it("adds the team to the target role when not yet bound", () => {
    const next = assignTeamToRole(roles, "newteam", "admin");
    expect(next.find((r) => r.name === "admin")!.teams).toContain("newteam");
  });

  it("moves a team to the target role, removing it from all others (consolidation)", () => {
    const multi: ProjectRole[] = [
      { name: "admin", teams: ["dup", "keep"] },
      { name: "developer", teams: ["dup"] },
    ];
    const next = assignTeamToRole(multi, "dup", "developer");
    expect(next.find((r) => r.name === "admin")!.teams).toEqual(["keep"]);
    expect(next.find((r) => r.name === "developer")!.teams).toEqual(["dup"]);
  });

  it("is idempotent when the team already binds only the target role", () => {
    const next = assignTeamToRole(roles, "payments", "developer");
    expect(next.find((r) => r.name === "developer")!.teams).toEqual(["payments"]);
    expect(next).toEqual(roles);
  });

  it("does not mutate the input roles", () => {
    const snapshot = JSON.parse(JSON.stringify(roles));
    assignTeamToRole(roles, "payments", "admin");
    expect(roles).toEqual(snapshot);
  });
});

describe("removeTeamFromProject", () => {
  it("removes the team from every role", () => {
    const multi: ProjectRole[] = [
      { name: "admin", teams: ["dup", "keep"] },
      { name: "developer", teams: ["dup"] },
    ];
    const next = removeTeamFromProject(multi, "dup");
    expect(next.find((r) => r.name === "admin")!.teams).toEqual(["keep"]);
    expect(next.find((r) => r.name === "developer")!.teams).toEqual([]);
  });

  it("leaves roles untouched when the team is not bound", () => {
    expect(removeTeamFromProject(roles, "ghost")).toEqual(roles);
  });
});

describe("boundTeamNames", () => {
  it("returns distinct bound team names", () => {
    expect(boundTeamNames(roles).sort()).toEqual(["payments", "platform-eng"]);
  });
});

describe("scopeLabel", () => {
  it("maps empty destinations to All namespaces", () => {
    expect(scopeLabel([])).toBe("All namespaces");
  });
  it("shows a single namespace name", () => {
    expect(scopeLabel(["payments-system"])).toBe("payments-system");
  });
  it("summarizes multiple namespaces", () => {
    expect(scopeLabel(["a", "b", "c"])).toBe("3 namespaces");
  });
});
