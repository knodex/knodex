// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useFormContext, Controller } from "react-hook-form";
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
 * Main form field router component.
 * Renders the appropriate field component based on the property type.
 */
export function FormField({
  name,
  property,
  required = false,
  depth = 0,
  deploymentNamespace,
}: FormFieldProps) {
  const {
    register,
    formState: { errors },
    control,
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
          register={register}
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
          />
        );
      }
      return (
        <Controller
          name={name}
          control={control}
          render={({ field }) => (
            <KeyValueField
              name={name}
              label={label}
              description={description}
              value={(field.value as Record<string, string>) || {}}
              onChange={field.onChange}
              error={errorMessage}
            />
          )}
        />
      );

    case "array":
      return (
        <Controller
          name={name}
          control={control}
          render={({ field }) => (
            <ArrayField
              name={name}
              label={label}
              description={description}
              property={property}
              value={(field.value as unknown[]) || []}
              onChange={field.onChange}
              required={required}
              error={errorMessage}
              depth={depth}
              deploymentNamespace={deploymentNamespace}
            />
          )}
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
