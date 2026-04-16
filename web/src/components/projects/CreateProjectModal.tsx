// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import { Check, FolderOpen, Loader2, X } from "@/lib/icons";
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
import { cn } from "@/lib/utils";
import type { CreateProjectRequest, Destination, ProjectRole } from "@/types/project";
import { ProjectStep } from "./create-project/project-step";
import { DestinationsStep } from "./create-project/destinations-step";
import { RolesStep } from "./create-project/roles-step";

// ---------------------------------------------------------------------------
// Step definitions
// ---------------------------------------------------------------------------

const STEPS = [
  { id: "project", label: "Project" },
  { id: "destinations", label: "Destinations" },
  { id: "roles", label: "Roles" },
] as const;

type StepIndex = 0 | 1 | 2;

// ---------------------------------------------------------------------------
// Step progress bar (matches DeployModal pattern)
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

interface CreateProjectModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateProjectRequest) => Promise<void>;
  isLoading?: boolean;
}

const PROJECT_NAME_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export function CreateProjectModal({
  open,
  onClose,
  onSubmit,
  isLoading = false,
}: CreateProjectModalProps) {
  // Wizard state
  const [currentStep, setCurrentStep] = useState<StepIndex>(0);
  const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set());

  // Step 1 — Project
  const [projectName, setProjectName] = useState("");
  const [projectNameError, setProjectNameError] = useState("");
  const [description, setDescription] = useState("");

  // Step 2 — Destinations
  const [destinations, setDestinations] = useState<Destination[]>([]);
  const [destinationError, setDestinationError] = useState("");

  // Step 3 — Roles
  const [roles, setRoles] = useState<ProjectRole[]>([]);
  const [roleError, setRoleError] = useState("");

  // Close safety
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  // ---------------------------------------------------------------------------
  // Validation
  // ---------------------------------------------------------------------------

  const validateProjectName = useCallback((name: string): string => {
    if (!name) return "Project name is required";
    if (name.length > 63) return "Must be 63 characters or fewer";
    if (!PROJECT_NAME_RE.test(name))
      return "Lowercase letters, numbers, and hyphens only; must start and end with alphanumeric";
    return "";
  }, []);

  const projectStepValid = useCallback((): boolean => {
    return validateProjectName(projectName) === "";
  }, [projectName, validateProjectName]);

  const destinationsStepValid = useCallback((): boolean => {
    return destinations.length > 0;
  }, [destinations]);

  const rolesStepValid = useCallback((): boolean => {
    // Roles are optional, but if present they need a name and policies
    for (const role of roles) {
      if (!role.name?.trim()) return false;
      if (!role.policies?.length) return false;
    }
    return true;
  }, [roles]);

  const currentStepValid = useCallback((): boolean => {
    if (currentStep === 0) return projectStepValid();
    if (currentStep === 1) return destinationsStepValid();
    return rolesStepValid();
  }, [currentStep, projectStepValid, destinationsStepValid, rolesStepValid]);

  // ---------------------------------------------------------------------------
  // Navigation
  // ---------------------------------------------------------------------------

  const handleNext = useCallback(() => {
    if (!currentStepValid()) {
      // Show specific errors
      if (currentStep === 0) {
        setProjectNameError(validateProjectName(projectName));
      } else if (currentStep === 1) {
        setDestinationError("At least one destination is required");
      }
      return;
    }

    if (currentStep === 0) setProjectNameError("");
    if (currentStep === 1) setDestinationError("");

    if (currentStep < 2) {
      setCompletedSteps((prev) => new Set(prev).add(currentStep));
      setCurrentStep((prev) => (prev + 1) as StepIndex);
    }
  }, [currentStep, currentStepValid, projectName, validateProjectName]);

  const handleBack = useCallback(() => {
    if (currentStep > 0) {
      setCurrentStep((prev) => (prev - 1) as StepIndex);
    }
  }, [currentStep]);

  // ---------------------------------------------------------------------------
  // Submit
  // ---------------------------------------------------------------------------

  const handleCreate = useCallback(async () => {
    if (isLoading) return;

    // Validate roles
    if (roles.length > 0) {
      const invalidRole = roles.find((r) => !r.name?.trim());
      if (invalidRole) {
        setRoleError("All roles must have a name");
        return;
      }
      const noPolicyRole = roles.find((r) => !r.policies?.length);
      if (noPolicyRole) {
        setRoleError(`Role "${noPolicyRole.name}" must have at least one policy`);
        return;
      }
    }
    setRoleError("");

    await onSubmit({
      name: projectName,
      description: description || undefined,
      destinations: destinations.length > 0 ? destinations : undefined,
      roles: roles.length > 0 ? roles : undefined,
    });
  }, [isLoading, projectName, description, destinations, roles, onSubmit]);

  // ---------------------------------------------------------------------------
  // Reset on close
  // ---------------------------------------------------------------------------

  const resetAndClose = useCallback(() => {
    setCurrentStep(0);
    setCompletedSteps(new Set());
    setProjectName("");
    setProjectNameError("");
    setDescription("");
    setDestinations([]);
    setDestinationError("");
    setRoles([]);
    setRoleError("");
    setHasUnsavedChanges(false);
    onClose();
  }, [onClose]);

  const handleCloseRequest = useCallback(() => {
    if (hasUnsavedChanges) {
      setShowDiscardConfirm(true);
    } else {
      resetAndClose();
    }
  }, [hasUnsavedChanges, resetAndClose]);

  // ---------------------------------------------------------------------------
  // Stable callbacks
  // ---------------------------------------------------------------------------

  const handleNameChange = useCallback((v: string) => {
    setProjectName(v);
    setProjectNameError("");
    setHasUnsavedChanges(true);
  }, []);

  const handleDescriptionChange = useCallback((v: string) => {
    setDescription(v);
    setHasUnsavedChanges(true);
  }, []);

  const handleDestinationsChange = useCallback((dests: Destination[]) => {
    setDestinations(dests);
    setDestinationError("");
    setHasUnsavedChanges(true);
  }, []);

  const handleRolesChange = useCallback((r: ProjectRole[]) => {
    setRoles(r);
    setRoleError("");
    setHasUnsavedChanges(true);
  }, []);

  // ---------------------------------------------------------------------------
  // Step content
  // ---------------------------------------------------------------------------

  const stepContent = useMemo(() => {
    if (currentStep === 0) {
      return (
        <ProjectStep
          name={projectName}
          onNameChange={handleNameChange}
          nameError={projectNameError}
          description={description}
          onDescriptionChange={handleDescriptionChange}
        />
      );
    }

    if (currentStep === 1) {
      return (
        <DestinationsStep
          destinations={destinations}
          onDestinationsChange={handleDestinationsChange}
          error={destinationError}
        />
      );
    }

    return (
      <RolesStep
        projectName={projectName}
        destinations={destinations}
        roles={roles}
        onRolesChange={handleRolesChange}
        error={roleError}
      />
    );
  }, [
    currentStep,
    projectName,
    handleNameChange,
    projectNameError,
    description,
    handleDescriptionChange,
    destinations,
    handleDestinationsChange,
    destinationError,
    roles,
    handleRolesChange,
    roleError,
  ]);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const isLastStep = currentStep === 2;
  const canAdvance = currentStepValid();
  // On the last step, allow creating even with no roles (they're optional)
  const canCreate = isLastStep && rolesStepValid();

  return (
    <>
      <DialogPrimitive.Root open={open} onOpenChange={(o) => !o && handleCloseRequest()}>
        <DialogPrimitive.Portal>
          {/* Overlay */}
          <DialogPrimitive.Overlay
            className="fixed inset-0 z-50 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
            style={{ backgroundColor: "rgba(0,0,0,0.75)", backdropFilter: "blur(2px)" }}
          />

          {/* Modal */}
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
            {/* Header */}
            <div
              className="flex items-start justify-between gap-3 px-5 pt-5 pb-4 shrink-0"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}
            >
              <div className="flex items-center gap-3 min-w-0">
                <div
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--radius-token-sm)]"
                  style={{ backgroundColor: "rgba(255,255,255,0.06)" }}
                >
                  <FolderOpen className="h-5 w-5 text-[var(--brand-primary)]" />
                </div>
                <div className="min-w-0">
                  <DialogPrimitive.Title
                    className="text-base font-semibold leading-tight"
                    style={{ color: "var(--text-primary)" }}
                  >
                    Create Project
                  </DialogPrimitive.Title>
                  <DialogPrimitive.Description
                    className="text-xs mt-0.5 line-clamp-1"
                    style={{ color: "var(--text-muted)" }}
                  >
                    Set up a new project with destinations and roles
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

            {/* Step progress */}
            <div
              className="px-5 py-4 shrink-0"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}
            >
              <StepBar current={currentStep} completed={completedSteps} />
            </div>

            {/* Step content — scrollable */}
            <div className="flex-1 overflow-y-auto px-5 py-5 min-h-0">
              {stepContent}
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
                disabled={currentStep === 0 || isLoading}
                className={cn(
                  "px-4 py-2 rounded-md text-sm font-medium transition-colors",
                  currentStep === 0 ? "invisible" : "hover:bg-[rgba(255,255,255,0.06)]",
                )}
                style={{ color: "var(--text-secondary)" }}
              >
                Back
              </button>

              <div className="flex items-center gap-3">
                {/* Cancel */}
                <button
                  type="button"
                  onClick={handleCloseRequest}
                  disabled={isLoading}
                  className="text-sm transition-colors hover:underline"
                  style={{ color: "var(--text-muted)" }}
                >
                  Cancel
                </button>

                {/* Next / Create */}
                {isLastStep ? (
                  <button
                    type="button"
                    onClick={handleCreate}
                    disabled={!canCreate || isLoading}
                    className="flex items-center gap-2 px-5 py-2 rounded-md text-sm font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{
                      backgroundColor:
                        canCreate && !isLoading
                          ? "var(--brand-primary)"
                          : "var(--text-muted)",
                      color: "var(--surface-bg)",
                    }}
                  >
                    {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
                    Create Project
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleNext}
                    disabled={!canAdvance || isLoading}
                    className="px-5 py-2 rounded-md text-sm font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{
                      backgroundColor:
                        canAdvance && !isLoading
                          ? "var(--brand-primary)"
                          : "rgba(255,255,255,0.10)",
                      color:
                        canAdvance && !isLoading
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
