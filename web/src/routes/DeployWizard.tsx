// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Loader2, AlertCircle } from "@/lib/icons";
import { toast } from "sonner";
import { useRGD, useRGDSchema } from "@/hooks/useRGDs";
import { useProjects } from "@/hooks/useProjects";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { useCurrentProject } from "@/hooks/useAuth";
import {
  StepWizard,
  type StepWizardRef,
  type WizardStep,
} from "@/components/ui/step-wizard";
import { ProjectStep } from "@/components/deploy/project-step";
import { ConfigureStep } from "@/components/deploy/configure-step";
import { ReviewStep } from "@/components/deploy/review-step";
import { YAMLPreview } from "@/components/deploy/YAMLPreview";
import { DiscardDialog } from "@/components/deploy/discard-dialog";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageSkeleton } from "@/components/ui/page-skeleton";
import { validateCompliance, type ComplianceValidateViolation } from "@/api/compliance";
import { createInstance } from "@/api/rgd";
import type { CreateInstanceRequest } from "@/types/rgd";

export default function DeployWizardRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const navigate = useNavigate();
  const wizardRef = useRef<StepWizardRef>(null);
  const decodedName = decodeURIComponent(rgdName || "");

  // Fetch RGD details and schema
  const { data: rgd, isLoading: rgdLoading, error: rgdError } = useRGD(decodedName);
  const { data: schemaResponse, isLoading: schemaLoading } = useRGDSchema(decodedName);

  // Fetch projects for auto-skip logic
  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData]);

  // Wizard form state (lifted to parent)
  const globalProject = useCurrentProject();
  const [selectedProject, setSelectedProject] = useState(globalProject ?? "");
  const [selectedNamespace, setSelectedNamespace] = useState("");
  const [_formValues, setFormValues] = useState<Record<string, unknown>>({});
  const [formValid, setFormValid] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Compliance state
  const [complianceResult, setComplianceResult] = useState<"pass" | "warning" | "block">("pass");
  const [complianceViolations, setComplianceViolations] = useState<ComplianceValidateViolation[]>([]);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);

  // Fetch namespaces for the selected project
  const { data: namespacesData } = useProjectNamespaces(selectedProject);
  const namespaces = useMemo(
    () => namespacesData?.items ?? namespacesData ?? [],
    [namespacesData]
  );

  // Auto-skip: if exactly 1 project, auto-select and start at step 1 (Configure)
  const singleProject = projects.length === 1;
  const autoSelectedProject = singleProject ? projects[0].name : "";

  // Use auto-selected project if single project scenario
  const effectiveProject = selectedProject || autoSelectedProject;

  // Namespace access is determined by Casbin namespace-scoped policies (roles[].destinations)
  const filteredNamespaces = namespaces as string[];

  // Track if project was auto-selected
  const projectStepValid = useCallback(() => {
    return !!effectiveProject && (!!selectedNamespace || schemaResponse?.schema?.isClusterScoped === true);
  }, [effectiveProject, selectedNamespace, schemaResponse?.schema?.isClusterScoped]);

  const configureStepValid = useCallback(() => {
    return formValid;
  }, [formValid]);

  // Build wizard steps
  const steps = useMemo((): WizardStep[] => {
    const schema = schemaResponse?.schema;

    const configureStep: WizardStep = {
      id: "configure",
      label: "Configure",
      component: schema ? (
        <ConfigureStep
          schema={schema}
          onValuesChange={(values, isValid) => {
            setFormValues(values);
            setFormValid(isValid);
            setHasUnsavedChanges(true);
          }}
        />
      ) : (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-[var(--text-muted)]" />
        </div>
      ),
      isValid: configureStepValid,
    };

    const reviewStep: WizardStep = {
      id: "review",
      label: "Review",
      component: (
        <div className="space-y-4">
          <ReviewStep
            project={effectiveProject}
            namespace={selectedNamespace}
            formValues={_formValues}
            isClusterScoped={schema?.isClusterScoped}
            complianceResult={complianceResult}
            complianceViolations={complianceViolations}
            onAcknowledgeWarnings={() => setWarningsAcknowledged(true)}
            onEditStep={(stepIndex) => wizardRef.current?.goToStep(stepIndex)}
            propertyOrder={schema?.propertyOrder}
          />
          {schema && (
            <YAMLPreview
              apiVersion={`${schema.group}/${schema.version}`}
              kind={schema.kind}
              name=""
              namespace={selectedNamespace}
              spec={_formValues}
              propertyOrder={schema.propertyOrder}
            />
          )}
        </div>
      ),
      isValid: () => {
        if (complianceResult === "block") return false;
        if (complianceResult === "warning" && !warningsAcknowledged) return false;
        return true;
      },
    };

    // Skip project step if single project
    if (singleProject) {
      return [configureStep, reviewStep];
    }

    return [
      {
        id: "project",
        label: "Project",
        component: (
          <ProjectStep
            projects={projects}
            selectedProject={effectiveProject}
            onProjectChange={(p) => {
              setSelectedProject(p);
              setSelectedNamespace("");
              setHasUnsavedChanges(true);
            }}
            namespaces={filteredNamespaces}
            selectedNamespace={selectedNamespace}
            onNamespaceChange={(ns) => {
              setSelectedNamespace(ns);
              setHasUnsavedChanges(true);
            }}
            isClusterScoped={schema?.isClusterScoped}
          />
        ),
        isValid: projectStepValid,
      },
      configureStep,
      reviewStep,
    ];
  }, [
    schemaResponse?.schema,
    singleProject,
    projects,
    effectiveProject,
    filteredNamespaces,
    selectedNamespace,
    projectStepValid,
    configureStepValid,
    _formValues,
    complianceResult,
    complianceViolations,
    warningsAcknowledged,
  ]);

  // Run compliance validation when advancing from Configure to Review
  const runComplianceCheck = useCallback(async () => {
    try {
      const result = await validateCompliance({
        rgdName: decodedName,
        project: effectiveProject,
        namespace: selectedNamespace,
        values: _formValues,
      });
      setComplianceResult(result.result);
      setComplianceViolations(result.violations);
      setWarningsAcknowledged(false);
    } catch {
      // On API error, allow proceeding (fail-open for validation)
      setComplianceResult("pass");
      setComplianceViolations([]);
    }
  }, [decodedName, effectiveProject, selectedNamespace, _formValues]);

  // Handle wizard completion — submit the deployment
  const handleComplete = useCallback(async () => {
    if (isSubmitting) return;

    // Run compliance check before submitting
    await runComplianceCheck();

    const kind = schemaResponse?.schema?.kind;
    if (!kind) {
      toast.error("Cannot deploy: RGD schema not available");
      return;
    }

    setIsSubmitting(true);
    try {
      const request: CreateInstanceRequest = {
        name: `${decodedName}-${Date.now()}`,
        namespace: selectedNamespace || undefined,
        projectId: effectiveProject,
        rgdName: decodedName,
        spec: _formValues,
      };

      const result = await createInstance(kind, request);
      toast.success(`Instance "${result.name}" deployed successfully`);
      setHasUnsavedChanges(false);

      // Navigate to instance detail
      const ns = result.namespace || "";
      navigate(`/instances/${encodeURIComponent(ns)}/${encodeURIComponent(kind)}/${encodeURIComponent(result.name)}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Deployment failed";
      toast.error(message);
    } finally {
      setIsSubmitting(false);
    }
  }, [isSubmitting, runComplianceCheck, schemaResponse?.schema?.kind, decodedName, selectedNamespace, effectiveProject, _formValues, navigate]);

  // Breadcrumb items
  const breadcrumbs = useMemo(
    () => [
      { label: "Catalog", href: "/catalog" },
      {
        label: rgd?.title || decodedName,
        href: `/catalog/${encodeURIComponent(decodedName)}`,
      },
      { label: "Deploy" },
    ],
    [rgd?.title, decodedName]
  );

  // Loading state
  if (rgdLoading || schemaLoading || projectsLoading) {
    return <PageSkeleton />;
  }

  // Error state
  if (rgdError || !rgd) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <AlertCircle className="h-12 w-12 text-destructive" />
        <div className="text-center">
          <h2 className="text-lg font-semibold">RGD Not Found</h2>
          <p className="text-sm text-muted-foreground">
            The RGD &ldquo;{decodedName}&rdquo; could not be found.
          </p>
        </div>
        <button
          onClick={() => navigate("/catalog")}
          className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90"
        >
          Back to Catalog
        </button>
      </div>
    );
  }

  return (
    <>
      <PageHeader
        title={`Deploy ${rgd.title || rgd.name}`}
        breadcrumbs={breadcrumbs}
      />
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <div className="mx-auto max-w-2xl">
          <div className="rounded-[var(--radius-token-lg)] border border-[rgba(255,255,255,0.08)] bg-[var(--surface-primary)]">
            <StepWizard
              ref={wizardRef}
              steps={steps}
              initialStep={0}
              onComplete={handleComplete}
              actionLabel="Deploy"
              isSubmitting={isSubmitting}
            />
          </div>
        </div>
      </div>
      <DiscardDialog hasUnsavedChanges={hasUnsavedChanges} />
    </>
  );
}
