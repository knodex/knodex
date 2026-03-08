// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/* eslint-disable react-hooks/set-state-in-effect, react-hooks/incompatible-library */
import { useState, useMemo, useEffect } from "react";
import { useForm, FormProvider } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowLeft,
  Loader2,
  Rocket,
  AlertTriangle,
  AlertCircle,
  CheckCircle2,
  FileCode,
  Box,
  Settings,
  FolderKanban,
  ShieldAlert,
} from "lucide-react";
import type { CatalogRGD, FormSchema, DeploymentMode } from "@/types/rgd";
import type { RepositoryConfig } from "@/types/repository";
import type { Project } from "@/types/project";
import { useRGDSchema, useCreateInstance } from "@/hooks/useRGDs";
import { useRepositories } from "@/hooks/useRepositories";
import { useProjects } from "@/hooks/useProjects";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { useCanI } from "@/hooks/useCanI";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { FormField } from "./FormField";
import { YAMLPreview } from "./YAMLPreview";
import { DeploymentModeSelector } from "./DeploymentModeSelector";
import { AdvancedConfigToggle, useAdvancedConfigToggle } from "./AdvancedConfigToggle";
import { cn } from "@/lib/utils";
import { useFieldVisibility } from "@/hooks/useFieldVisibility";

interface DeployPageProps {
  rgd: CatalogRGD;
  onBack: () => void;
  onDeploySuccess?: (instanceName: string, namespace: string) => void;
}

export function DeployPage({ rgd, onBack, onDeploySuccess }: DeployPageProps) {
  const { data: schemaResponse, isLoading, error } = useRGDSchema(rgd.name, rgd.namespace);
  const createInstanceMutation = useCreateInstance();
  const [instanceName, setInstanceName] = useState("");
  const [selectedProjectId, setSelectedProjectId] = useState("");
  const [namespace, setNamespace] = useState("");
  const [deploymentMode, setDeploymentMode] = useState<DeploymentMode>("direct");
  const [repositoryId, setRepositoryId] = useState("");
  const [gitBranch, setGitBranch] = useState("");
  const [gitPath, setGitPath] = useState("");
  const [submitSuccess, setSubmitSuccess] = useState(false);

  // Fetch repositories for deployment mode selector, filtered by selected project
  const { data: repositoriesData, isLoading: isLoadingRepos, error: reposError } = useRepositories(selectedProjectId || undefined);
  const repositories = useMemo(() => repositoriesData?.items ?? [], [repositoriesData]);

  // Real-time permission check via backend Casbin enforcer
  // Check if user can create instances in the selected project
  const { allowed: canDeployInProject, isLoading: isLoadingPermission, isError: isErrorPermission } = useCanI(
    'instances',
    'create',
    selectedProjectId || '-'
  );

  // Fetch available projects
  const { data: projectsData, isLoading: isLoadingProjects } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData]);

  // Fetch real namespaces for the selected project from the API
  // This returns actual K8s namespaces that match the project's destination patterns
  const {
    data: namespacesData,
    isLoading: isLoadingNamespaces
  } = useProjectNamespaces(selectedProjectId);

  // Get allowed namespaces from API response
  const allowedNamespaces = useMemo(() => namespacesData?.namespaces ?? [], [namespacesData]);

  // Select first project by default when projects load
  useEffect(() => {
    if (projects.length > 0 && !selectedProjectId) {
      setSelectedProjectId(projects[0].name);
    }
  }, [projects, selectedProjectId]);

  // Reset repository selection when project changes (repos are project-scoped)
  useEffect(() => {
    setRepositoryId("");
  }, [selectedProjectId]);

  // Auto-select first namespace when namespaces load or change
  useEffect(() => {
    if (allowedNamespaces.length > 0) {
      // Reset namespace if it's not in the allowed list
      if (!namespace || !allowedNamespaces.includes(namespace)) {
        setNamespace(allowedNamespaces[0]);
      }
    } else if (namespace && !isLoadingNamespaces) {
      // Clear namespace if no namespaces available
      setNamespace("");
    }
  }, [allowedNamespaces, namespace, isLoadingNamespaces]);

  // Auto-populate gitBranch when repository is selected
  useEffect(() => {
    if (repositoryId) {
      const selectedRepo = repositories.find((r) => r.id === repositoryId);
      if (selectedRepo && selectedRepo.defaultBranch) {
        setGitBranch(selectedRepo.defaultBranch);
      } else {
        setGitBranch("main"); // Fallback to "main"
      }
    }
  }, [repositoryId, repositories]);

  // Auto-populate gitPath with semantic path structure
  useEffect(() => {
    if (repositoryId && deploymentMode !== "direct") {
      // Build semantic path: manifests/{project}/{namespace}/{rgdName}/{instanceName}.yaml
      const parts = ["manifests"];
      if (selectedProjectId) {
        parts.push(selectedProjectId);
      }
      if (namespace) {
        parts.push(namespace);
      }
      if (rgd.name) {
        parts.push(rgd.name);
      }
      if (instanceName) {
        parts.push(`${instanceName}.yaml`);
      } else {
        parts.push("instance.yaml"); // Placeholder until instance name is provided
      }
      setGitPath(parts.join("/"));
    }
  }, [repositoryId, deploymentMode, selectedProjectId, namespace, rgd.name, instanceName]);

  if (isLoading) {
    return (
      <div className="space-y-6 animate-fade-in">
        <BackButton onBack={onBack} />
        <div className="rounded-lg border border-border bg-card p-6">
          <div className="flex items-center justify-center h-64">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" />
              <span className="text-sm">Loading schema...</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (error || !schemaResponse?.schema) {
    return (
      <div className="space-y-6 animate-fade-in">
        <BackButton onBack={onBack} />
        <div className="rounded-lg border border-border bg-card p-6">
          <div className="flex flex-col items-center justify-center h-64 gap-3">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <h3 className="text-sm font-medium text-foreground">Cannot load deployment form</h3>
            <p className="text-sm text-muted-foreground text-center max-w-md">
              {error instanceof Error
                ? error.message
                : schemaResponse?.error || "No CRD schema found for this RGD. The CRD may not exist yet."}
            </p>
            <button
              onClick={onBack}
              className="mt-4 px-4 py-2 text-sm font-medium text-primary hover:text-primary/80 transition-colors"
            >
              Return to catalog
            </button>
          </div>
        </div>
      </div>
    );
  }

  const { schema } = schemaResponse;

  return (
    <div className="space-y-6 animate-fade-in">
      <BackButton onBack={onBack} />

      {/* Header */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-start gap-4">
          <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
            <Rocket className="h-6 w-6 text-primary" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Deploy {rgd.title || rgd.name}
            </h1>
            {rgd.title && rgd.title !== rgd.name && (
              <p className="text-sm text-muted-foreground font-mono mt-0.5">{rgd.name}</p>
            )}
            <p className="text-sm text-muted-foreground mt-1">
              Create a new instance of this resource
            </p>
            <div className="flex items-center gap-2 mt-2">
              <span className="text-xs font-mono text-muted-foreground bg-secondary px-2 py-0.5 rounded">
                {schema.group}/{schema.version}
              </span>
              <span className="text-xs font-mono text-muted-foreground bg-secondary px-2 py-0.5 rounded">
                {schema.kind}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Degraded mode indicator */}
      {schemaResponse.source === "rgd-only" && (
        <div className="rounded-lg border border-status-warning/50 bg-status-warning/10 p-4">
          <div className="flex items-center gap-3">
            <AlertTriangle className="h-5 w-5 text-status-warning shrink-0" />
            <p className="text-sm text-muted-foreground">
              <span className="font-medium text-status-warning">Preview mode</span> — some validation constraints are pending CRD generation
            </p>
          </div>
        </div>
      )}

      {/* Success Message */}
      {submitSuccess && (
        <div className="rounded-lg border border-status-success bg-status-success/10 p-4">
          <div className="flex items-center gap-3">
            <CheckCircle2 className="h-5 w-5 text-status-success" />
            <div>
              <h3 className="text-sm font-medium text-foreground">
                {deploymentMode === "direct" && "Deployment submitted successfully!"}
                {deploymentMode === "gitops" && "Pushed to Git successfully!"}
                {deploymentMode === "hybrid" && "Deployed and pushed to Git successfully!"}
              </h3>
              <p className="text-sm text-muted-foreground mt-1">
                {deploymentMode === "direct" && (
                  <>
                    Instance <span className="font-mono">{instanceName}</span> is being created.
                  </>
                )}
                {deploymentMode === "gitops" && (
                  <>
                    Instance <span className="font-mono">{instanceName}</span> manifest pushed to repository.
                    GitOps tool will sync it to your cluster.
                  </>
                )}
                {deploymentMode === "hybrid" && (
                  <>
                    Instance <span className="font-mono">{instanceName}</span> is being created and pushed to repository for audit trail.
                  </>
                )}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Main Form */}
      {!submitSuccess && (
        <DeployFormContent
          schema={schema}
          instanceName={instanceName}
          onInstanceNameChange={setInstanceName}
          selectedProjectId={selectedProjectId}
          onProjectChange={setSelectedProjectId}
          projects={projects}
          isLoadingProjects={isLoadingProjects}
          namespace={namespace}
          onNamespaceChange={setNamespace}
          allowedNamespaces={allowedNamespaces}
          isLoadingNamespaces={isLoadingNamespaces}
          deploymentMode={deploymentMode}
          onDeploymentModeChange={setDeploymentMode}
          repositoryId={repositoryId}
          onRepositoryChange={setRepositoryId}
          gitBranch={gitBranch}
          onGitBranchChange={setGitBranch}
          gitPath={gitPath}
          onGitPathChange={setGitPath}
          repositories={repositories}
          isLoadingRepositories={isLoadingRepos}
          repositoriesError={reposError?.message ?? null}
          allowedDeploymentModes={rgd.allowedDeploymentModes}
          isSubmitting={createInstanceMutation.isPending}
          submitError={createInstanceMutation.error?.message ?? null}
          canDeployInProject={canDeployInProject}
          isLoadingPermission={isLoadingPermission}
          isErrorPermission={isErrorPermission}
          onSubmit={async (values) => {
            createInstanceMutation.mutate(
              {
                name: instanceName,
                namespace,
                projectId: selectedProjectId || undefined,
                rgdName: rgd.name,
                rgdNamespace: rgd.namespace,
                spec: values,
                deploymentMode,
                repositoryId: deploymentMode !== "direct" ? repositoryId : undefined,
                gitBranch: deploymentMode !== "direct" && gitBranch ? gitBranch : undefined,
                gitPath: deploymentMode !== "direct" && gitPath ? gitPath : undefined,
              },
              {
                onSuccess: () => {
                  setSubmitSuccess(true);
                  onDeploySuccess?.(instanceName, namespace);
                },
              }
            );
          }}
        />
      )}
    </div>
  );
}

function BackButton({ onBack }: { onBack: () => void }) {
  return (
    <button
      onClick={onBack}
      className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
    >
      <ArrowLeft className="h-4 w-4" />
      Back to details
    </button>
  );
}

interface DeployFormContentProps {
  schema: FormSchema;
  instanceName: string;
  onInstanceNameChange: (name: string) => void;
  selectedProjectId: string;
  onProjectChange: (projectId: string) => void;
  projects: Project[];
  isLoadingProjects?: boolean;
  namespace: string;
  onNamespaceChange: (namespace: string) => void;
  allowedNamespaces: string[];
  isLoadingNamespaces?: boolean;
  deploymentMode: DeploymentMode;
  onDeploymentModeChange: (mode: DeploymentMode) => void;
  repositoryId: string;
  onRepositoryChange: (repositoryId: string) => void;
  gitBranch: string;
  onGitBranchChange: (branch: string) => void;
  gitPath: string;
  onGitPathChange: (path: string) => void;
  repositories: RepositoryConfig[];
  isLoadingRepositories?: boolean;
  repositoriesError?: string | null;
  /** Allowed deployment modes from RGD annotation. Empty/undefined = all modes allowed. */
  allowedDeploymentModes?: DeploymentMode[];
  isSubmitting: boolean;
  submitError: string | null;
  canDeployInProject: boolean | undefined;
  isLoadingPermission?: boolean;
  isErrorPermission?: boolean;
  onSubmit: (values: Record<string, unknown>) => void;
}

function DeployFormContent({
  schema,
  instanceName,
  onInstanceNameChange,
  selectedProjectId,
  onProjectChange,
  projects,
  isLoadingProjects,
  namespace,
  onNamespaceChange,
  allowedNamespaces,
  isLoadingNamespaces,
  deploymentMode,
  onDeploymentModeChange,
  repositoryId,
  onRepositoryChange,
  gitBranch,
  onGitBranchChange,
  gitPath,
  onGitPathChange,
  repositories,
  isLoadingRepositories,
  repositoriesError,
  allowedDeploymentModes,
  isSubmitting,
  submitError,
  canDeployInProject,
  isLoadingPermission,
  isErrorPermission: _isErrorPermission,
  onSubmit,
}: DeployFormContentProps) {
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
    formState: { errors },
    watch,
  } = methods;

  const formValues = watch();
  const hasErrors = Object.keys(errors).length > 0;
  const isInstanceNameValid = instanceName.length >= 1 && /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(instanceName);
  const needsRepository = deploymentMode === "gitops" || deploymentMode === "hybrid";
  const isRepositoryValid = !needsRepository || repositoryId.length > 0;

  // Get field visibility based on conditional sections (CEL AST rules + AND-based hiding)
  const { isFieldVisible } = useFieldVisibility(
    schema.conditionalSections,
    formValues
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
  }, [schema]);

  return (
    <FormProvider {...methods}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
        {/* Instance Metadata Section */}
        <div className="rounded-lg border border-border bg-card p-4 space-y-4">
          <div className="flex items-center gap-2">
            <Box className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-medium text-foreground">Instance Details</h3>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5" data-testid="field-instanceName">
              <label htmlFor="instanceName" className="text-sm font-medium text-foreground flex items-center gap-1">
                Instance Name
                <span className="text-destructive">*</span>
              </label>
              <input
                id="instanceName"
                type="text"
                value={instanceName}
                onChange={(e) => onInstanceNameChange(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "-"))}
                placeholder="my-instance"
                data-testid="input-instanceName"
                className={cn(
                  "w-full px-3 py-2 text-sm rounded-md border bg-background",
                  "focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary",
                  instanceName && !isInstanceNameValid ? "border-destructive" : "border-border"
                )}
              />
              <p className="text-xs text-muted-foreground">
                Must be lowercase, alphanumeric, and may contain hyphens
              </p>
            </div>

            <div className="space-y-1.5" data-testid="field-project">
              <label htmlFor="project" className="text-sm font-medium text-foreground flex items-center gap-1">
                <FolderKanban className="h-3.5 w-3.5" />
                Project
                <span className="text-destructive">*</span>
              </label>
              <select
                id="project"
                value={selectedProjectId}
                onChange={(e) => onProjectChange(e.target.value)}
                data-testid="input-project"
                disabled={isLoadingProjects}
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary disabled:opacity-50"
              >
                {isLoadingProjects ? (
                  <option value="">Loading projects...</option>
                ) : projects.length === 0 ? (
                  <option value="">No projects available</option>
                ) : (
                  <>
                    <option value="">Select a project</option>
                    {projects.map((project) => (
                      <option key={project.name} value={project.name}>
                        {project.name}
                      </option>
                    ))}
                  </>
                )}
              </select>
              <p className="text-xs text-muted-foreground">
                Project defines allowed namespaces and repositories
              </p>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5" data-testid="field-namespace">
              <label htmlFor="namespace" className="text-sm font-medium text-foreground flex items-center gap-1">
                Namespace
                <span className="text-destructive">*</span>
              </label>
              <select
                id="namespace"
                value={namespace}
                onChange={(e) => onNamespaceChange(e.target.value)}
                data-testid="input-namespace"
                disabled={!selectedProjectId || isLoadingNamespaces || allowedNamespaces.length === 0}
                className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary disabled:opacity-50"
              >
                {!selectedProjectId ? (
                  <option value="">Select a project first</option>
                ) : isLoadingNamespaces ? (
                  <option value="">Loading namespaces...</option>
                ) : allowedNamespaces.length === 0 ? (
                  <option value="">No namespaces available</option>
                ) : (
                  <>
                    <option value="">Select a namespace</option>
                    {allowedNamespaces.map((ns) => (
                      <option key={ns} value={ns}>
                        {ns}
                      </option>
                    ))}
                  </>
                )}
              </select>
              <p className="text-xs text-muted-foreground">
                Namespace for deployment (matching project policies)
              </p>
            </div>
          </div>

          {/* Permission Warning — only on explicit deny, not on error/loading */}
          {selectedProjectId && !isLoadingPermission && canDeployInProject === false && (
            <div className="mt-4 rounded-lg border border-status-warning bg-status-warning/10 p-4" data-testid="deploy-permission-warning">
              <div className="flex items-start gap-3">
                <ShieldAlert className="h-5 w-5 text-status-warning shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <h4 className="text-sm font-medium text-status-warning">
                    Cannot deploy to this project
                  </h4>
                  <p className="text-sm text-muted-foreground">
                    You do not have permission to deploy instances in this project.
                    Contact a project administrator for access.
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Deployment Mode Section */}
        <div className="rounded-lg border border-border bg-card p-4">
          <div className="flex items-center gap-2 mb-4">
            <Settings className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-medium text-foreground">Deployment Options</h3>
          </div>
          <DeploymentModeSelector
            mode={deploymentMode}
            onModeChange={onDeploymentModeChange}
            repositoryId={repositoryId}
            onRepositoryChange={onRepositoryChange}
            gitBranch={gitBranch}
            onGitBranchChange={onGitBranchChange}
            gitPath={gitPath}
            onGitPathChange={onGitPathChange}
            repositories={repositories}
            isLoadingRepositories={isLoadingRepositories}
            repositoriesError={repositoriesError}
            allowedModes={allowedDeploymentModes}
          />
        </div>

        {/* Schema Properties Section */}
        <div className="rounded-lg border border-border bg-card p-4 space-y-4">
          <div className="flex items-center gap-2">
            <FileCode className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-medium text-foreground">Configuration</h3>
          </div>

          {schema.description && (
            <p className="text-sm text-muted-foreground pb-4 border-b border-border">
              {schema.description}
            </p>
          )}

          {/* Regular (non-advanced) fields */}
          <div className="space-y-4">
            {(() => {
              // Build a map of controlled fields for quick lookup
              const controlledFieldsMap = new Map<string, string[]>();
              // Only skip fields from regular rendering if their controlling field
              // is also a regular (non-advanced) field. If controlled by an advanced
              // field, render normally — isFieldVisible handles visibility.
              const regularControlledFields = new Set<string>();

              schema.conditionalSections?.forEach(section => {
                const controllingField = section.controllingField.replace(/^spec\./, "");
                const controlled = controlledFieldsMap.get(controllingField) || [];
                const controllerIsAdvanced = isAdvancedField(controllingField);
                section.affectedProperties.forEach(prop => {
                  if (!controlled.includes(prop)) {
                    controlled.push(prop);
                  }
                  // Only defer rendering if the controlling field is in the same
                  // (regular) section so it can render the controlled field after itself
                  if (!controllerIsAdvanced) {
                    regularControlledFields.add(prop);
                  }
                });
                controlledFieldsMap.set(controllingField, controlled);
              });

              // For fields with multiple controllers (e.g. hiddenAnnotation controlled
              // by both enableCache and enableDatabase), render after the LAST controller
              // in form order so the field appears below the toggle the user clicked.
              const controllerFormOrder = new Map<string, number>();
              regularProperties.forEach(([name], index) => {
                controllerFormOrder.set(name, index);
              });

              const lastControllerForField = new Map<string, string>();
              controlledFieldsMap.forEach((fields, controllerName) => {
                for (const field of fields) {
                  if (isAdvancedField(field)) continue;
                  const currentLast = lastControllerForField.get(field);
                  if (!currentLast || (controllerFormOrder.get(controllerName) ?? -1) > (controllerFormOrder.get(currentLast) ?? -1)) {
                    lastControllerForField.set(field, controllerName);
                  }
                }
              });

              // Render only regular (non-advanced) fields
              return regularProperties.map(([name, property]) => {
                // Skip if this is a controlled field whose controller is also regular
                // (it will be rendered after its controlling field below)
                if (regularControlledFields.has(name)) {
                  return null;
                }

                // Skip hidden fields based on conditional sections
                if (!isFieldVisible(name)) {
                  return null;
                }

                // Get controlled fields for this field — only those where this
                // controller is the LAST one in form order (prevents duplicates
                // and ensures the field appears below the last relevant toggle)
                const controlledFields = (controlledFieldsMap.get(name) || []).filter(
                  cf => !isAdvancedField(cf) && lastControllerForField.get(cf) === name
                );

                // Render the field and any visible controlled fields
                return (
                  <div key={name} className="space-y-4">
                    <FormField
                      name={name}
                      property={property}
                      required={schema.required?.includes(name)}
                      deploymentNamespace={namespace}
                    />
                    {controlledFields.map(controlledName => {
                      const controlledProperty = schema.properties[controlledName];
                      if (!controlledProperty || !isFieldVisible(controlledName)) {
                        return null;
                      }

                      return (
                        <FormField
                          key={controlledName}
                          name={controlledName}
                          property={controlledProperty}
                          required={schema.required?.includes(controlledName)}
                          deploymentNamespace={namespace}
                        />
                      );
                    })}
                  </div>
                );
              });
            })()}
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

          {regularProperties.length === 0 && advancedProperties.length === 0 && (
            <p className="text-sm text-muted-foreground text-center py-8">
              No configuration options required for this resource.
            </p>
          )}
        </div>

        {/* YAML Preview */}
        <YAMLPreview
          apiVersion={`${schema.group}/${schema.version}`}
          kind={schema.kind}
          name={instanceName}
          namespace={namespace}
          spec={formValues}
        />

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
            disabled={isSubmitting || isLoadingPermission || !isInstanceNameValid || !isRepositoryValid || hasErrors || canDeployInProject === false}
            data-testid="deploy-submit-button"
            className={cn(
              "flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md transition-colors",
              "bg-primary text-primary-foreground hover:bg-primary/90",
              "disabled:opacity-50 disabled:cursor-not-allowed"
            )}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                {deploymentMode === "direct" && "Deploying..."}
                {deploymentMode === "gitops" && "Pushing to Git..."}
                {deploymentMode === "hybrid" && "Deploying & Pushing..."}
              </>
            ) : isLoadingPermission ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Checking permissions...
              </>
            ) : canDeployInProject === false ? (
              <>
                <ShieldAlert className="h-4 w-4" />
                No Permission to Deploy
              </>
            ) : (
              <>
                <Rocket className="h-4 w-4" />
                {deploymentMode === "direct" && "Deploy Instance"}
                {deploymentMode === "gitops" && "Push to Git"}
                {deploymentMode === "hybrid" && "Deploy & Push"}
              </>
            )}
          </button>
        </div>
      </form>
    </FormProvider>
  );
}
