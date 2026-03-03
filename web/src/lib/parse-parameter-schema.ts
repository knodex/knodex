import type { FormProperty } from "@/types/rgd";

/**
 * OpenAPI v3 schema structure used in ConstraintTemplate parameters
 */
interface OpenAPISchema {
  type?: string;
  properties?: Record<string, OpenAPISchema>;
  required?: string[];
  items?: OpenAPISchema;
  enum?: unknown[];
  default?: unknown;
  description?: string;
  title?: string;
  format?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  nullable?: boolean;
  "x-kubernetes-preserve-unknown-fields"?: boolean;
  additionalProperties?: boolean | OpenAPISchema;
}

/**
 * Check if a schema can be rendered as a dynamic form.
 * Returns false for schemas that are too complex or unstructured.
 */
export function canRenderAsForm(schema: Record<string, unknown> | undefined): boolean {
  if (!schema) return false;

  const typed = schema as OpenAPISchema;

  // Must have properties or be a simple type
  if (typed.properties && Object.keys(typed.properties).length > 0) {
    return true;
  }

  // Check if it's a simple schema with type
  if (typed.type && ["string", "number", "integer", "boolean", "array"].includes(typed.type)) {
    return true;
  }

  return false;
}

/**
 * Parse an OpenAPI parameter schema from a ConstraintTemplate into FormProperty format.
 * This enables reusing the existing FormField components from the deploy form.
 *
 * @param schema - The OpenAPI v3 schema from template.parameters
 * @returns Parsed schema with properties in FormProperty format, or null if unparseable
 */
export function parseParameterSchema(
  schema: Record<string, unknown> | undefined
): { properties: Record<string, FormProperty>; required: string[] } | null {
  if (!schema) return null;

  const typed = schema as OpenAPISchema;

  // If schema has properties at the top level, parse them
  if (typed.properties) {
    const properties: Record<string, FormProperty> = {};

    for (const [key, propSchema] of Object.entries(typed.properties)) {
      properties[key] = convertToFormProperty(propSchema, key);
    }

    return {
      properties,
      required: typed.required || [],
    };
  }

  // If schema is a single property (rare but possible), wrap it
  if (typed.type) {
    return {
      properties: {
        value: convertToFormProperty(typed, "value"),
      },
      required: [],
    };
  }

  return null;
}

/**
 * Convert an OpenAPI schema property to FormProperty format
 */
function convertToFormProperty(schema: OpenAPISchema, key: string): FormProperty {
  const property: FormProperty = {
    type: schema.type || "string",
    title: schema.title,
    description: schema.description,
    default: schema.default,
    enum: schema.enum,
    format: schema.format,
    minimum: schema.minimum,
    maximum: schema.maximum,
    minLength: schema.minLength,
    maxLength: schema.maxLength,
    pattern: schema.pattern,
    nullable: schema.nullable,
    "x-kubernetes-preserve-unknown-fields": schema["x-kubernetes-preserve-unknown-fields"],
  };

  // Handle nested objects
  if (schema.type === "object" && schema.properties) {
    property.properties = {};
    for (const [propKey, propSchema] of Object.entries(schema.properties)) {
      property.properties[propKey] = convertToFormProperty(propSchema, propKey);
    }
    property.required = schema.required;
  }

  // Handle arrays
  if (schema.type === "array" && schema.items) {
    property.items = convertToFormProperty(schema.items, `${key}[]`);
  }

  // Handle additionalProperties (free-form object)
  if (schema.type === "object" && !schema.properties && schema.additionalProperties) {
    property["x-kubernetes-preserve-unknown-fields"] = true;
  }

  return property;
}

/**
 * Get default values from a parsed parameter schema
 */
export function getParameterDefaultValues(
  properties: Record<string, FormProperty>
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};

  for (const [key, prop] of Object.entries(properties)) {
    if (prop.default !== undefined) {
      defaults[key] = prop.default;
    } else {
      // Set type-appropriate defaults
      switch (prop.type) {
        case "string":
          if (prop.enum && prop.enum.length > 0) {
            defaults[key] = prop.enum[0];
          } else {
            defaults[key] = "";
          }
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
          if (prop.properties) {
            // Recursively get defaults for nested objects
            defaults[key] = getParameterDefaultValues(prop.properties);
          } else {
            defaults[key] = {};
          }
          break;
        default:
          defaults[key] = undefined;
      }
    }
  }

  return defaults;
}
