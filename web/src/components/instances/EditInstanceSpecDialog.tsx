// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { useForm, FormProvider } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import {
  Loader2,
  AlertTriangle,
  AlertCircle,
  Save,
  GitBranch,
  Cloud,
  RefreshCw,
} from "@/lib/icons";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { FormField } from "@/components/deploy/FormField";
import { AdvancedConfigToggle } from "@/components/deploy/AdvancedConfigToggle";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { useFieldVisibility } from "@/hooks/useFieldVisibility";
import { useAdvancedFieldSplit } from "@/hooks/useAdvancedFieldSplit";
import { useRGDSchema } from "@/hooks/useRGDs";
import { useUpdateInstanceSpec } from "@/hooks/useInstances";
import type { Instance, DeploymentMode } from "@/types/rgd";
import type { GitInfo } from "@/types/rgd";
import { cn } from "@/lib/utils";

interface EditInstanceSpecDialogProps {
  instance: Instance;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

export function EditInstanceSpecDialog({
  instance,
  open,
  onOpenChange,
  onSuccess,
}: EditInstanceSpecDialogProps) {
  const { data: schemaResponse, isLoading: isLoadingSchema, error: schemaError } = useRGDSchema(
    instance.rgdName,
    instance.rgdNamespace
  );
  const updateMutation = useUpdateInstanceSpec();
  const [showConfirmation, setShowConfirmation] = useState(false);
  const [pendingValues, setPendingValues] = useState<Record<string, unknown> | null>(null);
  const [successInfo, setSuccessInfo] = useState<{ gitInfo?: GitInfo } | null>(null);

  const deploymentMode = (instance.deploymentMode || instance.labels?.["knodex.io/deployment-mode"] || "direct") as DeploymentMode;

  const handleFormSubmit = (values: Record<string, unknown>) => {
    setPendingValues(values);
    setShowConfirmation(true);
  };

  const handleConfirm = async () => {
    if (!pendingValues) return;

    try {
      const result = await updateMutation.mutateAsync({
        namespace: instance.namespace,
        kind: instance.kind,
        name: instance.name,
        request: {
          spec: pendingValues,
          resourceVersion: instance.resourceVersion,
          repositoryId: instance.gitInfo?.repositoryId,
          gitBranch: instance.gitInfo?.branch,
          gitPath: instance.gitInfo?.path,
        },
      });
      setShowConfirmation(false);
      setPendingValues(null);
      setSuccessInfo({ gitInfo: result.gitInfo });
      toast.success(`Instance "${instance.name}" spec updated`);
    } catch (err) {
      // Error is captured by updateMutation.error
      setShowConfirmation(false);
      toast.error(err instanceof Error ? err.message : "Failed to update spec");
    }
  };

  const handleClose = () => {
    if (updateMutation.isPending) return;
    onOpenChange(false);
    // Reset state after dialog animation
    setTimeout(() => {
      setShowConfirmation(false);
      setPendingValues(null);
      setSuccessInfo(null);
      updateMutation.reset();
    }, 200);
    if (successInfo) {
      onSuccess?.();
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Spec — {instance.name}</DialogTitle>
          <DialogDescription>
            Update the configuration for this {instance.kind} instance
          </DialogDescription>
        </DialogHeader>

        {/* Loading schema */}
        {isLoadingSchema && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            <span className="ml-2 text-sm text-muted-foreground">Loading schema...</span>
          </div>
        )}

        {/* Schema error */}
        {(schemaError || (schemaResponse && !schemaResponse.schema)) && (
          <div className="flex flex-col items-center justify-center py-12 gap-3">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <p className="text-sm text-muted-foreground text-center">
              {schemaError instanceof Error
                ? schemaError.message
                : schemaResponse?.error || "No CRD schema found for this RGD."}
            </p>
          </div>
        )}

        {/* Degraded mode indicator */}
        {schemaResponse?.source === "rgd-only" && schemaResponse.schema && !showConfirmation && !successInfo && (
          <div className="rounded-lg border border-status-warning/50 bg-status-warning/10 p-3">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-status-warning shrink-0" />
              <p className="text-xs text-muted-foreground">
                <span className="font-medium text-status-warning">Preview mode</span> — some validation constraints are pending CRD generation
              </p>
            </div>
          </div>
        )}

        {/* Success state */}
        {successInfo && (
          <div className="space-y-4">
            <div className="rounded-lg border border-status-success bg-status-success/10 p-4">
              <div className="flex items-center gap-3">
                <Save className="h-5 w-5 text-status-success" />
                <div>
                  <h3 className="text-sm font-medium text-foreground">
                    {deploymentMode === "direct" && "Spec updated successfully"}
                    {deploymentMode === "gitops" && "Spec update pushed to Git"}
                    {deploymentMode === "hybrid" && "Spec updated and pushed to Git"}
                  </h3>
                  {successInfo.gitInfo && (
                    <p className="text-xs text-muted-foreground mt-1">
                      {(successInfo.gitInfo.pushStatus === "completed" || successInfo.gitInfo.pushStatus === "success") ? (
                        <>Commit: <span className="font-mono">{successInfo.gitInfo.commitSha?.slice(0, 8)}</span></>
                      ) : successInfo.gitInfo.pushStatus === "failed" ? (
                        <span className="text-status-warning">
                          Applied to cluster but Git push failed: {successInfo.gitInfo.pushError}
                        </span>
                      ) : (
                        <>Git push status: {successInfo.gitInfo.pushStatus}</>
                      )}
                    </p>
                  )}
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={handleClose}>Close</Button>
            </DialogFooter>
          </div>
        )}

        {/* Confirmation step */}
        {showConfirmation && !successInfo && (
          <div className="space-y-4">
            <DeploymentModeWarning mode={deploymentMode} instance={instance} />
            <DialogFooter className="gap-2">
              <Button
                variant="outline"
                onClick={() => setShowConfirmation(false)}
                disabled={updateMutation.isPending}
              >
                Back
              </Button>
              <Button
                onClick={handleConfirm}
                disabled={updateMutation.isPending}
                variant="destructive"
              >
                {updateMutation.isPending ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin mr-1.5" />
                    {deploymentMode === "gitops" ? "Pushing..." : "Saving..."}
                  </>
                ) : (
                  <>
                    <Save className="h-4 w-4 mr-1.5" />
                    I understand, proceed
                  </>
                )}
              </Button>
            </DialogFooter>

            {/* Mutation error */}
            {updateMutation.isError && (
              <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4">
                <div className="flex items-center gap-2 text-destructive">
                  <AlertTriangle className="h-4 w-4" />
                  <span className="text-sm font-medium">Update failed</span>
                </div>
                <p className="mt-1 text-sm text-destructive">
                  {updateMutation.error instanceof Error
                    ? updateMutation.error.message
                    : "An unexpected error occurred"}
                </p>
              </div>
            )}
          </div>
        )}

        {/* Edit form */}
        {schemaResponse?.schema && !showConfirmation && !successInfo && (
          <EditSpecForm
            schema={schemaResponse.schema}
            currentSpec={instance.spec || {}}
            namespace={instance.namespace}
            onSubmit={handleFormSubmit}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function DeploymentModeWarning({ mode, instance }: { mode: DeploymentMode; instance: Instance }) {
  const gitBranch = instance.gitInfo?.branch;
  const gitPath = instance.gitInfo?.path;
  const gitRepo = instance.gitInfo?.repositoryId;

  switch (mode) {
    case "gitops":
      return (
        <div className="rounded-lg border border-status-warning bg-status-warning/10 p-4">
          <div className="flex items-start gap-3">
            <GitBranch className="h-5 w-5 text-status-warning shrink-0 mt-0.5" />
            <div>
              <h4 className="text-sm font-medium text-status-warning">
                GitOps Update
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                This change will be pushed to the Git repository
                {gitRepo && <> (<span className="font-mono text-xs">{gitRepo}</span>)</>}
                {gitBranch && <>, branch: <span className="font-mono text-xs">{gitBranch}</span></>}
                {gitPath && <>, path: <span className="font-mono text-xs">{gitPath}</span></>}
                . ArgoCD/Flux will sync the update to the cluster. The change will NOT take
                effect immediately — it depends on the sync interval. Editing the spec may also
                cause the underlying resources to be recreated.
              </p>
            </div>
          </div>
        </div>
      );
    case "hybrid":
      return (
        <div className="rounded-lg border border-blue-500/30 bg-blue-500/10 p-4">
          <div className="flex items-start gap-3">
            <RefreshCw className="h-5 w-5 text-blue-500 shrink-0 mt-0.5" />
            <div>
              <h4 className="text-sm font-medium text-blue-500">
                Hybrid Update
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                This change will be applied directly to the cluster AND pushed to Git
                {gitBranch && <> (branch: <span className="font-mono text-xs">{gitBranch}</span>)</>}
                . The underlying resources may be recreated, causing potential downtime.
                If the Git push fails, the cluster change will still be applied.
              </p>
            </div>
          </div>
        </div>
      );
    default:
      return (
        <div className="rounded-lg border border-border bg-secondary/50 p-4">
          <div className="flex items-start gap-3">
            <Cloud className="h-5 w-5 text-muted-foreground shrink-0 mt-0.5" />
            <div>
              <h4 className="text-sm font-medium text-foreground">
                Direct Update
              </h4>
              <p className="text-sm text-muted-foreground mt-1">
                Editing the spec may cause the underlying Kubernetes resources to be recreated.
                This could result in downtime or data loss.
              </p>
            </div>
          </div>
        </div>
      );
  }
}

interface EditSpecFormProps {
  schema: import("@/types/rgd").FormSchema;
  currentSpec: Record<string, unknown>;
  namespace: string;
  onSubmit: (values: Record<string, unknown>) => void;
}

function EditSpecForm({ schema, currentSpec, namespace, onSubmit }: EditSpecFormProps) {
  const zodSchema = useMemo(
    () => buildFormSchema(schema.properties, schema.required),
    [schema.properties, schema.required]
  );

  // Merge schema defaults with current instance spec (current spec takes priority)
  const defaultValues = useMemo(() => {
    const schemaDefaults = getDefaultValues(schema.properties);
    return deepMerge(schemaDefaults, currentSpec);
  }, [schema.properties, currentSpec]);

  const methods = useForm({
    resolver: zodResolver(zodSchema),
    defaultValues,
    mode: "onChange",
  });

  const {
    handleSubmit,
    formState: { errors, isDirty },
    watch,
  } = methods;

  // eslint-disable-next-line react-hooks/incompatible-library -- watch() is required for conditional field visibility
  const formValues = watch();
  const hasErrors = Object.keys(errors).length > 0;

  const { isFieldVisible } = useFieldVisibility(
    schema.conditionalSections,
    formValues
  );

  const { regularProperties, advancedProperties, globalSection, isAdvancedExpanded, toggleAdvanced } =
    useAdvancedFieldSplit(schema.properties, schema.advancedSections);

  return (
    <FormProvider {...methods}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        {/* Regular fields */}
        <div className="space-y-4">
          {regularProperties.map(([name, property]) => {
            if (!isFieldVisible(name)) return null;
            return (
              <FormField
                key={name}
                name={name}
                property={property}
                required={schema.required?.includes(name)}
                deploymentNamespace={namespace}
                inlineAdvancedSection={schema.advancedSections?.find(s => s.path === `${name}.advanced`)}
              />
            );
          })}
        </div>

        {/* Advanced fields */}
        <AdvancedConfigToggle
          advancedSection={globalSection}
          isExpanded={isAdvancedExpanded}
          onToggle={toggleAdvanced}
        >
          {advancedProperties.map(([name, property]) => {
            if (!isFieldVisible(name)) return null;
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

        {/* Error summary */}
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

        {/* Submit */}
        <DialogFooter>
          <Button
            type="submit"
            disabled={hasErrors || !isDirty}
            className={cn(!isDirty && "opacity-50")}
          >
            <Save className="h-4 w-4 mr-1.5" />
            Review Changes
          </Button>
        </DialogFooter>
      </form>
    </FormProvider>
  );
}

/**
 * Deep merge source into target. Source values override target values.
 * Only merges plain objects; arrays and primitives from source take priority.
 */
function deepMerge(
  target: Record<string, unknown>,
  source: Record<string, unknown>
): Record<string, unknown> {
  const result = { ...target };
  for (const key of Object.keys(source)) {
    const sourceVal = source[key];
    const targetVal = result[key];
    if (
      sourceVal !== null &&
      sourceVal !== undefined &&
      typeof sourceVal === "object" &&
      !Array.isArray(sourceVal) &&
      typeof targetVal === "object" &&
      targetVal !== null &&
      !Array.isArray(targetVal)
    ) {
      result[key] = deepMerge(
        targetVal as Record<string, unknown>,
        sourceVal as Record<string, unknown>
      );
    } else if (sourceVal !== undefined) {
      result[key] = sourceVal;
    }
  }
  return result;
}
