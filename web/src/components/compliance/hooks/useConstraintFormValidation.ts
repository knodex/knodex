// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { z } from "zod";
import {
  parseParameterSchema,
  canRenderAsForm,
  getParameterDefaultValues,
} from "@/lib/parse-parameter-schema";
import { buildFormSchema } from "@/lib/schema-to-zod";
import type { ConstraintTemplate, EnforcementAction } from "@/types/compliance";

/**
 * Validation schema for constraint creation form
 * Uses arrays for apiGroups and kinds to support multi-select autocomplete
 */
export const baseConstraintFormSchema = z.object({
  name: z
    .string()
    .min(1, "Name is required")
    .max(253, "Name must be 253 characters or less")
    .regex(
      /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
      "Name must be DNS-compatible: lowercase alphanumeric, may contain hyphens, cannot start/end with hyphen"
    ),
  enforcementAction: z.enum(["deny", "warn", "dryrun"]),
  matchKinds: z
    .array(
      z.object({
        apiGroups: z.array(z.string()),
        kinds: z.array(z.string()).min(1, "At least one kind is required"),
      })
    )
    .optional(),
  matchNamespaces: z.string().optional(),
  parametersRaw: z.string().optional(),
  params: z.record(z.unknown()).optional(),
});

export type ConstraintFormValues = z.infer<typeof baseConstraintFormSchema>;

function buildConstraintFormSchema(template: ConstraintTemplate) {
  const parsedSchema = parseParameterSchema(template.parameters);

  if (parsedSchema && canRenderAsForm(template.parameters)) {
    const paramsSchema = buildFormSchema(parsedSchema.properties, parsedSchema.required);

    return baseConstraintFormSchema.extend({
      params: paramsSchema.optional(),
    });
  }

  return baseConstraintFormSchema;
}

/**
 * Hook encapsulating constraint form schema building, parsed schema,
 * form schema, and default values calculation.
 */
export function useConstraintFormValidation(template: ConstraintTemplate) {
  const parsedSchema = useMemo(
    () => parseParameterSchema(template.parameters),
    [template.parameters]
  );

  const canUseFormMode = canRenderAsForm(template.parameters);

  const formSchema = useMemo(
    () => buildConstraintFormSchema(template),
    [template]
  );

  const defaultValues: ConstraintFormValues = useMemo(() => {
    const paramDefaults = parsedSchema
      ? getParameterDefaultValues(parsedSchema.properties)
      : {};

    return {
      name: "",
      enforcementAction: "deny" as EnforcementAction,
      matchKinds: [{ apiGroups: [], kinds: [] }],
      matchNamespaces: "",
      parametersRaw: template.parameters
        ? JSON.stringify(getDefaultParameterValuesLegacy(template.parameters), null, 2)
        : "{}",
      params: paramDefaults,
    };
  }, [template.parameters, parsedSchema]);

  return { parsedSchema, canUseFormMode, formSchema, defaultValues };
}

/**
 * Get default values for parameters based on schema (legacy - for raw JSON mode)
 */
function getDefaultParameterValuesLegacy(
  schema: Record<string, unknown>
): Record<string, unknown> {
  const result: Record<string, unknown> = {};

  if (schema && typeof schema === "object") {
    const properties = (schema as { properties?: Record<string, unknown> })
      .properties;
    if (properties) {
      for (const [key, prop] of Object.entries(properties)) {
        if (
          prop &&
          typeof prop === "object" &&
          "default" in prop
        ) {
          result[key] = (prop as { default: unknown }).default;
        }
      }
    }
  }

  return result;
}
