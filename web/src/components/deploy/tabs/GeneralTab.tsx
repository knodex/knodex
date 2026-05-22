// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, useEffect, useId, useMemo } from "react";
import { Controller, useFormContext, useWatch } from "react-hook-form";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjects } from "@/hooks/useProjects";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { useRepositories } from "@/hooks/useRepositories";
import { validateInstanceName } from "@/lib/validate-instance-name";
import { orderEntries } from "@/lib/order-properties";
import type { DeployTab } from "@/lib/build-tabs";
import type { FormSchema } from "@/types/rgd";
import type { DeploymentMode } from "@/types/deployment";
import { DeploymentModeSelector } from "@/components/deploy/DeploymentModeSelector";
import { renderFlattenableField } from "@/components/deploy/tabs/render-flattenable-field";

interface GeneralTabProps {
  schema: FormSchema;
  tab: DeployTab;
  /** Allowed deployment modes (from RGD annotations). */
  allowedDeploymentModes?: DeploymentMode[];
}

/**
 * The General tab merges what were two tabs in the original spec — "Basics"
 * (Knodex-owned plumbing) and an auto-generated "General" (RGD top-level
 * scalars). It also hosts the top-level `externalRef` object (folded in via
 * `build-tabs.ts`), rendered inline as a nested section.
 */
export const GeneralTab = memo(function GeneralTab({
  schema,
  tab,
  allowedDeploymentModes,
}: GeneralTabProps) {
  const form = useFormContext();
  const nameId = useId();
  const projectId = useId();
  const nsId = useId();

  const isClusterScoped = schema.isClusterScoped === true;

  const { data: projectsData } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData]);

  const selectedProject = useWatch({
    control: form.control,
    name: "project",
  }) as string | undefined;

  const { data: namespacesData } = useProjectNamespaces(selectedProject ?? "");
  const namespaces = useMemo(
    () => namespacesData?.namespaces ?? [],
    [namespacesData]
  );

  const { data: reposData, isLoading: reposLoading } = useRepositories(
    selectedProject ?? ""
  );
  const repositories = useMemo(() => reposData?.items ?? [], [reposData]);

  const deploymentMode = useWatch({
    control: form.control,
    name: "deploymentMode",
  }) as DeploymentMode | undefined;

  const repositoryIdValue =
    (useWatch({ control: form.control, name: "repositoryId" }) as
      | string
      | undefined) ?? "";

  const gitBranchValue =
    (useWatch({ control: form.control, name: "gitBranch" }) as
      | string
      | undefined) ?? "";

  const gitPathValue =
    (useWatch({ control: form.control, name: "gitPath" }) as
      | string
      | undefined) ?? "";

  // Clear GitOps-only fields when switching to direct mode.
  const { unregister } = form;
  useEffect(() => {
    if (deploymentMode === "direct") {
      unregister(["repositoryId", "gitBranch", "gitPath"]);
    }
  }, [deploymentMode, unregister]);

  // Validate the current namespace against the fetched list whenever either
  // changes. zodResolver owns validation, so a Controller `rules.validate`
  // would be ignored.
  const currentNamespace = useWatch({
    control: form.control,
    name: "namespace",
  }) as string | undefined;
  const { setError, clearErrors } = form;
  useEffect(() => {
    if (isClusterScoped) return;
    if (!currentNamespace) {
      clearErrors("namespace");
      return;
    }
    if (namespaces.length === 0) return; // not loaded yet
    if (!namespaces.includes(currentNamespace)) {
      setError("namespace", {
        type: "validate",
        message: "Namespace is not available in this project",
      });
    } else {
      clearErrors("namespace");
    }
  }, [currentNamespace, namespaces, isClusterScoped, setError, clearErrors]);

  const instanceNameError =
    form.formState.errors.instanceName?.message?.toString();
  const projectError = form.formState.errors.project?.message?.toString();
  const namespaceError = form.formState.errors.namespace?.message?.toString();

  const nsPlaceholder = !selectedProject
    ? "Select a project first"
    : namespaces.length === 0
      ? "No namespaces available"
      : "Select a namespace";

  // Schema-driven properties owned by this tab (scalars + top-level externalRef).
  const requiredSet = useMemo(
    () => new Set(tab.required ?? []),
    [tab.required]
  );
  const orderedSchemaEntries = useMemo(
    () => orderEntries(Object.entries(tab.properties ?? {}), tab.propertyOrder),
    [tab.properties, tab.propertyOrder]
  );

  const deploymentNamespace = currentNamespace ?? "";

  return (
    <div className="space-y-5" data-testid="general-tab">
      {/* Instance Name */}
      <div className="space-y-1.5">
        <Label htmlFor={nameId}>
          Instance Name <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <Input
          id={nameId}
          data-testid="instance-name-input"
          placeholder="my-instance"
          autoComplete="off"
          spellCheck={false}
          aria-describedby={
            instanceNameError ? `${nameId}-error` : `${nameId}-hint`
          }
          {...form.register("instanceName", {
            validate: (value: unknown) => {
              const err = validateInstanceName(
                typeof value === "string" ? value : ""
              );
              return err === "" ? true : err;
            },
          })}
        />
        {instanceNameError ? (
          <p id={`${nameId}-error`} className="text-xs text-[var(--status-error)]">
            {instanceNameError}
          </p>
        ) : (
          <p id={`${nameId}-hint`} className="text-xs text-[var(--text-muted)]">
            Lowercase, alphanumeric, and hyphens only
          </p>
        )}
      </div>

      {/* Project — shadcn Select via Controller (RHF register would cause Radix ref-composition loop) */}
      <div className="space-y-1.5">
        <Label htmlFor={projectId}>
          Project <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <Controller
          control={form.control}
          name="project"
          render={({ field }) => (
            <Select
              value={(field.value as string) ?? ""}
              onValueChange={(value) => {
                field.onChange(value);
                form.setValue("namespace", "");
                form.clearErrors("namespace");
              }}
              onOpenChange={(open) => {
                if (!open) field.onBlur();
              }}
            >
              <SelectTrigger
                id={projectId}
                data-testid="project-select"
                className="h-9"
              >
                <SelectValue placeholder="Select a project" />
              </SelectTrigger>
              <SelectContent>
                {projects.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.name}
                    {p.description ? ` — ${p.description}` : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        {projectError ? (
          <p className="text-xs text-[var(--status-error)]">{projectError}</p>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">
            The project that will own this deployment
          </p>
        )}
      </div>

      {/* Namespace — hidden when cluster-scoped */}
      {!isClusterScoped && (
        <div className="space-y-1.5">
          <Label htmlFor={nsId}>
            Namespace <span className="text-[var(--brand-primary)]">*</span>
          </Label>
          <Controller
            control={form.control}
            name="namespace"
            render={({ field }) => (
              <Select
                value={(field.value as string) ?? ""}
                onValueChange={field.onChange}
                onOpenChange={(open) => {
                  if (!open) field.onBlur();
                }}
                disabled={!selectedProject || namespaces.length === 0}
              >
                <SelectTrigger
                  id={nsId}
                  data-testid="namespace-select"
                  className="h-9"
                >
                  <SelectValue placeholder={nsPlaceholder} />
                </SelectTrigger>
                <SelectContent>
                  {namespaces.map((ns) => (
                    <SelectItem key={ns} value={ns}>
                      {ns}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          {namespaceError ? (
            <p className="text-xs text-[var(--status-error)]">
              {namespaceError}
            </p>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">
              Target namespace for this deployment
            </p>
          )}
        </div>
      )}

      {/* Deployment Mode + GitOps fields */}
      <div
        className="pt-2 border-t"
        style={{ borderColor: "rgba(255,255,255,0.06)" }}
      >
        <Controller
          control={form.control}
          name="deploymentMode"
          render={({ field }) => (
            <DeploymentModeSelector
              mode={(field.value as DeploymentMode) ?? "direct"}
              onModeChange={field.onChange}
              repositoryId={repositoryIdValue}
              onRepositoryChange={(v) => form.setValue("repositoryId", v)}
              gitBranch={gitBranchValue}
              onGitBranchChange={(v) => form.setValue("gitBranch", v)}
              gitPath={gitPathValue}
              onGitPathChange={(v) => form.setValue("gitPath", v)}
              repositories={repositories}
              isLoadingRepositories={reposLoading}
              allowedModes={allowedDeploymentModes}
            />
          )}
        />
      </div>

      {/* Schema-driven fields (RGD top-level scalars + folded externalRef).
        * For keys in TOP_LEVEL_GENERAL_OBJECT_KEYS (e.g. `externalRef`), we
        * flatten one level — render each child directly so the form shows the
        * child's own label (e.g. "Cluster") + picker without an outer
        * "External Ref" header or extra indentation. Nested externalRef (under
        * an object tab) is unaffected because this only applies at General. */}
      {orderedSchemaEntries.length > 0 && (
        <div
          className="pt-4 border-t space-y-4"
          style={{ borderColor: "rgba(255,255,255,0.06)" }}
          data-testid="general-tab-schema-fields"
        >
          {orderedSchemaEntries.flatMap(([key, prop]) =>
            renderFlattenableField({
              parentName: "",
              key,
              prop,
              required: requiredSet.has(key),
              deploymentNamespace,
            })
          )}
        </div>
      )}
    </div>
  );
});

