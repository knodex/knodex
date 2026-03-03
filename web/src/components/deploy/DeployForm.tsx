import { useMemo } from "react";
import { useForm, FormProvider } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2, Rocket, AlertTriangle } from "lucide-react";
import type { FormSchema } from "@/types/rgd";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { FormField } from "./FormField";
import { AdvancedConfigToggle, useAdvancedConfigToggle } from "./AdvancedConfigToggle";
import { cn } from "@/lib/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useConditionalFields,
  getControllingFields,
} from "@/hooks/useConditionalFields";

interface DeployFormProps {
  schema: FormSchema;
  onSubmit: (values: Record<string, unknown>) => Promise<void>;
  isSubmitting?: boolean;
  submitError?: string | null;
  instanceName?: string;
  onInstanceNameChange?: (name: string) => void;
  namespace?: string;
  onNamespaceChange?: (namespace: string) => void;
  availableNamespaces?: string[];
  className?: string;
}

export function DeployForm({
  schema,
  onSubmit,
  isSubmitting = false,
  submitError,
  instanceName = "",
  onInstanceNameChange,
  namespace = "default",
  onNamespaceChange,
  availableNamespaces = ["default"],
  className,
}: DeployFormProps) {
  // Build Zod schema from FormSchema
  const zodSchema = useMemo(
    () => buildFormSchema(schema.properties, schema.required),
    [schema.properties, schema.required]
  );

  // Get default values
  const defaultValues = useMemo(
    () => getDefaultValues(schema.properties),
    [schema.properties]
  );

  const methods = useForm({
    resolver: zodResolver(zodSchema),
    defaultValues,
    mode: "onChange",
  });

  const {
    handleSubmit,
    formState: { errors, isValid, isDirty },
    watch,
  } = methods;

  // Watch all form values for conditional field logic
  const formValues = watch();

  // Get hidden fields based on conditional sections
  const hiddenFields = useConditionalFields(
    schema.conditionalSections,
    formValues
  );

  // Get controlling fields that should always be visible
  const controllingFields = useMemo(
    () => getControllingFields(schema.conditionalSections),
    [schema.conditionalSections]
  );

  // Advanced config toggle state
  const { isExpanded: isAdvancedExpanded, toggle: toggleAdvanced } = useAdvancedConfigToggle();

  // Check if a field is under the advanced section
  const isAdvancedField = (fieldName: string): boolean => {
    // "advanced" is the root property for advanced fields
    return fieldName === "advanced" || fieldName.startsWith("advanced.");
  };

  // Separate properties into regular and advanced
  const { regularProperties, advancedProperties } = useMemo(() => {
    const regular: Array<[string, typeof schema.properties[string]]> = [];
    const advanced: Array<[string, typeof schema.properties[string]]> = [];

    for (const [name, property] of Object.entries(schema.properties)) {
      if (isAdvancedField(name)) {
        // If this is the "advanced" object container itself, flatten its children
        // to avoid a redundant collapsible header inside AdvancedConfigToggle
        if (name === "advanced" && property.type === "object" && property.properties) {
          for (const [childName, childProp] of Object.entries(property.properties)) {
            advanced.push([`advanced.${childName}`, childProp]);
          }
        } else {
          advanced.push([name, property]);
        }
      } else {
        regular.push([name, property]);
      }
    }

    return { regularProperties: regular, advancedProperties: advanced };
  // eslint-disable-next-line react-hooks/exhaustive-deps -- schema.properties is the only relevant dependency
  }, [schema.properties]);

  // Determine if a field should be visible
  const isFieldVisible = (fieldName: string): boolean => {
    // Controlling fields are always visible
    if (controllingFields.has(fieldName)) {
      return true;
    }
    // Check if hidden by conditional section
    // hiddenFields contains resource kinds from affectedProperties (e.g., "Ingress")
    // Form field names are lowercase paths (e.g., "ingress.host")
    // We need case-insensitive prefix matching
    const fieldNameLower = fieldName.toLowerCase();
    for (const hiddenProp of hiddenFields) {
      const hiddenPropLower = hiddenProp.toLowerCase();
      // Check if field starts with the hidden property name
      // e.g., "ingress.host" starts with "ingress"
      if (fieldNameLower.startsWith(hiddenPropLower)) {
        // Make sure we're matching a full segment, not a partial match
        // e.g., "ingress" should match "ingress.host" but not "ingressRoute"
        const nextChar = fieldNameLower[hiddenPropLower.length];
        if (nextChar === undefined || nextChar === ".") {
          return false;
        }
      }
    }
    return true;
  };

  const hasErrors = Object.keys(errors).length > 0;

  const onFormSubmit = async (values: Record<string, unknown>) => {
    await onSubmit(values);
  };

  return (
    <FormProvider {...methods}>
      <form
        onSubmit={handleSubmit(onFormSubmit)}
        className={cn("space-y-6", className)}
      >
        {/* Instance Metadata Section */}
        <div className="rounded-lg border border-border bg-card p-4 space-y-4">
          <h3 className="text-sm font-medium text-foreground">Instance Details</h3>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <label htmlFor="instanceName" className="text-sm font-medium text-foreground flex items-center gap-1">
                Instance Name
                <span className="text-destructive">*</span>
              </label>
              <input
                id="instanceName"
                type="text"
                value={instanceName}
                onChange={(e) => onInstanceNameChange?.(e.target.value)}
                placeholder="my-instance"
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
              <p className="text-xs text-muted-foreground">
                Unique name for this deployment instance
              </p>
            </div>

            <div className="space-y-1.5">
              <label htmlFor="namespace" className="text-sm font-medium text-foreground flex items-center gap-1">
                Namespace
                <span className="text-destructive">*</span>
              </label>
              <Select value={namespace} onValueChange={(value) => onNamespaceChange?.(value)}>
                <SelectTrigger id="namespace" className="w-full">
                  <SelectValue placeholder="Select namespace" />
                </SelectTrigger>
                <SelectContent>
                  {availableNamespaces.map((ns) => (
                    <SelectItem key={ns} value={ns}>
                      {ns}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Target namespace for deployment
              </p>
            </div>
          </div>
        </div>

        {/* Schema Properties Section */}
        <div className="rounded-lg border border-border bg-card p-4 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium text-foreground">Configuration</h3>
            <span className="text-xs text-muted-foreground">
              {schema.group}/{schema.version}
            </span>
          </div>

          {schema.description && (
            <p className="text-sm text-muted-foreground pb-4 border-b border-border">
              {schema.description}
            </p>
          )}

          {/* Regular (non-advanced) fields */}
          <div className="space-y-4">
            {regularProperties.map(([name, property]) => {
              // Skip hidden fields
              if (!isFieldVisible(name)) {
                return null;
              }
              return (
                <FormField
                  key={name}
                  name={name}
                  property={property}
                  required={schema.required?.includes(name)}
                  deploymentNamespace={namespace}
                />
              );
            })}
          </div>

          {/* Advanced Configuration Toggle */}
          <AdvancedConfigToggle
            advancedSection={schema.advancedSection ?? null}
            isExpanded={isAdvancedExpanded}
            onToggle={toggleAdvanced}
          >
            {advancedProperties.map(([name, property]) => {
              // Skip hidden fields
              if (!isFieldVisible(name)) {
                return null;
              }
              return (
                <FormField
                  key={name}
                  name={name}
                  property={property}
                  required={schema.required?.includes(name)}
                  deploymentNamespace={namespace}
                />
              );
            })}
          </AdvancedConfigToggle>
        </div>

        {/* Error Summary */}
        {hasErrors && (
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4">
            <div className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-4 w-4" />
              <span className="text-sm font-medium">Please fix the following errors:</span>
            </div>
            <ul className="mt-2 space-y-1 text-sm text-destructive">
              {Object.entries(errors).map(([field, error]) => (
                <li key={field}>
                  <span className="font-mono">{field}</span>: {(error as { message?: string })?.message || "Invalid value"}
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Submit Error */}
        {submitError && (
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4">
            <div className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-4 w-4" />
              <span className="text-sm font-medium">Deployment failed</span>
            </div>
            <p className="mt-1 text-sm text-destructive">{submitError}</p>
          </div>
        )}

        {/* Submit Button */}
        <div className="flex justify-end gap-3">
          <button
            type="submit"
            disabled={isSubmitting || !instanceName || hasErrors}
            className={cn(
              "flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md transition-colors",
              "bg-primary text-primary-foreground hover:bg-primary/90",
              "disabled:opacity-50 disabled:cursor-not-allowed"
            )}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Deploying...
              </>
            ) : (
              <>
                <Rocket className="h-4 w-4" />
                Deploy Instance
              </>
            )}
          </button>
        </div>

        {/* Form State Debug (development only) */}
        {import.meta.env.DEV && (
          <div className="text-xs text-muted-foreground">
            <span className={isValid ? "text-status-success" : "text-destructive"}>
              {isValid ? "Valid" : "Invalid"}
            </span>
            {" | "}
            <span>{isDirty ? "Modified" : "Unchanged"}</span>
            {" | "}
            <span>{Object.keys(errors).length} errors</span>
          </div>
        )}
      </form>
    </FormProvider>
  );
}
