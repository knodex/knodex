// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useMemo } from "react";
import { useForm, useWatch, FormProvider } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { FormSchema } from "@/types/rgd";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { orderProperties } from "@/lib/order-properties";
import { useFieldVisibility } from "@/hooks/useFieldVisibility";
import { FormField } from "./FormField";

interface ConfigureStepProps {
  schema: FormSchema;
  onValuesChange: (values: Record<string, unknown>, isValid: boolean) => void;
  deploymentNamespace?: string;
}

export function ConfigureStep({ schema, onValuesChange, deploymentNamespace }: ConfigureStepProps) {
  const zodSchema = useMemo(
    () => buildFormSchema(schema.properties, schema.required),
    [schema.properties, schema.required]
  );

  const defaultValues = useMemo(
    () => getDefaultValues(schema.properties),
    [schema.properties]
  );

  const methods = useForm({
    resolver: zodResolver(zodSchema),
    defaultValues,
    mode: "onBlur",
  });

  const {
    formState: { isValid },
    watch,
  } = methods;

  // Extract controlling field names for conditional visibility
  const controllingFieldNames = useMemo(() => {
    if (!schema.conditionalSections?.length) return [] as string[];
    const names = new Set<string>();
    for (const section of schema.conditionalSections) {
      names.add(section.controllingField.replace(/^spec\./, ""));
      if (section.rules) {
        for (const rule of section.rules) {
          names.add(rule.field.replace(/^spec\./, ""));
        }
      }
    }
    return [...names];
  }, [schema.conditionalSections]);

  // Watch controlling fields for visibility evaluation
  const watchedValues = useWatch({
    control: methods.control,
    name: controllingFieldNames,
  });

  // Build partial form values for the visibility hook
  const visibilityFormValues = useMemo(() => {
    if (!controllingFieldNames.length) return {} as Record<string, unknown>;
    const values: Record<string, unknown> = {};
    const watched = Array.isArray(watchedValues) ? watchedValues : [watchedValues];
    for (let i = 0; i < controllingFieldNames.length; i++) {
      const parts = controllingFieldNames[i].split(".");
      let current = values;
      for (let j = 0; j < parts.length - 1; j++) {
        if (!(parts[j] in current)) {
          current[parts[j]] = {};
        }
        current = current[parts[j]] as Record<string, unknown>;
      }
      current[parts[parts.length - 1]] = watched[i];
    }
    return values;
  }, [controllingFieldNames, watchedValues]);

  const { isFieldVisible } = useFieldVisibility(
    schema.conditionalSections,
    visibilityFormValues
  );

  // Trigger validation on mount so isValid reflects default values
  useEffect(() => {
    void methods.trigger();
  }, [methods]);

  // Notify parent of value/validity changes
  useEffect(() => {
    const subscription = watch((values) => {
      onValuesChange(values as Record<string, unknown>, isValid);
    });
    return () => subscription.unsubscribe();
  }, [watch, onValuesChange, isValid]);

  // Re-notify parent whenever validity changes (e.g., after mount trigger or blur)
  useEffect(() => {
    const values = methods.getValues();
    onValuesChange(values as Record<string, unknown>, isValid);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isValid]);

  const propertyEntries = useMemo(
    () => orderProperties(Object.entries(schema.properties), schema.propertyOrder),
    [schema.properties, schema.propertyOrder]
  );

  const requiredFields = useMemo(
    () => new Set(schema.required ?? []),
    [schema.required]
  );

  return (
    <FormProvider {...methods}>
      <div className="space-y-4" data-testid="configure-step">
        {propertyEntries.map(([key, prop]) => {
          if (!isFieldVisible(key)) return null;
          return (
            <FormField
              key={key}
              name={key}
              property={prop}
              required={requiredFields.has(key)}
              deploymentNamespace={deploymentNamespace}
            />
          );
        })}
      </div>
    </FormProvider>
  );
}
