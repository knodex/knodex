// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo } from "react";
import { useFormContext } from "react-hook-form";
import { Code, FormInput, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { FormField } from "@/components/deploy/form-fields/FormField";
import {
  parseParameterSchema,
  canRenderAsForm,
} from "@/lib/parse-parameter-schema";
import type { FormProperty } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { logger } from "@/lib/logger";

interface ParameterFormSectionProps {
  /** OpenAPI schema from ConstraintTemplate.parameters */
  schema: Record<string, unknown> | undefined;
  /** Current parameter values as JSON string (for raw mode) */
  rawValue: string;
  /** Callback when raw JSON value changes */
  onRawChange: (value: string) => void;
  /** Field name prefix for form fields (e.g., "parameters") */
  fieldNamePrefix?: string;
  /** Error message for raw JSON validation */
  rawError?: string;
  /** Controlled mode - whether to use form or raw JSON */
  useFormMode?: boolean;
  /** Callback when mode changes */
  onModeChange?: (useFormMode: boolean) => void;
}

/**
 * ParameterFormSection renders dynamic form fields based on a ConstraintTemplate's
 * parameter schema. It supports toggling between form mode (structured fields) and
 * raw JSON mode for advanced users.
 *
 * AC-PARAM-01: Form fields are dynamically rendered based on template schema
 * AC-PARAM-02: Supports string, number, boolean, array, and object types
 * AC-PARAM-03: Raw JSON toggle available for advanced users
 * AC-PARAM-04: Form validation based on schema constraints
 */
export function ParameterFormSection({
  schema,
  rawValue,
  onRawChange,
  fieldNamePrefix = "params",
  rawError,
  useFormMode: controlledUseFormMode,
  onModeChange,
}: ParameterFormSectionProps) {
  const [internalUseRawJson, setInternalUseRawJson] = useState(false);
  const { register } = useFormContext();

  // Use controlled mode if provided, otherwise use internal state
  const useRawJson = controlledUseFormMode !== undefined ? !controlledUseFormMode : internalUseRawJson;
  const setUseRawJson = (value: boolean) => {
    if (onModeChange) {
      onModeChange(!value);
    } else {
      setInternalUseRawJson(value);
    }
  };

  // Parse the schema to determine if we can render form fields
  const parsedSchema = useMemo(() => {
    if (!schema) return null;
    const result = parseParameterSchema(schema);
    logger.debug('[ParameterFormSection] parseParameterSchema result:', result);
    return result;
  }, [schema]);

  const canUseForm = useMemo(() => {
    const result = canRenderAsForm(schema);
    logger.debug('[ParameterFormSection] canRenderAsForm result:', result, 'schema:', schema);
    return result;
  }, [schema]);

  // If no schema or empty schema, show no parameters message
  if (!schema || Object.keys(schema).length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        <Settings className="h-8 w-8 mx-auto mb-2 opacity-50" />
        <p>This template does not require any parameters.</p>
      </div>
    );
  }

  // If schema can't be rendered as form, force raw JSON mode
  if (!canUseForm || !parsedSchema) {
    return (
      <div data-testid="params-json-mode" data-can-use-form={String(canUseForm)} data-has-parsed-schema={String(!!parsedSchema)}>
        <RawJsonEditor
          schema={schema}
          value={rawValue}
          onChange={onRawChange}
          error={rawError}
          register={register}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4" data-testid="params-form-mode" data-can-use-form={String(canUseForm)} data-has-parsed-schema={String(!!parsedSchema)}>
      {/* Mode Toggle */}
      <div className="flex items-center justify-between">
        <Label className="text-sm font-medium">Parameter Values</Label>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant={useRawJson ? "outline" : "secondary"}
            size="sm"
            onClick={() => setUseRawJson(false)}
            className="flex items-center gap-1"
            data-testid="form-mode-btn"
          >
            <FormInput className="h-3.5 w-3.5" />
            Form
          </Button>
          <Button
            type="button"
            variant={useRawJson ? "secondary" : "outline"}
            size="sm"
            onClick={() => setUseRawJson(true)}
            className="flex items-center gap-1"
            data-testid="raw-json-mode-btn"
          >
            <Code className="h-3.5 w-3.5" />
            JSON
          </Button>
        </div>
      </div>

      {useRawJson ? (
        <RawJsonEditor
          schema={schema}
          value={rawValue}
          onChange={onRawChange}
          error={rawError}
          register={register}
        />
      ) : (
        <DynamicFormFields
          properties={parsedSchema.properties}
          required={parsedSchema.required}
          fieldNamePrefix={fieldNamePrefix}
        />
      )}
    </div>
  );
}

/**
 * Dynamic form fields rendered from parsed schema
 */
interface DynamicFormFieldsProps {
  properties: Record<string, FormProperty>;
  required: string[];
  fieldNamePrefix: string;
}

function DynamicFormFields({
  properties,
  required,
  fieldNamePrefix,
}: DynamicFormFieldsProps) {
  if (Object.keys(properties).length === 0) {
    return (
      <div className="text-center py-4 text-muted-foreground border rounded-lg">
        <p className="text-sm">No parameters to configure.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {Object.entries(properties).map(([key, property]) => (
        <FormField
          key={key}
          name={`${fieldNamePrefix}.${key}`}
          property={property}
          required={required.includes(key)}
          depth={0}
        />
      ))}
    </div>
  );
}

/**
 * Raw JSON editor with schema reference
 */
interface RawJsonEditorProps {
  schema: Record<string, unknown>;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  register: ReturnType<typeof useFormContext>["register"];
}

function RawJsonEditor({ schema, value, onChange, error }: RawJsonEditorProps) {
  return (
    <div className="space-y-4">
      {/* Schema Reference */}
      <div className="rounded-lg border bg-muted/50 p-4">
        <p className="text-sm font-medium mb-2">Parameter Schema</p>
        <pre className="text-xs overflow-x-auto max-h-[150px]">
          <code>{JSON.stringify(schema, null, 2)}</code>
        </pre>
      </div>

      {/* JSON Editor */}
      <div className="space-y-2">
        <Label htmlFor="parameters-raw">Parameter Values (JSON)</Label>
        <Textarea
          id="parameters-raw"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="{}"
          className={cn(
            "font-mono text-sm min-h-[200px]",
            error && "border-destructive"
          )}
          data-testid="parameters-raw-input"
        />
        {error && (
          <p className="text-xs text-destructive">{error}</p>
        )}
        <p className="text-xs text-muted-foreground">
          Enter parameter values as JSON. These values are passed to the Rego
          policy.
        </p>
      </div>
    </div>
  );
}

export default ParameterFormSection;
