// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook } from "@testing-library/react";
import { isAdvancedField, useAdvancedFieldSplit } from "./useAdvancedFieldSplit";
import type { FormProperty, AdvancedSection } from "@/types/rgd";

describe("isAdvancedField", () => {
  it('returns true for "advanced"', () => {
    expect(isAdvancedField("advanced")).toBe(true);
  });

  it('returns true for "advanced.foo"', () => {
    expect(isAdvancedField("advanced.foo")).toBe(true);
  });

  it("returns false for regular fields", () => {
    expect(isAdvancedField("name")).toBe(false);
    expect(isAdvancedField("bastion")).toBe(false);
    expect(isAdvancedField("advancedOptions")).toBe(false);
  });
});

describe("useAdvancedFieldSplit", () => {
  const makeProps = (
    props: Record<string, Partial<FormProperty>>
  ): Record<string, FormProperty> =>
    Object.fromEntries(
      Object.entries(props).map(([k, v]) => [k, { type: "string", ...v } as FormProperty])
    );

  it("puts all fields in regularProperties when no advanced fields exist", () => {
    const properties = makeProps({
      name: { type: "string" },
      port: { type: "integer" },
    });

    const { result } = renderHook(() => useAdvancedFieldSplit(properties));

    expect(result.current.regularProperties).toHaveLength(2);
    expect(result.current.advancedProperties).toHaveLength(0);
    expect(result.current.globalSection).toBeNull();
  });

  it("splits top-level advanced key into advancedProperties and sets globalSection", () => {
    const properties = makeProps({
      name: { type: "string" },
      advanced: {
        type: "object",
        properties: {
          replicas: { type: "integer" } as FormProperty,
          cpu: { type: "string" } as FormProperty,
        },
      },
    });

    const advancedSections: AdvancedSection[] = [
      { path: "advanced", affectedProperties: ["advanced.replicas", "advanced.cpu"] },
    ];

    const { result } = renderHook(() =>
      useAdvancedFieldSplit(properties, advancedSections)
    );

    expect(result.current.regularProperties).toHaveLength(1);
    expect(result.current.regularProperties[0][0]).toBe("name");
    // Flattened children of the "advanced" object
    expect(result.current.advancedProperties).toHaveLength(2);
    expect(result.current.advancedProperties.map(([n]) => n)).toEqual(
      expect.arrayContaining(["advanced.replicas", "advanced.cpu"])
    );
    expect(result.current.globalSection).toEqual(advancedSections[0]);
  });

  it("keeps bastion (with nested advanced) in regularProperties and globalSection is null", () => {
    const properties = makeProps({
      name: { type: "string" },
      bastion: {
        type: "object",
        properties: {
          enabled: { type: "boolean" } as FormProperty,
          advanced: {
            type: "object",
            properties: {
              asoCredentialSecretName: { type: "string" } as FormProperty,
            },
          } as FormProperty,
        },
      },
    });

    const advancedSections: AdvancedSection[] = [
      {
        path: "bastion.advanced",
        affectedProperties: ["bastion.advanced.asoCredentialSecretName"],
      },
    ];

    const { result } = renderHook(() =>
      useAdvancedFieldSplit(properties, advancedSections)
    );

    expect(result.current.regularProperties).toHaveLength(2);
    expect(result.current.regularProperties.map(([n]) => n)).toEqual(["bastion", "name"]);
    expect(result.current.advancedProperties).toHaveLength(0);
    expect(result.current.globalSection).toBeNull();
  });

  it("respects propertyOrder when splitting regular properties", () => {
    const properties = makeProps({
      zebra: { type: "string" },
      alpha: { type: "string" },
      mike: { type: "string" },
    });

    const { result } = renderHook(() =>
      useAdvancedFieldSplit(properties, undefined, ["mike", "alpha", "zebra"])
    );

    expect(result.current.regularProperties.map(([n]) => n)).toEqual([
      "mike",
      "alpha",
      "zebra",
    ]);
  });

  it("orders advanced children by prop.propertyOrder when flattening", () => {
    const properties = makeProps({
      name: { type: "string" },
      advanced: {
        type: "object",
        properties: {
          replicas: { type: "integer" } as FormProperty,
          cpu: { type: "string" } as FormProperty,
          memory: { type: "string" } as FormProperty,
        },
        propertyOrder: ["memory", "cpu", "replicas"],
      },
    });

    const { result } = renderHook(() =>
      useAdvancedFieldSplit(properties, [
        { path: "advanced", affectedProperties: ["advanced.replicas", "advanced.cpu", "advanced.memory"] },
      ])
    );

    expect(result.current.advancedProperties.map(([n]) => n)).toEqual([
      "advanced.memory",
      "advanced.cpu",
      "advanced.replicas",
    ]);
  });

  it("handles both global and per-feature sections", () => {
    const properties = makeProps({
      name: { type: "string" },
      advanced: {
        type: "object",
        properties: {
          replicas: { type: "integer" } as FormProperty,
        },
      },
      bastion: {
        type: "object",
        properties: {
          enabled: { type: "boolean" } as FormProperty,
          advanced: {
            type: "object",
            properties: {
              secret: { type: "string" } as FormProperty,
            },
          } as FormProperty,
        },
      },
    });

    const advancedSections: AdvancedSection[] = [
      { path: "advanced", affectedProperties: ["advanced.replicas"] },
      { path: "bastion.advanced", affectedProperties: ["bastion.advanced.secret"] },
    ];

    const { result } = renderHook(() =>
      useAdvancedFieldSplit(properties, advancedSections)
    );

    // "bastion" stays in regular, "advanced" goes to advanced
    expect(result.current.regularProperties.map(([n]) => n)).toEqual(["bastion", "name"]);
    expect(result.current.advancedProperties).toHaveLength(1);
    expect(result.current.globalSection).toEqual(advancedSections[0]);
  });
});
