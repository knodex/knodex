// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { deriveActorLabel } from "./instance-utils";

describe("deriveActorLabel", () => {
  it("returns 'manual edit' when the instance has drifted from desired spec", () => {
    expect(deriveActorLabel({ deploymentMode: "gitops", gitopsDrift: true })).toBe("manual edit");
  });

  it("prefers 'manual edit' over deployment-mode label when drifted", () => {
    expect(deriveActorLabel({ deploymentMode: "hybrid", gitopsDrift: true })).toBe("manual edit");
  });

  it("returns 'via GitOps' for gitops deployment mode without drift", () => {
    expect(deriveActorLabel({ deploymentMode: "gitops", gitopsDrift: false })).toBe("via GitOps");
  });

  it("returns 'via GitOps' for hybrid deployment mode without drift", () => {
    expect(deriveActorLabel({ deploymentMode: "hybrid", gitopsDrift: false })).toBe("via GitOps");
  });

  it("returns 'via Knodex' for direct deployment mode", () => {
    expect(deriveActorLabel({ deploymentMode: "direct", gitopsDrift: false })).toBe("via Knodex");
  });

  it("returns 'via Knodex' when deployment mode is missing", () => {
    expect(deriveActorLabel({ gitopsDrift: false })).toBe("via Knodex");
  });

  it("returns 'via GitOps' when gitopsDrift is undefined (older API payloads)", () => {
    // Real-world default for instances coming back from servers that pre-date the gitopsDrift field.
    expect(deriveActorLabel({ deploymentMode: "gitops" })).toBe("via GitOps");
  });

  it("returns 'via Knodex' when both fields are absent", () => {
    expect(deriveActorLabel({})).toBe("via Knodex");
  });
});
