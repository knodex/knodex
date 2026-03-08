// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook } from "@testing-library/react";
import { useFieldVisibility } from "../useFieldVisibility";
import type { ConditionalSection } from "@/types/rgd";

describe("useFieldVisibility", () => {
  it("returns empty sets for undefined sections", () => {
    const { result } = renderHook(() =>
      useFieldVisibility(undefined, {})
    );

    expect(result.current.hiddenFields.size).toBe(0);
    expect(result.current.controllingFields.size).toBe(0);
    expect(result.current.isFieldVisible("anything")).toBe(true);
  });

  it("returns empty sets for empty sections", () => {
    const { result } = renderHook(() =>
      useFieldVisibility([], {})
    );

    expect(result.current.hiddenFields.size).toBe(0);
  });

  it("hides fields when simple boolean condition is unmet", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.enabled == true",
        controllingField: "enabled",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.enabled", op: "==", value: true }],
        affectedProperties: ["configMap", "secret"],
      },
    ];

    const { result } = renderHook(() =>
      useFieldVisibility(sections, { enabled: false })
    );

    expect(result.current.hiddenFields.has("configMap")).toBe(true);
    expect(result.current.hiddenFields.has("secret")).toBe(true);
    expect(result.current.isFieldVisible("configMap")).toBe(false);
  });

  it("shows fields when simple boolean condition is met", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.enabled == true",
        controllingField: "enabled",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.enabled", op: "==", value: true }],
        affectedProperties: ["configMap"],
      },
    ];

    const { result } = renderHook(() =>
      useFieldVisibility(sections, { enabled: true })
    );

    expect(result.current.hiddenFields.has("configMap")).toBe(false);
    expect(result.current.isFieldVisible("configMap")).toBe(true);
  });

  it("falls back to expectedValue when clientEvaluable is false", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.a || schema.spec.b",
        controllingField: "a",
        expectedValue: true,
        clientEvaluable: false,
        affectedProperties: ["featureField"],
      },
    ];

    // expectedValue=true, controlling value is false → hidden
    const { result } = renderHook(() =>
      useFieldVisibility(sections, { a: false })
    );

    expect(result.current.hiddenFields.has("featureField")).toBe(true);
  });

  it("controlling fields are always visible", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.enabled == true",
        controllingField: "enabled",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.enabled", op: "==", value: true }],
        affectedProperties: ["configMap"],
      },
    ];

    const { result } = renderHook(() =>
      useFieldVisibility(sections, { enabled: false })
    );

    // "enabled" is a controlling field — must always be visible
    expect(result.current.controllingFields.has("enabled")).toBe(true);
    expect(result.current.isFieldVisible("enabled")).toBe(true);
  });

  it("AND-based hiding: field shown when at least one controller is met", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.featureA == true",
        controllingField: "featureA",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.featureA", op: "==", value: true }],
        affectedProperties: ["sharedField"],
      },
      {
        condition: "schema.spec.featureB == true",
        controllingField: "featureB",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.featureB", op: "==", value: true }],
        affectedProperties: ["sharedField"],
      },
    ];

    // featureA is false, but featureB is true → sharedField should be VISIBLE
    const { result } = renderHook(() =>
      useFieldVisibility(sections, { featureA: false, featureB: true })
    );

    expect(result.current.isFieldVisible("sharedField")).toBe(true);
  });

  it("AND-based hiding: field hidden only when ALL controllers are unmet", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.featureA == true",
        controllingField: "featureA",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.featureA", op: "==", value: true }],
        affectedProperties: ["sharedField"],
      },
      {
        condition: "schema.spec.featureB == true",
        controllingField: "featureB",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.featureB", op: "==", value: true }],
        affectedProperties: ["sharedField"],
      },
    ];

    // Both controllers are false → sharedField should be HIDDEN
    const { result } = renderHook(() =>
      useFieldVisibility(sections, { featureA: false, featureB: false })
    );

    expect(result.current.isFieldVisible("sharedField")).toBe(false);
  });

  it("evaluates numeric comparison operators", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.replicas > 0",
        controllingField: "replicas",
        clientEvaluable: true,
        rules: [{ field: "spec.replicas", op: ">", value: 0 }],
        affectedProperties: ["replicaConfig"],
      },
    ];

    // replicas = 3 > 0 → visible
    const { result: visible } = renderHook(() =>
      useFieldVisibility(sections, { replicas: 3 })
    );
    expect(visible.current.isFieldVisible("replicaConfig")).toBe(true);

    // replicas = 0 is NOT > 0 → hidden
    const { result: hidden } = renderHook(() =>
      useFieldVisibility(sections, { replicas: 0 })
    );
    expect(hidden.current.isFieldVisible("replicaConfig")).toBe(false);
  });

  it("evaluates string comparison", () => {
    const sections: ConditionalSection[] = [
      {
        condition: 'schema.spec.mode == "advanced"',
        controllingField: "mode",
        clientEvaluable: true,
        rules: [{ field: "spec.mode", op: "==", value: "advanced" }],
        affectedProperties: ["advancedOption"],
      },
    ];

    const { result: match } = renderHook(() =>
      useFieldVisibility(sections, { mode: "advanced" })
    );
    expect(match.current.isFieldVisible("advancedOption")).toBe(true);

    const { result: noMatch } = renderHook(() =>
      useFieldVisibility(sections, { mode: "basic" })
    );
    expect(noMatch.current.isFieldVisible("advancedOption")).toBe(false);
  });

  it("handles nested field paths", () => {
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.ingress.enabled == true",
        controllingField: "ingress.enabled",
        clientEvaluable: true,
        rules: [{ field: "spec.ingress.enabled", op: "==", value: true }],
        affectedProperties: ["tlsSecretName"],
      },
    ];

    const { result } = renderHook(() =>
      useFieldVisibility(sections, { ingress: { enabled: true } })
    );

    expect(result.current.isFieldVisible("tlsSecretName")).toBe(true);
    // Parent path "ingress" should be a controlling field
    expect(result.current.controllingFields.has("ingress")).toBe(true);
    expect(result.current.controllingFields.has("ingress.enabled")).toBe(true);
  });

  it("AND-based hiding with template fields: shared field hidden only when all conditions unmet", () => {
    // Simulates the webapp-full-featured scenario:
    // hiddenAnnotation is in both enableDatabase and enableCache sections
    const sections: ConditionalSection[] = [
      {
        condition: "schema.spec.enableDatabase == true",
        controllingField: "enableDatabase",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.enableDatabase", op: "==", value: true }],
        affectedProperties: ["database", "hiddenAnnotation"],
      },
      {
        condition: "schema.spec.enableCache == true",
        controllingField: "enableCache",
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: "spec.enableCache", op: "==", value: true }],
        affectedProperties: ["hiddenAnnotation"],
      },
    ];

    // Both false → hiddenAnnotation hidden, database hidden
    const { result: allOff } = renderHook(() =>
      useFieldVisibility(sections, { enableDatabase: false, enableCache: false })
    );
    expect(allOff.current.isFieldVisible("hiddenAnnotation")).toBe(false);
    expect(allOff.current.isFieldVisible("database")).toBe(false);

    // enableDatabase true → hiddenAnnotation visible, database visible
    const { result: dbOn } = renderHook(() =>
      useFieldVisibility(sections, { enableDatabase: true, enableCache: false })
    );
    expect(dbOn.current.isFieldVisible("hiddenAnnotation")).toBe(true);
    expect(dbOn.current.isFieldVisible("database")).toBe(true);

    // enableCache true → hiddenAnnotation visible, database still hidden
    const { result: cacheOn } = renderHook(() =>
      useFieldVisibility(sections, { enableDatabase: false, enableCache: true })
    );
    expect(cacheOn.current.isFieldVisible("hiddenAnnotation")).toBe(true);
    expect(cacheOn.current.isFieldVisible("database")).toBe(false);

    // Both true → everything visible
    const { result: allOn } = renderHook(() =>
      useFieldVisibility(sections, { enableDatabase: true, enableCache: true })
    );
    expect(allOn.current.isFieldVisible("hiddenAnnotation")).toBe(true);
    expect(allOn.current.isFieldVisible("database")).toBe(true);
  });

  it("handles != operator", () => {
    const sections: ConditionalSection[] = [
      {
        condition: 'schema.spec.tier != "free"',
        controllingField: "tier",
        clientEvaluable: true,
        rules: [{ field: "spec.tier", op: "!=", value: "free" }],
        affectedProperties: ["premiumFeature"],
      },
    ];

    // tier = "premium" != "free" → visible
    const { result: premium } = renderHook(() =>
      useFieldVisibility(sections, { tier: "premium" })
    );
    expect(premium.current.isFieldVisible("premiumFeature")).toBe(true);

    // tier = "free" — not != "free" → hidden
    const { result: free } = renderHook(() =>
      useFieldVisibility(sections, { tier: "free" })
    );
    expect(free.current.isFieldVisible("premiumFeature")).toBe(false);
  });
});
