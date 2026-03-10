// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { renderHook } from "@testing-library/react";
import { useConditionalFields, getControllingFields } from "./useConditionalFields";
import type { ConditionalSection } from "@/types/rgd";

describe("useConditionalFields", () => {
  it("returns empty set when no conditional sections", () => {
    const { result } = renderHook(() =>
      useConditionalFields(undefined, {})
    );
    expect(result.current.size).toBe(0);
  });

  it("returns empty set when conditional sections array is empty", () => {
    const { result } = renderHook(() =>
      useConditionalFields([], {})
    );
    expect(result.current.size).toBe(0);
  });

  it("hides fields when controlling value is false", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.ingress.enabled == true",
        controllingField: "spec.ingress.enabled",
        expectedValue: true,
        affectedProperties: ["Ingress"],
      },
    ];

    const formValues = {
      ingress: {
        enabled: false,
        host: "example.com",
      },
    };

    const { result } = renderHook(() =>
      useConditionalFields(conditionalSections, formValues)
    );

    expect(result.current.has("Ingress")).toBe(true);
  });

  it("shows fields when controlling value is true", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.ingress.enabled == true",
        controllingField: "spec.ingress.enabled",
        expectedValue: true,
        affectedProperties: ["Ingress"],
      },
    ];

    const formValues = {
      ingress: {
        enabled: true,
        host: "example.com",
      },
    };

    const { result } = renderHook(() =>
      useConditionalFields(conditionalSections, formValues)
    );

    expect(result.current.has("Ingress")).toBe(false);
  });

  it("handles string expected values", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.mode == 'advanced'",
        controllingField: "spec.mode",
        expectedValue: "advanced",
        affectedProperties: ["AdvancedConfig"],
      },
    ];

    const formValues = {
      mode: "basic",
    };

    const { result } = renderHook(() =>
      useConditionalFields(conditionalSections, formValues)
    );

    expect(result.current.has("AdvancedConfig")).toBe(true);
  });

  it("handles nested controlling fields", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.database.replication.enabled == true",
        controllingField: "spec.database.replication.enabled",
        expectedValue: true,
        affectedProperties: ["ReplicationConfig"],
      },
    ];

    const formValues = {
      database: {
        replication: {
          enabled: false,
        },
      },
    };

    const { result } = renderHook(() =>
      useConditionalFields(conditionalSections, formValues)
    );

    expect(result.current.has("ReplicationConfig")).toBe(true);
  });
});

describe("getControllingFields", () => {
  it("returns empty set when no conditional sections", () => {
    const result = getControllingFields(undefined);
    expect(result.size).toBe(0);
  });

  it("extracts controlling fields from sections", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.ingress.enabled == true",
        controllingField: "spec.ingress.enabled",
        expectedValue: true,
        affectedProperties: ["Ingress"],
      },
    ];

    const result = getControllingFields(conditionalSections);

    expect(result.has("ingress.enabled")).toBe(true);
    expect(result.has("ingress")).toBe(true); // Parent path
  });

  it("handles multiple conditional sections", () => {
    const conditionalSections: ConditionalSection[] = [
      {
        condition: "spec.ingress.enabled == true",
        controllingField: "spec.ingress.enabled",
        expectedValue: true,
        affectedProperties: ["Ingress"],
      },
      {
        condition: "spec.monitoring.enabled == true",
        controllingField: "spec.monitoring.enabled",
        expectedValue: true,
        affectedProperties: ["Prometheus", "Grafana"],
      },
    ];

    const result = getControllingFields(conditionalSections);

    expect(result.has("ingress.enabled")).toBe(true);
    expect(result.has("ingress")).toBe(true);
    expect(result.has("monitoring.enabled")).toBe(true);
    expect(result.has("monitoring")).toBe(true);
  });
});

/**
 * Tests for the isFieldVisible logic that is used in DeployForm.
 * This tests the matching algorithm that maps resource kinds (e.g., "Ingress")
 * to form field paths (e.g., "ingress.host").
 */
describe("isFieldVisible logic", () => {
  // Helper function that mimics the isFieldVisible logic in DeployForm
  function isFieldVisible(
    fieldName: string,
    hiddenFields: Set<string>,
    controllingFields: Set<string>
  ): boolean {
    // Controlling fields are always visible
    if (controllingFields.has(fieldName)) {
      return true;
    }
    // Case-insensitive prefix matching
    const fieldNameLower = fieldName.toLowerCase();
    for (const hiddenProp of hiddenFields) {
      const hiddenPropLower = hiddenProp.toLowerCase();
      if (fieldNameLower.startsWith(hiddenPropLower)) {
        const nextChar = fieldNameLower[hiddenPropLower.length];
        if (nextChar === undefined || nextChar === ".") {
          return false;
        }
      }
    }
    return true;
  }

  it("hides ingress.host when Ingress is in hiddenFields", () => {
    const hiddenFields = new Set(["Ingress"]);
    const controllingFields = new Set(["ingress", "ingress.enabled"]);

    // ingress.enabled is a controlling field - always visible
    expect(isFieldVisible("ingress.enabled", hiddenFields, controllingFields)).toBe(true);

    // ingress.host is NOT a controlling field - should be hidden
    expect(isFieldVisible("ingress.host", hiddenFields, controllingFields)).toBe(false);
  });

  it("shows ingress.host when Ingress is NOT in hiddenFields", () => {
    const hiddenFields = new Set<string>();
    const controllingFields = new Set(["ingress", "ingress.enabled"]);

    expect(isFieldVisible("ingress.host", hiddenFields, controllingFields)).toBe(true);
  });

  it("does not hide ingressRoute when only Ingress is hidden", () => {
    const hiddenFields = new Set(["Ingress"]);
    const controllingFields = new Set<string>();

    // ingressRoute should NOT be hidden - it's a different resource
    expect(isFieldVisible("ingressRoute.path", hiddenFields, controllingFields)).toBe(true);
  });

  it("handles case-insensitive matching", () => {
    const hiddenFields = new Set(["INGRESS"]);
    const controllingFields = new Set<string>();

    expect(isFieldVisible("ingress.host", hiddenFields, controllingFields)).toBe(false);
    expect(isFieldVisible("Ingress.host", hiddenFields, controllingFields)).toBe(false);
  });

  it("hides top-level field matching resource kind", () => {
    const hiddenFields = new Set(["database"]);
    const controllingFields = new Set<string>();

    // Top-level field "database" should be hidden
    expect(isFieldVisible("database", hiddenFields, controllingFields)).toBe(false);
    // Nested fields should also be hidden
    expect(isFieldVisible("database.name", hiddenFields, controllingFields)).toBe(false);
    expect(isFieldVisible("database.port", hiddenFields, controllingFields)).toBe(false);
  });
});
