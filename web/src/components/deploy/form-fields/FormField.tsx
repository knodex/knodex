// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import { useFormContext, useController } from "react-hook-form";
import { ExternalRefSelector } from "../ExternalRefSelector";
import { TextField } from "./TextField";
import { NumberField } from "./NumberField";
import { CheckboxField } from "./CheckboxField";
import { SelectField } from "./SelectField";
import { ObjectField } from "./ObjectField";
import { ArrayField } from "./ArrayField";
import { KeyValueField } from "./KeyValueField";
import { formatLabel } from "./utils";
import type { FormFieldProps } from "./types";

/**
 * Stable wrapper for Controller-based ArrayField.
 * Uses useController instead of Controller render prop to avoid new closure on every render.
 */
const ControlledArrayField = memo(function ControlledArrayField({
  name,
  label,
  description,
  property,
  required,
  error,
  depth,
  deploymentNamespace,
}: {
  name: string;
  label: string;
  description?: string;
  property: FormFieldProps["property"];
  required: boolean;
  error?: string;
  depth: number;
  deploymentNamespace?: string;
}) {
  const { field } = useController({ name });
  return (
    <ArrayField
      name={name}
      label={label}
      description={description}
      property={property}
      value={(field.value as unknown[]) || []}
      onChange={field.onChange}
      required={required}
      error={error}
      depth={depth}
      deploymentNamespace={deploymentNamespace}
    />
  );
});

/**
 * Stable wrapper for Controller-based KeyValueField.
 */
const ControlledKeyValueField = memo(function ControlledKeyValueField({
  name,
  label,
  description,
  error,
}: {
  name: string;
  label: string;
  description?: string;
  error?: string;
}) {
  const { field } = useController({ name });
  return (
    <KeyValueField
      name={name}
      label={label}
      description={description}
      value={(field.value as Record<string, string>) || {}}
      onChange={field.onChange}
      error={error}
    />
  );
});

/**
 * Main form field router component.
 * Renders the appropriate field component based on the property type.
 */
export function FormField({
  name,
  property,
  required = false,
  depth = 0,
  deploymentNamespace,
  inlineAdvancedSection,
}: FormFieldProps) {
  const {
    register,
    formState: { errors },
  } = useFormContext();

  // Get nested error
  const error = name.split(".").reduce((acc: unknown, key) => {
    if (acc && typeof acc === "object" && key in acc) {
      return (acc as Record<string, unknown>)[key];
    }
    return undefined;
  }, errors as unknown) as { message?: string } | undefined;

  const errorMessage = error?.message;
  const label = property.title || formatLabel(name.split(".").pop() || name);
  const description = property.description;

  // Render based on type
  switch (property.type) {
    case "string":
      if (property.enum && property.enum.length > 0) {
        return (
          <SelectField
            name={name}
            label={label}
            description={description}
            options={property.enum.map(String)}
            required={required}
            error={errorMessage}
            defaultValue={property.default as string}
            register={register}
          />
        );
      }
      return (
        <TextField
          name={name}
          label={label}
          description={description}
          required={required}
          error={errorMessage}
          format={property.format}
          register={register}
        />
      );

    case "integer":
    case "number":
      return (
        <NumberField
          name={name}
          label={label}
          description={description}
          required={required}
          error={errorMessage}
          min={property.minimum}
          max={property.maximum}
          isInteger={property.type === "integer"}
        />
      );

    case "boolean":
      return (
        <CheckboxField
          name={name}
          label={label}
          description={description}
          required={required}
          error={errorMessage}
          register={register}
        />
      );

    case "object":
      // Resource picker: object with externalRefSelector + autoFillFields
      if (property.externalRefSelector?.autoFillFields) {
        return (
          <ExternalRefSelector
            name={name}
            apiVersion={property.externalRefSelector.apiVersion}
            kind={property.externalRefSelector.kind}
            deploymentNamespace={deploymentNamespace}
            useInstanceNamespace={property.externalRefSelector.useInstanceNamespace}
            autoFillFields={property.externalRefSelector.autoFillFields}
            label={label}
            description={description}
            required={required}
            error={errorMessage}
          />
        );
      }
      if (property.properties && Object.keys(property.properties).length > 0) {
        return (
          <ObjectField
            name={name}
            label={label}
            description={description}
            property={property}
            required={required}
            depth={depth}
            deploymentNamespace={deploymentNamespace}
            inlineAdvancedSection={inlineAdvancedSection}
          />
        );
      }
      return (
        <ControlledKeyValueField
          name={name}
          label={label}
          description={description}
          error={errorMessage}
        />
      );

    case "array":
      return (
        <ControlledArrayField
          name={name}
          label={label}
          description={description}
          property={property}
          required={required}
          error={errorMessage}
          depth={depth}
          deploymentNamespace={deploymentNamespace}
        />
      );

    default:
      return (
        <TextField
          name={name}
          label={label}
          description={description}
          required={required}
          error={errorMessage}
          register={register}
        />
      );
  }
}
