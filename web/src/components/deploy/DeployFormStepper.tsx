// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { CheckCircle2 } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { DEPLOY_STEPS } from "./deploy-steps";

interface DeployFormStepperProps {
  className?: string;
  /** Render as horizontal strip instead of vertical sidebar */
  horizontal?: boolean;
  /** Active step index — managed by parent to share a single IntersectionObserver across instances */
  activeStep: number;
}

export function DeployFormStepper({ className, horizontal = false, activeStep }: DeployFormStepperProps) {
  const handleStepClick = useCallback((sectionId: string) => {
    const el = document.getElementById(sectionId);
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "start" });
    } else if (import.meta.env.DEV) {
      console.warn(`[DeployFormStepper] Section element #${sectionId} not found in DOM`);
    }
  }, []);

  if (horizontal) {
    return (
      <nav
        className={cn(
          "flex items-stretch gap-1 rounded-lg border border-border bg-card p-1",
          className
        )}
        aria-label="Deploy form steps"
      >
        {DEPLOY_STEPS.map((step, index) => {
          const isActive = activeStep === index;
          const isPast = index < activeStep;
          return (
            <button
              key={step.id}
              onClick={() => handleStepClick(step.sectionId)}
              aria-current={isActive ? "step" : undefined}
              className={cn(
                "flex flex-1 items-center justify-center gap-1.5 px-2 py-1.5 rounded-md text-xs font-medium transition-colors",
                isActive
                  ? "bg-primary text-primary-foreground"
                  : isPast
                  ? "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                  : "text-muted-foreground/60 hover:text-muted-foreground hover:bg-muted/30"
              )}
            >
              <span
                className={cn(
                  "flex h-4 w-4 items-center justify-center rounded-full text-[10px] font-bold shrink-0",
                  isActive
                    ? "bg-primary-foreground/20 text-primary-foreground"
                    : isPast
                    ? "bg-muted text-muted-foreground"
                    : "bg-muted/60 text-muted-foreground/60"
                )}
                aria-hidden="true"
              >
                {isPast ? <CheckCircle2 className="h-3 w-3" /> : index + 1}
              </span>
              <span className="hidden sm:block truncate">{step.label}</span>
              <span className="sm:hidden">{index + 1}</span>
            </button>
          );
        })}
      </nav>
    );
  }

  return (
    <nav
      className={cn("sticky top-4 flex flex-col gap-1 w-44", className)}
      aria-label="Deploy form steps"
    >
      <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground px-3 mb-1">
        Progress
      </p>
      {DEPLOY_STEPS.map((step, index) => {
        const isActive = activeStep === index;
        const isPast = index < activeStep;
        return (
          <button
            key={step.id}
            onClick={() => handleStepClick(step.sectionId)}
            aria-current={isActive ? "step" : undefined}
            className={cn(
              "flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-left transition-colors w-full",
              isActive
                ? "bg-primary/10 text-primary font-medium"
                : isPast
                ? "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                : "text-muted-foreground/60 hover:text-muted-foreground hover:bg-muted/30"
            )}
          >
            <span
              className={cn(
                "flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold",
                isActive
                  ? "bg-primary text-primary-foreground"
                  : isPast
                  ? "bg-muted text-muted-foreground"
                  : "bg-muted/60 text-muted-foreground/60"
              )}
              aria-hidden="true"
            >
              {isPast ? <CheckCircle2 className="h-4 w-4" /> : index + 1}
            </span>
            <span className="truncate">{step.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
