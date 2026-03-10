// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { z } from "zod";
import type { ZodTypeAny } from "zod";
import type { FormProperty } from "@/types/rgd";

/**
 * Convert a FormProperty schema to a Zod schema for validation
 */
export function propertyToZod(property: FormProperty, required: boolean): ZodTypeAny {
  let schema: ZodTypeAny;

  switch (property.type) {
    case "string":
      schema = createStringSchema(property);
      break;
    case "integer":
    case "number":
      schema = createNumberSchema(property);
      break;
    case "boolean":
      schema = z.boolean();
      break;
    case "array":
      schema = createArraySchema(property);
      break;
    case "object":
      schema = createObjectSchema(property);
      break;
    default:
      // For unknown types, accept any value
      schema = z.any();
  }

  // Handle nullable
  if (property.nullable) {
    schema = schema.nullable();
  }

  // Make optional if not required
  if (!required) {
    schema = schema.optional();
  }

  return schema;
}

function createStringSchema(property: FormProperty): ZodTypeAny {
  let schema = z.string();

  // Handle enums
  if (property.enum && property.enum.length > 0) {
    const enumValues = property.enum.map(String);
    return z.enum(enumValues as [string, ...string[]]);
  }

  // Add validation rules
  if (property.minLength !== undefined) {
    schema = schema.min(property.minLength, {
      message: `Must be at least ${property.minLength} characters`,
    });
  }

  if (property.maxLength !== undefined) {
    schema = schema.max(property.maxLength, {
      message: `Must be at most ${property.maxLength} characters`,
    });
  }

  if (property.pattern) {
    try {
      schema = schema.regex(new RegExp(property.pattern), {
        message: `Must match pattern: ${property.pattern}`,
      });
    } catch {
      // Invalid regex pattern, skip
    }
  }

  // Handle format
  if (property.format) {
    switch (property.format) {
      case "email":
        schema = schema.email({ message: "Must be a valid email" });
        break;
      case "uri":
      case "url":
        schema = schema.url({ message: "Must be a valid URL" });
        break;
      case "uuid":
        schema = schema.uuid({ message: "Must be a valid UUID" });
        break;
      // date-time, date, time handled as strings
    }
  }

  return schema;
}

function createNumberSchema(property: FormProperty): ZodTypeAny {
  let schema = property.type === "integer" ? z.number().int() : z.number();

  if (property.minimum !== undefined) {
    schema = schema.min(property.minimum, {
      message: `Must be at least ${property.minimum}`,
    });
  }

  if (property.maximum !== undefined) {
    schema = schema.max(property.maximum, {
      message: `Must be at most ${property.maximum}`,
    });
  }

  // Handle coercion for form inputs (they return strings)
  return z.coerce.number().pipe(schema);
}

function createArraySchema(property: FormProperty): ZodTypeAny {
  const itemSchema = property.items
    ? propertyToZod(property.items, true)
    : z.any();

  return z.array(itemSchema);
}

function createObjectSchema(property: FormProperty): ZodTypeAny {
  if (!property.properties || Object.keys(property.properties).length === 0) {
    // Handle x-kubernetes-preserve-unknown-fields or empty objects
    if (property["x-kubernetes-preserve-unknown-fields"]) {
      return z.record(z.string(), z.unknown());
    }
    return z.object({});
  }

  const shape: Record<string, ZodTypeAny> = {};
  const requiredFields = property.required || [];

  for (const [key, prop] of Object.entries(property.properties)) {
    shape[key] = propertyToZod(prop, requiredFields.includes(key));
  }

  return z.object(shape);
}

/**
 * Build a complete Zod schema from FormProperty map
 */
export function buildFormSchema(
  properties: Record<string, FormProperty>,
  required: string[] = []
): z.ZodObject<Record<string, ZodTypeAny>> {
  const shape: Record<string, ZodTypeAny> = {};

  for (const [key, prop] of Object.entries(properties)) {
    shape[key] = propertyToZod(prop, required.includes(key));
  }

  return z.object(shape);
}

/**
 * Get default values from schema properties.
 * Priority order:
 * 1. RGD-defined default (prop.default)
 * 2. Type-appropriate fallback (empty string, false, [], {})
 *
 * Note: Advanced fields should have defaults defined by the RGD author.
 * This is the platform operator's responsibility to ensure secure defaults.
 */
export function getDefaultValues(
  properties: Record<string, FormProperty>
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};

  for (const [key, prop] of Object.entries(properties)) {
    // For object types with nested properties, always recurse to get nested defaults
    // This ensures nested defaults are applied even when parent has default: {}
    if (prop.type === "object" && prop.properties) {
      defaults[key] = getDefaultValues(prop.properties);
    } else if (prop.default !== undefined) {
      // RGD-defined default takes priority for non-object types
      defaults[key] = prop.default;
    } else {
      // Type-appropriate defaults
      switch (prop.type) {
        case "string":
          defaults[key] = "";
          break;
        case "integer":
        case "number":
          defaults[key] = undefined;
          break;
        case "boolean":
          defaults[key] = false;
          break;
        case "array":
          defaults[key] = [];
          break;
        case "object":
          // Object without properties
          defaults[key] = {};
          break;
      }
    }
  }

  return defaults;
}
