// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Check, Loader2, X } from "@/lib/icons";
import { toast } from "sonner";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { RGDIcon } from "@/components/ui/rgd-icon";
import { cn } from "@/lib/utils";
import type { CatalogRGD } from "@/types/rgd";
import { useRGDSchema } from "@/hooks/useRGDs";
import { useProjects } from "@/hooks/useProjects";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { useRepositories } from "@/hooks/useRepositories";
import { useCurrentProject } from "@/hooks/useAuth";
import { validateCompliance, type ComplianceValidateViolation } from "@/api/compliance";
import { createInstance, preflightInstance } from "@/api/rgd";
import type { CreateInstanceRequest } from "@/types/rgd";
import type { DeploymentMode } from "@/types/deployment";
import { TargetStep } from "./target-step";
import { ConfigureStep } from "./configure-step";
import { ReviewStep } from "./review-step";
import { DeploymentModeSelector } from "./DeploymentModeSelector";

// ---------------------------------------------------------------------------
// Step definitions
// ---------------------------------------------------------------------------

const STEPS = [
  { id: "target", label: "Target" },
  { id: "configure", label: "Configure" },
  { id: "review", label: "Review" },
] as const;

type StepIndex = 0 | 1 | 2;

// ---------------------------------------------------------------------------
// Inline step progress bar
// ---------------------------------------------------------------------------

function StepBar({ current, completed }: { current: StepIndex; completed: Set<number> }) {
  return (
    <div className="flex items-center justify-center gap-0 px-2">
      {STEPS.map((step, i) => {
        const isCompleted = completed.has(i);
        const isCurrent = i === current;
        const isFuture = !isCompleted && !isCurrent;

        return (
          <div key={step.id} className="flex items-center">
            <div className="flex flex-col items-center gap-1">
              <div
                className={cn(
                  "flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold transition-all duration-200",
                  isCompleted && "ring-2 ring-offset-2",
                  isCurrent && "ring-2 ring-offset-2",
                )}
                style={{
                  backgroundColor:
                    isCompleted || isCurrent
                      ? "var(--brand-primary)"
                      : "rgba(255,255,255,0.06)",
                  color:
                    isCompleted || isCurrent
                      ? "var(--surface-bg)"
                      : "var(--text-muted)",
                  ringColor:
                    isCompleted || isCurrent ? "var(--brand-primary)" : "transparent",
                  ringOffsetColor: "var(--surface-primary)",
                }}
                aria-current={isCurrent ? "step" : undefined}
              >
                {isCompleted ? <Check className="h-3.5 w-3.5" /> : i + 1}
              </div>
              <span
                className="text-[11px] font-medium whitespace-nowrap"
                style={{
                  color: isFuture ? "var(--text-muted)" : "var(--text-primary)",
                }}
              >
                {step.label}
              </span>
            </div>
            {i < STEPS.length - 1 && (
              <div
                className="mx-3 mb-4 h-px w-14 transition-colors duration-300"
                style={{
                  backgroundColor: isCompleted
                    ? "var(--brand-primary)"
                    : "rgba(255,255,255,0.10)",
                }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

interface DeployModalProps {
  rgd: CatalogRGD | null;
  open: boolean;
  onClose: () => void;
}

const INSTANCE_NAME_RE = /^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$/;

export function DeployModal({ rgd, open, onClose }: DeployModalProps) {
  const navigate = useNavigate();
  const rgdName = rgd?.name ?? "";

  // Fetch schema + projects
  const { data: schemaResponse, isLoading: schemaLoading } = useRGDSchema(
    rgdName,
    undefined,
  );
  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData]);

  // Wizard state
  const globalProject = useCurrentProject();
  const [currentStep, setCurrentStep] = useState<StepIndex>(0);
  const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set());

  // Step 1 — Target
  const [instanceName, setInstanceName] = useState("");
  const [instanceNameError, setInstanceNameError] = useState("");
  const [selectedProject, setSelectedProject] = useState(globalProject ?? "");
  const [selectedNamespace, setSelectedNamespace] = useState("");
  // Deployment Mode
  const [deploymentMode, setDeploymentMode] = useState<DeploymentMode>("direct");
  const [repositoryId, setRepositoryId] = useState("");
  const [gitBranch, setGitBranch] = useState("");
  const [gitPath, setGitPath] = useState("");
  // Step 2 — Configure
  const [formValues, setFormValues] = useState<Record<string, unknown>>({});
  const [formValid, setFormValid] = useState(false);

  // Step 3 — Review / compliance
  const [complianceResult, setComplianceResult] = useState<"pass" | "warning" | "block">("pass");
  const [complianceViolations, setComplianceViolations] = useState<ComplianceValidateViolation[]>([]);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  // Preflight dry-run result
  const [preflightBlocked, setPreflightBlocked] = useState(false);
  const [preflightMessage, setPreflightMessage] = useState<string | undefined>();

  // Submission
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  const schema = schemaResponse?.schema;
  const isClusterScoped = schema?.isClusterScoped ?? rgd?.isClusterScoped;

  // Auto-select single project — computed before hooks that depend on it
  const singleProject = projects.length === 1;
  const effectiveProject = selectedProject || (singleProject ? projects[0]?.name ?? "" : "");

  // Namespaces — use effectiveProject so single-project auto-selection works
  const { data: namespacesData } = useProjectNamespaces(effectiveProject);
  const namespaces = useMemo(
    () => namespacesData?.namespaces ?? [],
    [namespacesData],
  );

  // Namespace access is determined by Casbin namespace-scoped policies (roles[].destinations)
  const filteredNamespaces = namespaces;

  // Repositories — for GitOps and Hybrid deployment modes
  const { data: reposData, isLoading: reposLoading } = useRepositories(effectiveProject);
  const repositories = useMemo(() => reposData?.items ?? [], [reposData]);

  // ---------------------------------------------------------------------------
  // Validation
  // ---------------------------------------------------------------------------

  const validateInstanceName = useCallback((name: string): string => {
    if (!name) return "Instance name is required";
    if (name.length > 63) return "Must be 63 characters or fewer";
    if (!INSTANCE_NAME_RE.test(name))
      return "Lowercase letters, numbers, and hyphens only; must start and end with alphanumeric";
    return "";
  }, []);

  const targetStepValid = useCallback((): boolean => {
    const nameErr = validateInstanceName(instanceName);
    if (nameErr) return false;
    if (!effectiveProject) return false;
    if (!isClusterScoped && !selectedNamespace) return false;
    // Require repository for gitops/hybrid modes
    if ((deploymentMode === "gitops" || deploymentMode === "hybrid") && !repositoryId) return false;
    return true;
  }, [instanceName, effectiveProject, selectedNamespace, isClusterScoped, validateInstanceName, deploymentMode, repositoryId]);

  const configureStepValid = useCallback(() => formValid, [formValid]);

  const reviewStepValid = useCallback(() => {
    if (preflightBlocked) return false;
    if (complianceResult === "block") return false;
    if (complianceResult === "warning" && !warningsAcknowledged) return false;
    return true;
  }, [preflightBlocked, complianceResult, warningsAcknowledged]);

  const currentStepValid = useCallback((): boolean => {
    if (currentStep === 0) return targetStepValid();
    if (currentStep === 1) return configureStepValid();
    return reviewStepValid();
  }, [currentStep, targetStepValid, configureStepValid, reviewStepValid]);

  // ---------------------------------------------------------------------------
  // Compliance check
  // ---------------------------------------------------------------------------

  const runComplianceCheck = useCallback(async () => {
    try {
      const result = await validateCompliance({
        rgdName,
        project: effectiveProject,
        namespace: selectedNamespace,
        values: formValues,
      });
      setComplianceResult(result.result);
      setComplianceViolations(result.violations);
      setWarningsAcknowledged(false);
    } catch {
      setComplianceResult("pass");
      setComplianceViolations([]);
    }
  }, [rgdName, effectiveProject, selectedNamespace, formValues]);

  const runPreflight = useCallback(async () => {
    const kind = schema?.kind;
    if (!kind) return;
    try {
      const result = await preflightInstance(kind, {
        name: "preflight-check",
        namespace: selectedNamespace || undefined,
        projectId: effectiveProject,
        rgdName,
        spec: formValues,
      });
      setPreflightBlocked(!result.valid);
      setPreflightMessage(result.message);
    } catch {
      // If preflight endpoint fails (e.g. network error), don't block the deploy
      setPreflightBlocked(false);
      setPreflightMessage(undefined);
    }
  }, [schema?.kind, selectedNamespace, effectiveProject, rgdName, formValues]);

  // ---------------------------------------------------------------------------
  // Navigation
  // ---------------------------------------------------------------------------

  const handleNext = useCallback(async () => {
    if (!currentStepValid()) return;

    if (currentStep === 0) {
      const nameErr = validateInstanceName(instanceName);
      if (nameErr) {
        setInstanceNameError(nameErr);
        return;
      }
      setInstanceNameError("");
    }

    // Run compliance + preflight dry-run before showing Review
    if (currentStep === 1) {
      await Promise.all([runComplianceCheck(), runPreflight()]);
    }

    if (currentStep < 2) {
      setCompletedSteps((prev) => new Set(prev).add(currentStep));
      setCurrentStep((prev) => (prev + 1) as StepIndex);
    }
  }, [currentStep, currentStepValid, instanceName, validateInstanceName, runComplianceCheck, runPreflight]);

  const handleBack = useCallback(() => {
    if (currentStep > 0) {
      setCurrentStep((prev) => (prev - 1) as StepIndex);
    }
  }, [currentStep]);

  // ---------------------------------------------------------------------------
  // Submit
  // ---------------------------------------------------------------------------

  const handleDeploy = useCallback(async () => {
    if (isSubmitting || !reviewStepValid()) return;
    const kind = schema?.kind;
    if (!kind) {
      toast.error("Cannot deploy: schema not available");
      return;
    }

    setIsSubmitting(true);
    try {
      const request: CreateInstanceRequest = {
        name: instanceName,
        namespace: selectedNamespace || undefined,
        projectId: effectiveProject,
        rgdName,
        spec: formValues,
        deploymentMode,
        repositoryId: repositoryId || undefined,
        gitBranch: gitBranch || undefined,
        gitPath: gitPath || undefined,
      };

      const result = await createInstance(kind, request);
      toast.success(`"${result.name}" deployed successfully`);
      setHasUnsavedChanges(false);
      onClose();

      const ns = result.namespace || "";
      navigate(
        `/instances/${encodeURIComponent(ns)}/${encodeURIComponent(kind)}/${encodeURIComponent(result.name)}`,
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : "Deployment failed";
      toast.error(message);
    } finally {
      setIsSubmitting(false);
    }
  }, [
    isSubmitting,
    reviewStepValid,
    schema?.kind,
    instanceName,
    selectedNamespace,
    effectiveProject,
    rgdName,
    formValues,
    deploymentMode,
    repositoryId,
    gitBranch,
    gitPath,
    onClose,
    navigate,
  ]);

  // ---------------------------------------------------------------------------
  // Reset on close
  // ---------------------------------------------------------------------------

  const resetAndClose = useCallback(() => {
    setCurrentStep(0);
    setCompletedSteps(new Set());
    setInstanceName("");
    setInstanceNameError("");
    setSelectedProject(globalProject ?? "");
    setSelectedNamespace("");

    setDeploymentMode("direct");
    setRepositoryId("");
    setGitBranch("");
    setGitPath("");
    setFormValues({});
    setFormValid(false);
    setComplianceResult("pass");
    setComplianceViolations([]);
    setWarningsAcknowledged(false);
    setPreflightBlocked(false);
    setPreflightMessage(undefined);
    setHasUnsavedChanges(false);
    onClose();
  }, [globalProject, onClose]);

  const handleCloseRequest = useCallback(() => {
    if (hasUnsavedChanges) {
      setShowDiscardConfirm(true);
    } else {
      resetAndClose();
    }
  }, [hasUnsavedChanges, resetAndClose]);

  // ---------------------------------------------------------------------------
  // Stable step callbacks (extracted from useMemo to reduce invalidation)
  // ---------------------------------------------------------------------------

  const handleInstanceNameChange = useCallback((v: string) => {
    setInstanceName(v);
    setInstanceNameError("");
    setHasUnsavedChanges(true);
  }, []);

  const handleProjectChange = useCallback((p: string) => {
    setSelectedProject(p);
    setSelectedNamespace("");

    setHasUnsavedChanges(true);
  }, []);

  const handleNamespaceChange = useCallback((ns: string) => {
    setSelectedNamespace(ns);
    setHasUnsavedChanges(true);
  }, []);

  const handleDeploymentModeChange = useCallback((mode: DeploymentMode) => {
    setDeploymentMode(mode);
    // Reset repository fields when switching away from modes that need them
    if (mode === "direct") {
      setRepositoryId("");
      setGitBranch("");
      setGitPath("");
    }
    setHasUnsavedChanges(true);
  }, []);

  const handleRepositoryChange = useCallback((id: string) => {
    setRepositoryId(id);
    setHasUnsavedChanges(true);
  }, []);

  const handleGitBranchChange = useCallback((branch: string) => {
    setGitBranch(branch);
    setHasUnsavedChanges(true);
  }, []);

  const handleGitPathChange = useCallback((path: string) => {
    setGitPath(path);
    setHasUnsavedChanges(true);
  }, []);

  const handleValuesChange = useCallback((values: Record<string, unknown>, isValid: boolean) => {
    setFormValues(values);
    setFormValid(isValid);
    setHasUnsavedChanges(true);
  }, []);

  const handleAcknowledgeWarnings = useCallback(() => {
    setWarningsAcknowledged(true);
  }, []);

  const handleEditStep = useCallback((stepIndex: number) => {
    setCurrentStep(stepIndex as StepIndex);
  }, []);

  // ---------------------------------------------------------------------------
  // Render step content
  // ---------------------------------------------------------------------------

  const stepContent = useMemo(() => {
    if (currentStep === 0) {
      return (
        <>
          <TargetStep
            instanceName={instanceName}
            onInstanceNameChange={handleInstanceNameChange}
            instanceNameError={instanceNameError}
            projects={projects}
            selectedProject={effectiveProject}
            onProjectChange={handleProjectChange}
            namespaces={filteredNamespaces}
            selectedNamespace={selectedNamespace}
            onNamespaceChange={handleNamespaceChange}
            isClusterScoped={isClusterScoped}
          />
          <div className="mt-5 pt-5" style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}>
            <DeploymentModeSelector
              mode={deploymentMode}
              onModeChange={handleDeploymentModeChange}
              repositoryId={repositoryId}
              onRepositoryChange={handleRepositoryChange}
              gitBranch={gitBranch}
              onGitBranchChange={handleGitBranchChange}
              gitPath={gitPath}
              onGitPathChange={handleGitPathChange}
              repositories={repositories}
              isLoadingRepositories={reposLoading}
              allowedModes={rgd?.allowedDeploymentModes}
            />
          </div>
        </>
      );
    }

    if (currentStep === 1) {
      if (!schema) {
        return (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin" style={{ color: "var(--text-muted)" }} />
          </div>
        );
      }
      return (
        <ConfigureStep
          schema={schema}
          onValuesChange={handleValuesChange}
          deploymentNamespace={selectedNamespace}
        />
      );
    }

    return (
      <ReviewStep
        project={effectiveProject}
        namespace={selectedNamespace}
        instanceName={instanceName}
        formValues={formValues}
        isClusterScoped={isClusterScoped}
        complianceResult={complianceResult}
        complianceViolations={complianceViolations}
        onAcknowledgeWarnings={handleAcknowledgeWarnings}
        onEditStep={handleEditStep}
        preflightBlocked={preflightBlocked}
        preflightMessage={preflightMessage}
      />
    );
  }, [
    currentStep,
    instanceName,
    handleInstanceNameChange,
    instanceNameError,
    projects,
    effectiveProject,
    handleProjectChange,
    filteredNamespaces,
    selectedNamespace,
    handleNamespaceChange,
    isClusterScoped,
    deploymentMode,
    handleDeploymentModeChange,
    repositoryId,
    handleRepositoryChange,
    gitBranch,
    handleGitBranchChange,
    gitPath,
    handleGitPathChange,
    repositories,
    reposLoading,
    rgd?.allowedDeploymentModes,
    schema,
    handleValuesChange,
    formValues,
    complianceResult,
    complianceViolations,
    handleAcknowledgeWarnings,
    handleEditStep,
    preflightBlocked,
    preflightMessage,
  ]);

  // ---------------------------------------------------------------------------
  // Loading guard
  // ---------------------------------------------------------------------------

  const isLoading = schemaLoading || projectsLoading;
  const isLastStep = currentStep === 2;
  const canAdvance = currentStepValid();

  // ---------------------------------------------------------------------------
  // JSX
  // ---------------------------------------------------------------------------

  return (
    <>
      <DialogPrimitive.Root open={open} onOpenChange={(o) => !o && handleCloseRequest()}>
        <DialogPrimitive.Portal>
          {/* Overlay */}
          <DialogPrimitive.Overlay
            className="fixed inset-0 z-50 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
            style={{ backgroundColor: "rgba(0,0,0,0.75)", backdropFilter: "blur(2px)" }}
          />

          {/* Modal content */}
          <DialogPrimitive.Content
            className={cn(
              "fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
              "w-full max-w-[580px] mx-4",
              "rounded-[var(--radius-token-lg)] border shadow-2xl",
              "flex flex-col max-h-[90vh]",
              "data-[state=open]:animate-in data-[state=closed]:animate-out",
              "data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
              "data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
              "data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%]",
              "data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]",
              "duration-200",
            )}
            style={{
              backgroundColor: "var(--surface-primary)",
              borderColor: "rgba(255,255,255,0.08)",
            }}
          >
            {/* RGD Header */}
            <div
              className="flex items-start justify-between gap-3 px-5 pt-5 pb-4 shrink-0"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}
            >
              <div className="flex items-center gap-3 min-w-0">
                <div
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--radius-token-sm)]"
                  style={{ backgroundColor: "rgba(255,255,255,0.06)" }}
                >
                  <RGDIcon
                    icon={rgd?.icon}
                    category={rgd?.category ?? "uncategorized"}
                    className="h-5 w-5 text-[var(--brand-primary)]"
                  />
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <DialogPrimitive.Title
                      className="text-base font-semibold leading-tight"
                      style={{ color: "var(--text-primary)" }}
                    >
                      Deploy {rgd?.title || rgdName}
                    </DialogPrimitive.Title>
                  </div>
                  <DialogPrimitive.Description
                    className="text-xs mt-0.5 line-clamp-1"
                    style={{ color: "var(--text-muted)" }}
                  >
                    {rgd?.description || "Deploy a new instance of this resource"}
                  </DialogPrimitive.Description>
                </div>
              </div>
              <button
                onClick={handleCloseRequest}
                className="shrink-0 rounded-md p-1.5 transition-colors hover:bg-[rgba(255,255,255,0.06)]"
                style={{ color: "var(--text-muted)" }}
                aria-label="Close"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Step progress bar */}
            <div
              className="px-5 py-4 shrink-0"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}
            >
              <StepBar current={currentStep} completed={completedSteps} />
            </div>

            {/* Step content — scrollable */}
            <div className="flex-1 overflow-y-auto px-5 py-5 min-h-0">
              {isLoading ? (
                <div className="flex items-center justify-center py-16">
                  <Loader2
                    className="h-6 w-6 animate-spin"
                    style={{ color: "var(--text-muted)" }}
                  />
                </div>
              ) : (
                stepContent
              )}
            </div>

            {/* Footer */}
            <div
              className="flex items-center justify-between gap-3 px-5 py-4 shrink-0"
              style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}
            >
              {/* Back */}
              <button
                type="button"
                onClick={handleBack}
                disabled={currentStep === 0 || isSubmitting}
                className={cn(
                  "px-4 py-2 rounded-md text-sm font-medium transition-colors",
                  currentStep === 0 ? "invisible" : "hover:bg-[rgba(255,255,255,0.06)]",
                )}
                style={{ color: "var(--text-secondary)" }}
              >
                Back
              </button>

              <div className="flex items-center gap-3">
                {/* Cancel (text link) */}
                <button
                  type="button"
                  onClick={handleCloseRequest}
                  disabled={isSubmitting}
                  className="text-sm transition-colors hover:underline"
                  style={{ color: "var(--text-muted)" }}
                >
                  Cancel
                </button>

                {/* Next / Deploy */}
                {isLastStep ? (
                  <button
                    type="button"
                    onClick={handleDeploy}
                    disabled={!canAdvance || isSubmitting}
                    data-testid="deploy-submit-button"
                    className="flex items-center gap-2 px-5 py-2 rounded-md text-sm font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{
                      backgroundColor:
                        canAdvance && !isSubmitting
                          ? "var(--brand-primary)"
                          : "var(--text-muted)",
                      color: "var(--surface-bg)",
                    }}
                  >
                    {isSubmitting && <Loader2 className="h-4 w-4 animate-spin" />}
                    🚀 Deploy
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleNext}
                    disabled={!canAdvance || isSubmitting}
                    className="px-5 py-2 rounded-md text-sm font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{
                      backgroundColor:
                        canAdvance && !isSubmitting
                          ? "var(--brand-primary)"
                          : "rgba(255,255,255,0.10)",
                      color: canAdvance && !isSubmitting
                        ? "var(--surface-bg)"
                        : "var(--text-muted)",
                    }}
                  >
                    Continue
                  </button>
                )}
              </div>
            </div>
          </DialogPrimitive.Content>
        </DialogPrimitive.Portal>
      </DialogPrimitive.Root>

      {/* Discard confirm */}
      <AlertDialog open={showDiscardConfirm} onOpenChange={setShowDiscardConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>
              You have unsaved changes. Your progress will be lost if you close now.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setShowDiscardConfirm(false)}>
              Keep Editing
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setShowDiscardConfirm(false);
                resetAndClose();
              }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Discard
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
