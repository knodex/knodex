import { describe, it, expect } from "vitest";
import { getDefaultValues, buildFormSchema, propertyToZod } from "./schema-to-zod";
import type { FormProperty } from "@/types/rgd";

describe("getDefaultValues", () => {
  it("returns RGD default when specified", () => {
    const properties: Record<string, FormProperty> = {
      replicas: {
        type: "integer",
        default: 3,
      },
    };

    const defaults = getDefaultValues(properties);
    expect(defaults.replicas).toBe(3);
  });

  it("returns type-appropriate defaults when no RGD default", () => {
    const properties: Record<string, FormProperty> = {
      name: { type: "string" },
      count: { type: "integer" },
      enabled: { type: "boolean" },
      items: { type: "array" },
      config: { type: "object" },
    };

    const defaults = getDefaultValues(properties);
    expect(defaults.name).toBe("");
    expect(defaults.count).toBeUndefined();
    expect(defaults.enabled).toBe(false);
    expect(defaults.items).toEqual([]);
    expect(defaults.config).toEqual({});
  });

  it("handles nested objects without properties", () => {
    const properties: Record<string, FormProperty> = {
      metadata: {
        type: "object",
        // No properties specified
      },
    };

    const defaults = getDefaultValues(properties);
    expect(defaults.metadata).toEqual({});
  });

  it("handles nested objects with RGD defaults", () => {
    const properties: Record<string, FormProperty> = {
      advanced: {
        type: "object",
        isAdvanced: true,
        properties: {
          replicas: {
            type: "integer",
            isAdvanced: true,
            default: 1,
          },
          memory: {
            type: "string",
            isAdvanced: true,
            default: "256Mi",
          },
        },
      },
    };

    const defaults = getDefaultValues(properties);
    expect(defaults.advanced).toEqual({
      replicas: 1,
      memory: "256Mi",
    });
  });

  it("handles mixed advanced and non-advanced properties with RGD defaults", () => {
    const properties: Record<string, FormProperty> = {
      name: {
        type: "string",
        isAdvanced: false,
      },
      replicas: {
        type: "integer",
        isAdvanced: true,
        default: 1,
      },
      memory: {
        type: "string",
        isAdvanced: true,
        default: "256Mi",
      },
    };

    const defaults = getDefaultValues(properties);
    expect(defaults.name).toBe("");
    expect(defaults.replicas).toBe(1);
    expect(defaults.memory).toBe("256Mi");
  });
});

describe("propertyToZod", () => {
  it("creates string schema with enum", () => {
    const property: FormProperty = {
      type: "string",
      enum: ["small", "medium", "large"],
    };

    const schema = propertyToZod(property, true);
    expect(schema.safeParse("small").success).toBe(true);
    expect(schema.safeParse("invalid").success).toBe(false);
  });

  it("creates integer schema with min/max", () => {
    const property: FormProperty = {
      type: "integer",
      minimum: 1,
      maximum: 10,
    };

    const schema = propertyToZod(property, true);
    expect(schema.safeParse(5).success).toBe(true);
    expect(schema.safeParse(0).success).toBe(false);
    expect(schema.safeParse(11).success).toBe(false);
  });

  it("creates boolean schema", () => {
    const property: FormProperty = {
      type: "boolean",
    };

    const schema = propertyToZod(property, true);
    expect(schema.safeParse(true).success).toBe(true);
    expect(schema.safeParse(false).success).toBe(true);
  });

  it("makes field optional when not required", () => {
    const property: FormProperty = {
      type: "string",
    };

    const schema = propertyToZod(property, false);
    expect(schema.safeParse(undefined).success).toBe(true);
  });

  it("handles nullable fields", () => {
    const property: FormProperty = {
      type: "string",
      nullable: true,
    };

    const schema = propertyToZod(property, true);
    expect(schema.safeParse(null).success).toBe(true);
  });
});

describe("buildFormSchema", () => {
  it("builds complete schema from properties", () => {
    const properties: Record<string, FormProperty> = {
      name: { type: "string" },
      port: { type: "integer", minimum: 1, maximum: 65535 },
      enabled: { type: "boolean" },
    };

    const schema = buildFormSchema(properties, ["name"]);

    // Valid data
    expect(
      schema.safeParse({ name: "test", port: 8080, enabled: true }).success
    ).toBe(true);

    // Missing required field
    expect(
      schema.safeParse({ port: 8080, enabled: true }).success
    ).toBe(false);

    // Invalid port
    expect(
      schema.safeParse({ name: "test", port: 70000, enabled: true }).success
    ).toBe(false);
  });

  it("handles nested object properties", () => {
    const properties: Record<string, FormProperty> = {
      config: {
        type: "object",
        properties: {
          host: { type: "string" },
          port: { type: "integer" },
        },
        required: ["host"],
      },
    };

    const schema = buildFormSchema(properties, ["config"]);

    expect(
      schema.safeParse({ config: { host: "localhost", port: 8080 } }).success
    ).toBe(true);
  });
});
