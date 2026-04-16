// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import { Check, Loader2 } from "@/lib/icons";
import { cn } from "@/lib/utils";

// --- Types ---

export interface WizardStep {
  id: string;
  label: string;
  component: React.ReactNode;
  isValid?: () => boolean;
}

export interface StepWizardRef {
  goToStep: (index: number) => void;
  getCurrentStep: () => number;
}

interface StepWizardProps {
  steps: WizardStep[];
  initialStep?: number;
  onComplete: () => void;
  actionLabel?: string;
  isSubmitting?: boolean;
}

// --- Component ---

export const StepWizard = forwardRef<StepWizardRef, StepWizardProps>(
  function StepWizard(
    { steps, initialStep = 0, onComplete, actionLabel = "Next", isSubmitting = false },
    ref
  ) {
    const [currentStep, setCurrentStep] = useState(initialStep);
    const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set());
    const contentRef = useRef<HTMLDivElement>(null);

    const isLastStep = currentStep === steps.length - 1;
    const step = steps[currentStep];
    const canAdvance = step?.isValid ? step.isValid() : true;

    useImperativeHandle(ref, () => ({
      goToStep: (index: number) => {
        if (index >= 0 && index < steps.length) {
          setCurrentStep(index);
        }
      },
      getCurrentStep: () => currentStep,
    }));

    // Focus first interactive element when step changes
    useEffect(() => {
      const timer = setTimeout(() => {
        if (!contentRef.current) return;
        const focusable = contentRef.current.querySelector<HTMLElement>(
          'input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled])'
        );
        focusable?.focus();
      }, 100);
      return () => clearTimeout(timer);
    }, [currentStep]);

    const handleNext = useCallback(() => {
      if (!canAdvance) return;
      if (isLastStep) {
        onComplete();
        return;
      }
      setCompletedSteps((prev) => new Set(prev).add(currentStep));
      setCurrentStep((prev) => prev + 1);
    }, [canAdvance, isLastStep, onComplete, currentStep]);

    const handleBack = useCallback(() => {
      if (currentStep > 0) {
        setCurrentStep((prev) => prev - 1);
      }
    }, [currentStep]);

    const finalButtonLabel = isLastStep ? actionLabel : "Next";

    return (
      <div data-testid="step-wizard">
        {/* Step indicator bar */}
        {steps.length > 1 && (
          <div className="flex items-center justify-center px-6 pt-6 pb-2">
            {steps.map((s, index) => {
              const isCompleted = completedSteps.has(index);
              const isCurrent = index === currentStep;
              const isFuture = !isCompleted && !isCurrent;

              return (
                <div key={s.id} className="flex items-center">
                  {/* Step circle */}
                  <div className="flex flex-col items-center">
                    <div
                      className={cn(
                        "flex h-8 w-8 items-center justify-center rounded-full text-xs font-semibold transition-colors",
                        isCompleted && "text-[var(--surface-bg)]",
                        isCurrent && "text-[var(--surface-bg)]",
                        isFuture && "border text-[var(--text-muted)]"
                      )}
                      style={{
                        backgroundColor: isCompleted || isCurrent
                          ? "var(--brand-primary)"
                          : "transparent",
                        borderColor: isFuture ? "var(--text-muted)" : undefined,
                      }}
                      aria-current={isCurrent ? "step" : undefined}
                    >
                      {isCompleted ? (
                        <Check className="h-4 w-4" />
                      ) : (
                        index + 1
                      )}
                    </div>
                    <span
                      className="mt-1 text-xs"
                      style={{
                        color: isCurrent || isCompleted
                          ? "var(--text-primary)"
                          : "var(--text-muted)",
                      }}
                    >
                      {s.label}
                    </span>
                  </div>

                  {/* Connector line */}
                  {index < steps.length - 1 && (
                    <div
                      className="mx-2 h-0.5 w-12"
                      style={{
                        backgroundColor: isCompleted
                          ? "var(--brand-primary)"
                          : "var(--text-muted)",
                        opacity: isCompleted ? 1 : 0.3,
                      }}
                    />
                  )}
                </div>
              );
            })}
          </div>
        )}

        {/* Step content */}
        <div ref={contentRef} className="px-6 py-4">
          {step?.component}
        </div>

        {/* Footer: Back + Next/Action */}
        <div className="flex items-center justify-between border-t px-6 py-4"
          style={{ borderColor: "rgba(255,255,255,0.06)" }}
        >
          <button
            type="button"
            onClick={handleBack}
            disabled={currentStep === 0 || isSubmitting}
            className={cn(
              "px-4 py-2 rounded-md text-sm font-medium transition-colors",
              currentStep === 0
                ? "invisible"
                : "hover:bg-[rgba(255,255,255,0.06)]"
            )}
            style={{ color: "var(--text-secondary)" }}
          >
            Back
          </button>
          <button
            type="button"
            onClick={handleNext}
            disabled={!canAdvance || isSubmitting}
            className={cn(
              "px-4 py-2 rounded-md text-sm font-medium transition-colors",
              "disabled:opacity-50 disabled:cursor-not-allowed"
            )}
            style={{
              backgroundColor: canAdvance && !isSubmitting
                ? "var(--brand-primary)"
                : "var(--text-muted)",
              color: "var(--surface-bg)",
            }}
          >
            {isSubmitting ? (
              <span className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                {finalButtonLabel}
              </span>
            ) : (
              finalButtonLabel
            )}
          </button>
        </div>
      </div>
    );
  }
);
