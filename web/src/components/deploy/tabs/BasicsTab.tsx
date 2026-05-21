// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo, useEffect, useId, useMemo } from "react";
import { Controller, useFormContext, useWatch } from "react-hook-form";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
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

const nativeSelectClass =
  "h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50";

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

      {/* Project — native select avoids Radix SlotClone ref-composition loop */}
      <div className="space-y-1.5">
        <Label htmlFor={projectId}>
          Project <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <select
          id={projectId}
          data-testid="project-select"
          className={nativeSelectClass}
          {...form.register("project", {
            required: "Project is required",
            onChange: () => form.setValue("namespace", ""),
          })}
        >
          <option value="">Select a project</option>
          {projects.map((p) => (
            <option key={p.name} value={p.name}>
              {p.name}{p.description ? ` — ${p.description}` : ""}
            </option>
          ))}
        </select>
        {projectError ? (
          <p className="text-xs text-[var(--status-error)]">{projectError}</p>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">
            The project that will own this deployment
          </p>
        )}
      </div>

      {/* Namespace — hidden when cluster-scoped; native select avoids Radix SlotClone ref-composition loop */}
      {!isClusterScoped && (
        <div className="space-y-1.5">
          <Label htmlFor={nsId}>
            Namespace <span className="text-[var(--brand-primary)]">*</span>
          </Label>
          <select
            id={nsId}
            data-testid="namespace-select"
            disabled={!selectedProject}
            className={nativeSelectClass}
            {...form.register("namespace", {
              required: "Namespace is required",
              validate: (value: unknown) => {
                if (typeof value !== "string" || value === "") return true;
                if (namespaces.length === 0) return true;
                return (
                  namespaces.includes(value) ||
                  "Namespace is not available in this project"
                );
              },
            })}
          >
            <option value="">{nsPlaceholder}</option>
            {namespaces.map((ns) => (
              <option key={ns} value={ns}>
                {ns}
              </option>
            ))}
          </select>
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
