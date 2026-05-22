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
import type { FormSchema } from "@/types/rgd";
import type { DeploymentMode } from "@/types/deployment";
import { DeploymentModeSelector } from "@/components/deploy/DeploymentModeSelector";

interface BasicsTabProps {
  schema: FormSchema;
  /** Allowed deployment modes (from RGD annotations). */
  allowedDeploymentModes?: DeploymentMode[];
}

export const BasicsTab = memo(function BasicsTab({ schema, allowedDeploymentModes }: BasicsTabProps) {
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

  const repositoryIdValue = (useWatch({
    control: form.control,
    name: "repositoryId",
  }) as string | undefined) ?? "";

  const gitBranchValue = (useWatch({
    control: form.control,
    name: "gitBranch",
  }) as string | undefined) ?? "";

  const gitPathValue = (useWatch({
    control: form.control,
    name: "gitPath",
  }) as string | undefined) ?? "";

  // Clear GitOps-only fields when switching to direct mode.
  // NOTE: `form.unregister` is a stable function reference from RHF — safe dep.
  // Using the whole `form` object would cause a loop because FormProvider spreads
  // a new context object on every parent render, while the inner methods stay stable.
  const { unregister } = form;
  useEffect(() => {
    if (deploymentMode === "direct") {
      unregister(["repositoryId", "gitBranch", "gitPath"]);
    }
  }, [deploymentMode, unregister]);

  // Validate the current namespace against the fetched list whenever either
  // changes. zodResolver owns validation, so a Controller `rules.validate`
  // would be ignored — this useEffect restores the cross-field check that
  // existed when these were native <select> elements with RHF register().
  const currentNamespace = useWatch({ control: form.control, name: "namespace" }) as string | undefined;
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

  return (
    <div className="space-y-5" data-testid="basics-tab">
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
          aria-describedby={instanceNameError ? `${nameId}-error` : `${nameId}-hint`}
          {...form.register("instanceName", {
            validate: (value: unknown) => {
              const err = validateInstanceName(typeof value === "string" ? value : "");
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
                // setValue alone leaves the stale "required" error visible
                // until the user touches the namespace field again.
                form.clearErrors("namespace");
              }}
              onOpenChange={(open) => {
                if (!open) field.onBlur();
              }}
            >
              <SelectTrigger id={projectId} data-testid="project-select" className="h-9">
                <SelectValue placeholder="Select a project" />
              </SelectTrigger>
              <SelectContent>
                {projects.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.name}{p.description ? ` — ${p.description}` : ""}
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
                <SelectTrigger id={nsId} data-testid="namespace-select" className="h-9">
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
            <p className="text-xs text-[var(--status-error)]">{namespaceError}</p>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">
              Target namespace for this deployment
            </p>
          )}
        </div>
      )}

      {/* Deployment Mode + GitOps fields */}
      <div className="pt-2 border-t" style={{ borderColor: "rgba(255,255,255,0.06)" }}>
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

    </div>
  );
});
