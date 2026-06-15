// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect } from "react";
import { useForm, FormProvider } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Loader2,
  Plus,
  AlertTriangle,
  Code,
  Settings,
  Target,
} from "@/lib/icons";
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
import { ParameterFormSection } from "./ParameterFormSection";
import { EnforcementActionSelector } from "./EnforcementActionSelector";
import { MatchRulesEditor } from "./MatchRulesEditor";
import { YamlPreviewPanel } from "./YamlPreviewPanel";
import { useConstraintFormValidation } from "./hooks/useConstraintFormValidation";
import type { ConstraintFormValues } from "./hooks/useConstraintFormValidation";
import { useYamlPreview } from "./hooks/useYamlPreview";
import { useConstraintSubmission } from "./hooks/useConstraintSubmission";
import type { ConstraintTemplate, EnforcementAction } from "@/types/compliance";
import { cn } from "@/lib/utils";

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

  const { canUseFormMode, formSchema, defaultValues } =
    useConstraintFormValidation(template);

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
    watch,
    reset,
    setValue,
    formState: { errors, isValid, isSubmitting },
  } = methods;

  // eslint-disable-next-line react-hooks/incompatible-library -- React Hook Form's watch() is inherently non-memoizable
  const formValues = watch();

  const yamlPreview = useYamlPreview(template, formValues, canUseFormMode, useFormMode);

  const { createConstraint, onSubmit: submitConstraint } = useConstraintSubmission(
    template,
    methods,
    canUseFormMode,
    useFormMode,
    onSuccess
  );

  const handleClose = useCallback(() => {
    reset(defaultValues);
    createConstraint.reset();
    setActiveTab("basic");
    onOpenChange(false);
  }, [reset, defaultValues, createConstraint, onOpenChange]);

  const onSubmit = useCallback(async (data: ConstraintFormValues) => {
    const success = await submitConstraint(data);
    if (success) {
      handleClose();
    }
  }, [submitConstraint, handleClose]);

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
                  <EnforcementActionSelector
                    value={formValues.enforcementAction as EnforcementAction}
                    onChange={(value) => setValue("enforcementAction", value)}
                  />

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
                  <MatchRulesEditor />
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
                  <YamlPreviewPanel yamlPreview={yamlPreview} />
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

export default CreateConstraintDialog;
