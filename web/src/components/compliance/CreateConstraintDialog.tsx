import { useState, useMemo, useCallback, useEffect } from "react";
import { useForm, FormProvider, useFieldArray, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import {
  Loader2,
  Plus,
  Trash2,
  AlertTriangle,
  Code,
  Settings,
  Target,
  Copy,
  Check,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { useCreateConstraint } from "@/hooks/useCompliance";
import { isAlreadyExists } from "@/api/compliance";
import { ParameterFormSection } from "./ParameterFormSection";
import {
  parseParameterSchema,
  canRenderAsForm,
  getParameterDefaultValues,
} from "@/lib/parse-parameter-schema";
import { buildFormSchema } from "@/lib/schema-to-zod";
import type { ConstraintTemplate, EnforcementAction } from "@/types/compliance";
import { cn } from "@/lib/utils";
import { ApiGroupSelector } from "./ApiGroupSelector";
import { KindSelector } from "./KindSelector";
import { getApiGroupValue } from "@/api/apiResources";

/**
 * Validation schema for constraint creation form
 * Uses arrays for apiGroups and kinds to support multi-select autocomplete
 */
const baseConstraintFormSchema = z.object({
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
        apiGroups: z.array(z.string()), // Array of API group names (e.g., ["core", "apps"])
        kinds: z.array(z.string()).min(1, "At least one kind is required"), // Array of kind names (e.g., ["Pod", "Deployment"])
      })
    )
    .optional(),
  matchNamespaces: z.string().optional(),
  // Raw JSON parameters for fallback mode
  parametersRaw: z.string().optional(),
  // Structured parameters for form mode (dynamic, added in component)
  params: z.record(z.unknown()).optional(),
});

type ConstraintFormValues = z.infer<typeof baseConstraintFormSchema>;

/**
 * Build the complete form schema including dynamic parameter validation
 */
function buildConstraintFormSchema(template: ConstraintTemplate) {
  const parsedSchema = parseParameterSchema(template.parameters);

  if (parsedSchema && canRenderAsForm(template.parameters)) {
    // Build dynamic schema for params field
    const paramsSchema = buildFormSchema(parsedSchema.properties, parsedSchema.required);

    return baseConstraintFormSchema.extend({
      params: paramsSchema.optional(),
    });
  }

  return baseConstraintFormSchema;
}

interface CreateConstraintDialogProps {
  template: ConstraintTemplate;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: (constraintName: string) => void;
}

/**
 * Dialog for creating a new Constraint from a ConstraintTemplate
 * AC-FORM-01: Form displays name, enforcement action, match rules, parameters
 * AC-FORM-02: Name field validates DNS-compatible naming
 * AC-FORM-03: Enforcement action selector with deny/warn/dryrun options
 * AC-FORM-04: Match rules editor for kinds and namespaces
 * AC-FORM-05: Parameter form fields based on template schema
 * AC-FORM-06: Form validation with clear error messages
 * AC-FORM-07: Submit button disabled until form is valid
 * AC-FORM-08: Loading state during submission
 */
export function CreateConstraintDialog({
  template,
  open,
  onOpenChange,
  onSuccess,
}: CreateConstraintDialogProps) {
  const [activeTab, setActiveTab] = useState("basic");
  const [useFormMode, setUseFormMode] = useState(true);
  const createConstraint = useCreateConstraint();

  // Parse the parameter schema
  const parsedSchema = useMemo(
    () => parseParameterSchema(template.parameters),
    [template.parameters]
  );

  const canUseFormMode = canRenderAsForm(template.parameters);

  // Build dynamic form schema
  const formSchema = useMemo(
    () => buildConstraintFormSchema(template),
    [template]
  );

  // Calculate default values including structured params
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

  const methods = useForm<ConstraintFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues,
    mode: "onChange",
  });

  // Reset form when template changes
  useEffect(() => {
    methods.reset(defaultValues);
  }, [template.name, defaultValues, methods]);

  const {
    register,
    handleSubmit,
    control,
    watch,
    reset,
    setValue,
    formState: { errors, isValid, isSubmitting },
  } = methods;

  const { fields, append, remove } = useFieldArray({
    control,
    name: "matchKinds",
  });

  const formValues = watch();

  // Generate YAML preview - use structured params when in form mode
  const yamlPreview = useMemo(() => {
    const effectiveParameters = canUseFormMode && useFormMode && formValues.params
      ? cleanParameters(formValues.params)
      : formValues.parametersRaw;
    return generateYamlPreview(template, formValues, effectiveParameters);
  }, [template, formValues, canUseFormMode, useFormMode]);

  const handleClose = useCallback(() => {
    reset(defaultValues);
    createConstraint.reset();
    setActiveTab("basic");
    onOpenChange(false);
  }, [reset, defaultValues, createConstraint, onOpenChange]);

  const onSubmit = async (data: ConstraintFormValues) => {
    try {
      // Determine parameters from either form mode or raw JSON mode
      let parameters: Record<string, unknown> | undefined;

      if (canUseFormMode && useFormMode && data.params) {
        // Use structured form values
        parameters = cleanParameters(data.params);
      } else if (data.parametersRaw && data.parametersRaw.trim() !== "{}") {
        // Parse raw JSON
        try {
          parameters = JSON.parse(data.parametersRaw);
        } catch {
          methods.setError("parametersRaw", {
            type: "manual",
            message: "Invalid JSON format",
          });
          return;
        }
      }

      // Build match rules
      const match = buildMatchRules(data);

      const result = await createConstraint.mutateAsync({
        name: data.name,
        templateName: template.name,
        enforcementAction: data.enforcementAction,
        match: match || undefined,
        parameters,
      });

      toast.success(`Constraint "${result.name}" created successfully`);
      onSuccess?.(result.name);
      handleClose();
    } catch (error) {
      if (isAlreadyExists(error)) {
        methods.setError("name", {
          type: "manual",
          message: `A constraint named "${data.name}" already exists`,
        });
      }
      // Other errors are handled by the mutation
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            Create Constraint from {template.kind}
          </DialogTitle>
          <DialogDescription>
            Create a new constraint using the {template.name} template.
            {template.description && ` ${template.description}`}
          </DialogDescription>
        </DialogHeader>

        <FormProvider {...methods}>
          <form
            onSubmit={handleSubmit(onSubmit)}
            className="flex-1 overflow-hidden flex flex-col"
          >
            <Tabs
              value={activeTab}
              onValueChange={setActiveTab}
              className="flex-1 flex flex-col overflow-hidden"
            >
              <TabsList className="grid w-full grid-cols-4">
                <TabsTrigger value="basic" className="flex items-center gap-1">
                  <Settings className="h-3.5 w-3.5" />
                  Basic
                </TabsTrigger>
                <TabsTrigger value="match" className="flex items-center gap-1">
                  <Target className="h-3.5 w-3.5" />
                  Match Rules
                </TabsTrigger>
                <TabsTrigger
                  value="parameters"
                  className="flex items-center gap-1"
                >
                  <Settings className="h-3.5 w-3.5" />
                  Parameters
                </TabsTrigger>
                <TabsTrigger value="preview" className="flex items-center gap-1">
                  <Code className="h-3.5 w-3.5" />
                  YAML Preview
                </TabsTrigger>
              </TabsList>

              <div className="flex-1 overflow-y-auto p-4">
                {/* Basic Tab */}
                <TabsContent value="basic" className="mt-0 space-y-4">
                  {/* Constraint Name */}
                  <div className="space-y-2">
                    <Label htmlFor="name" className="flex items-center gap-1">
                      Constraint Name
                      <span className="text-destructive">*</span>
                    </Label>
                    <Input
                      id="name"
                      {...register("name")}
                      placeholder="my-constraint"
                      data-testid="constraint-name-input"
                      className={cn(errors.name && "border-destructive")}
                    />
                    {errors.name ? (
                      <p className="text-xs text-destructive">
                        {errors.name.message}
                      </p>
                    ) : (
                      <p className="text-xs text-muted-foreground">
                        DNS-compatible name: lowercase letters, numbers, and
                        hyphens only
                      </p>
                    )}
                  </div>

                  {/* Enforcement Action */}
                  <div className="space-y-2">
                    <Label className="flex items-center gap-1">
                      Enforcement Action
                      <span className="text-destructive">*</span>
                    </Label>
                    <Select
                      value={formValues.enforcementAction}
                      onValueChange={(value) =>
                        setValue(
                          "enforcementAction",
                          value as EnforcementAction
                        )
                      }
                    >
                      <SelectTrigger data-testid="enforcement-action-select">
                        <SelectValue placeholder="Select enforcement action" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="deny">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant="outline"
                              className="bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900"
                            >
                              deny
                            </Badge>
                            <span className="text-muted-foreground">
                              Block violating resources
                            </span>
                          </div>
                        </SelectItem>
                        <SelectItem value="warn">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant="outline"
                              className="bg-yellow-50 text-yellow-700 border-yellow-200 dark:bg-yellow-950/30 dark:text-yellow-400 dark:border-yellow-900"
                            >
                              warn
                            </Badge>
                            <span className="text-muted-foreground">
                              Warn but allow resources
                            </span>
                          </div>
                        </SelectItem>
                        <SelectItem value="dryrun">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant="outline"
                              className="bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900"
                            >
                              dryrun
                            </Badge>
                            <span className="text-muted-foreground">
                              Log only, no enforcement
                            </span>
                          </div>
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">
                      Controls how violations are handled
                    </p>
                  </div>

                  {/* Template Info */}
                  <div className="rounded-lg border bg-muted/50 p-4 space-y-2">
                    <p className="text-sm font-medium">Template Information</p>
                    <div className="grid grid-cols-2 gap-2 text-sm">
                      <div>
                        <span className="text-muted-foreground">Template:</span>{" "}
                        {template.name}
                      </div>
                      <div>
                        <span className="text-muted-foreground">Kind:</span>{" "}
                        {template.kind}
                      </div>
                    </div>
                  </div>
                </TabsContent>

                {/* Match Rules Tab */}
                <TabsContent value="match" className="mt-0 space-y-4">
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <Label>Resource Kinds to Match</Label>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => append({ apiGroups: [], kinds: [] })}
                        data-testid="add-match-kind-btn"
                      >
                        <Plus className="h-3.5 w-3.5 mr-1" />
                        Add Kind
                      </Button>
                    </div>

                    {fields.length === 0 ? (
                      <p className="text-sm text-muted-foreground text-center py-4 border rounded-lg">
                        No match rules configured. The constraint will not match
                        any resources.
                      </p>
                    ) : (
                      <div className="space-y-3">
                        {fields.map((field, index) => (
                          <div
                            key={field.id}
                            className="flex gap-2 items-start p-3 border rounded-lg"
                          >
                            <div className="flex-1 space-y-3">
                              <div className="space-y-2">
                                <Label
                                  htmlFor={`matchKinds.${index}.apiGroups`}
                                  className="text-xs"
                                >
                                  API Groups
                                </Label>
                                <Controller
                                  control={control}
                                  name={`matchKinds.${index}.apiGroups`}
                                  render={({ field: controllerField }) => (
                                    <ApiGroupSelector
                                      value={controllerField.value}
                                      onChange={controllerField.onChange}
                                      placeholder="Select API groups..."
                                      data-testid={`api-groups-selector-${index}`}
                                    />
                                  )}
                                />
                                <p className="text-xs text-muted-foreground">
                                  Select &quot;core&quot; for the core API group (Pods, Services, etc.)
                                </p>
                              </div>
                              <div className="space-y-2">
                                <Label
                                  htmlFor={`matchKinds.${index}.kinds`}
                                  className="text-xs"
                                >
                                  Kinds
                                </Label>
                                <Controller
                                  control={control}
                                  name={`matchKinds.${index}.kinds`}
                                  render={({ field: controllerField }) => (
                                    <KindSelector
                                      value={controllerField.value}
                                      onChange={controllerField.onChange}
                                      apiGroups={formValues.matchKinds?.[index]?.apiGroups ?? []}
                                      placeholder="Select kinds..."
                                      data-testid={`kinds-selector-${index}`}
                                    />
                                  )}
                                />
                                {errors.matchKinds?.[index]?.kinds && (
                                  <p className="text-xs text-destructive">
                                    {errors.matchKinds[index].kinds?.message}
                                  </p>
                                )}
                              </div>
                            </div>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              onClick={() => remove(index)}
                              className="text-muted-foreground hover:text-destructive mt-6"
                              data-testid={`remove-match-kind-${index}`}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </div>
                        ))}
                      </div>
                    )}

                    {/* Namespace Filter */}
                    <div className="space-y-2 pt-4 border-t">
                      <Label htmlFor="matchNamespaces">
                        Namespace Filter (optional)
                      </Label>
                      <Input
                        id="matchNamespaces"
                        {...register("matchNamespaces")}
                        placeholder="e.g., default, production (comma-separated, empty = all namespaces)"
                        data-testid="namespaces-input"
                      />
                      <p className="text-xs text-muted-foreground">
                        Leave empty to match resources in all namespaces
                      </p>
                    </div>
                  </div>
                </TabsContent>

                {/* Parameters Tab */}
                <TabsContent value="parameters" className="mt-0 space-y-4">
                  <ParameterFormSection
                    schema={template.parameters}
                    rawValue={formValues.parametersRaw || "{}"}
                    onRawChange={(value) => setValue("parametersRaw", value)}
                    fieldNamePrefix="params"
                    rawError={errors.parametersRaw?.message}
                    useFormMode={useFormMode}
                    onModeChange={setUseFormMode}
                  />
                </TabsContent>

                {/* YAML Preview Tab */}
                <TabsContent value="preview" className="mt-0">
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label>Generated Constraint YAML</Label>
                      <CopyButton text={yamlPreview} />
                    </div>
                    <div className="rounded-lg border bg-slate-950 text-slate-50 p-4 overflow-x-auto max-h-[400px]">
                      <pre className="text-sm font-mono whitespace-pre">
                        <code>{yamlPreview}</code>
                      </pre>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      This is the Kubernetes resource that will be created.
                    </p>
                  </div>
                </TabsContent>
              </div>
            </Tabs>

            {/* Error Summary */}
            {createConstraint.isError && (
              <div className="mx-4 mb-4 rounded-lg border border-destructive/50 bg-destructive/10 p-4">
                <div className="flex items-center gap-2 text-destructive">
                  <AlertTriangle className="h-4 w-4" />
                  <span className="text-sm font-medium">
                    Failed to create constraint
                  </span>
                </div>
                <p className="mt-1 text-sm text-destructive">
                  {createConstraint.error instanceof Error
                    ? createConstraint.error.message
                    : "An unexpected error occurred"}
                </p>
              </div>
            )}

            <DialogFooter className="px-4 pb-4">
              <Button type="button" variant="outline" onClick={handleClose}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!isValid || isSubmitting || createConstraint.isPending}
                data-testid="create-constraint-submit"
              >
                {createConstraint.isPending ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Creating...
                  </>
                ) : (
                  <>
                    <Plus className="h-4 w-4 mr-2" />
                    Create Constraint
                  </>
                )}
              </Button>
            </DialogFooter>
          </form>
        </FormProvider>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Get default values for parameters based on schema (legacy - for raw JSON mode)
 */
function getDefaultParameterValuesLegacy(
  schema: Record<string, unknown>
): Record<string, unknown> {
  const result: Record<string, unknown> = {};

  // If schema has properties, extract defaults
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

/**
 * Clean parameters object by removing undefined/empty/NaN values
 */
function cleanParameters(params: Record<string, unknown>): Record<string, unknown> {
  const result: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(params)) {
    // Skip undefined values
    if (value === undefined) continue;

    // Skip empty strings
    if (value === "") continue;

    // Skip NaN values (from empty number inputs)
    if (typeof value === "number" && isNaN(value)) continue;

    // Skip empty arrays
    if (Array.isArray(value) && value.length === 0) continue;

    // Recursively clean nested objects
    if (value !== null && typeof value === "object" && !Array.isArray(value)) {
      const cleaned = cleanParameters(value as Record<string, unknown>);
      if (Object.keys(cleaned).length > 0) {
        result[key] = cleaned;
      }
    } else {
      result[key] = value;
    }
  }

  return result;
}

/**
 * Build match rules from form data
 * Converts form arrays to the API format, with "core" display names converted to empty strings
 */
function buildMatchRules(data: ConstraintFormValues) {
  const hasKinds =
    data.matchKinds &&
    data.matchKinds.some((mk) => mk.kinds.length > 0);
  const hasNamespaces =
    data.matchNamespaces && data.matchNamespaces.trim() !== "";

  if (!hasKinds && !hasNamespaces) {
    return null;
  }

  const match: {
    kinds?: Array<{ apiGroups: string[]; kinds: string[] }>;
    namespaces?: string[];
  } = {};

  if (hasKinds) {
    match.kinds = data.matchKinds
      ?.filter((mk) => mk.kinds.length > 0)
      .map((mk) => ({
        // Convert "core" display name back to empty string for API
        apiGroups: mk.apiGroups.map((g) => getApiGroupValue(g)),
        kinds: mk.kinds,
      }));
  }

  if (hasNamespaces) {
    match.namespaces = data.matchNamespaces
      ?.split(",")
      .map((ns) => ns.trim())
      .filter(Boolean);
  }

  return match;
}

/**
 * Generate YAML preview of the constraint
 */
function generateYamlPreview(
  template: ConstraintTemplate,
  values: ConstraintFormValues,
  effectiveParameters: Record<string, unknown> | string | undefined
): string {
  const lines: string[] = [];

  lines.push(`apiVersion: constraints.gatekeeper.sh/v1beta1`);
  lines.push(`kind: ${template.kind}`);
  lines.push(`metadata:`);
  lines.push(`  name: ${values.name || "<name>"}`);
  lines.push(`spec:`);
  lines.push(`  enforcementAction: ${values.enforcementAction}`);

  // Match rules
  const match = buildMatchRules(values);
  if (match) {
    lines.push(`  match:`);
    if (match.kinds && match.kinds.length > 0) {
      lines.push(`    kinds:`);
      for (const kind of match.kinds) {
        lines.push(`      - apiGroups:`);
        if (kind.apiGroups.length === 0) {
          lines.push(`          - ""`);
        } else {
          for (const ag of kind.apiGroups) {
            lines.push(`          - "${ag}"`);
          }
        }
        lines.push(`        kinds:`);
        for (const k of kind.kinds) {
          lines.push(`          - ${k}`);
        }
      }
    }
    if (match.namespaces && match.namespaces.length > 0) {
      lines.push(`    namespaces:`);
      for (const ns of match.namespaces) {
        lines.push(`      - ${ns}`);
      }
    }
  }

  // Parameters - handle both structured object and raw JSON string
  let params: Record<string, unknown> | undefined;
  if (typeof effectiveParameters === "object" && effectiveParameters !== null) {
    params = effectiveParameters;
  } else if (typeof effectiveParameters === "string" && effectiveParameters.trim() !== "{}") {
    try {
      params = JSON.parse(effectiveParameters);
    } catch {
      lines.push(`  # Invalid JSON in parameters`);
      return lines.join("\n");
    }
  }

  if (params && Object.keys(params).length > 0) {
    lines.push(`  parameters:`);
    const yamlParams = jsonToYaml(params, 4);
    lines.push(yamlParams);
  }

  return lines.join("\n");
}

/**
 * Convert JSON object to YAML string with indentation
 */
function jsonToYaml(obj: unknown, indent: number): string {
  const spaces = " ".repeat(indent);

  if (obj === null || obj === undefined) {
    return `${spaces}~`;
  }

  if (typeof obj === "string") {
    return obj.includes("\n") ? `|\n${obj.split("\n").map((l) => spaces + "  " + l).join("\n")}` : `"${obj}"`;
  }

  if (typeof obj === "number" || typeof obj === "boolean") {
    return String(obj);
  }

  if (Array.isArray(obj)) {
    if (obj.length === 0) return "[]";
    return obj
      .map((item) => {
        const value = jsonToYaml(item, indent + 2);
        if (typeof item === "object" && item !== null) {
          return `${spaces}- ${value.trim().replace(/^\s+/, "")}`;
        }
        return `${spaces}- ${value}`;
      })
      .join("\n");
  }

  if (typeof obj === "object") {
    const entries = Object.entries(obj as Record<string, unknown>);
    if (entries.length === 0) return "{}";
    return entries
      .map(([key, value]) => {
        const yamlValue = jsonToYaml(value, indent + 2);
        if (typeof value === "object" && value !== null && !Array.isArray(value)) {
          return `${spaces}${key}:\n${yamlValue}`;
        }
        if (Array.isArray(value)) {
          return `${spaces}${key}:\n${yamlValue}`;
        }
        return `${spaces}${key}: ${yamlValue}`;
      })
      .join("\n");
  }

  return String(obj);
}

/**
 * Copy to clipboard button with feedback
 */
function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy to clipboard");
    }
  };

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={handleCopy}
      className="flex items-center gap-1"
      data-testid="copy-yaml-btn"
    >
      {copied ? (
        <>
          <Check className="h-3.5 w-3.5" />
          Copied
        </>
      ) : (
        <>
          <Copy className="h-3.5 w-3.5" />
          Copy YAML
        </>
      )}
    </Button>
  );
}

export default CreateConstraintDialog;
