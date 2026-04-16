// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StepWizard, type WizardStep } from "./step-wizard";

function createSteps(overrides?: Partial<WizardStep>[]): WizardStep[] {
  const defaults: WizardStep[] = [
    { id: "step-1", label: "Step 1", component: <div>Step 1 content <input data-testid="step1-input" /></div>, isValid: () => true },
    { id: "step-2", label: "Step 2", component: <div>Step 2 content <input data-testid="step2-input" /></div>, isValid: () => true },
    { id: "step-3", label: "Step 3", component: <div>Step 3 content</div>, isValid: () => true },
  ];
  if (overrides) {
    overrides.forEach((o, i) => {
      if (o) defaults[i] = { ...defaults[i], ...o };
    });
  }
  return defaults;
}

describe("StepWizard", () => {
  it("renders step indicator with all steps", () => {
    render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    expect(screen.getByText("Step 1")).toBeInTheDocument();
    expect(screen.getByText("Step 2")).toBeInTheDocument();
    expect(screen.getByText("Step 3")).toBeInTheDocument();
  });

  it("renders current step content", () => {
    render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    expect(screen.getByText("Step 1 content")).toBeInTheDocument();
  });

  it("highlights current step with aria-current", () => {
    const { container } = render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    const currentIndicator = container.querySelector('[aria-current="step"]');
    expect(currentIndicator).toBeInTheDocument();
    expect(currentIndicator?.textContent).toBe("1");
  });

  it("advances to next step on Next click", () => {
    render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    fireEvent.click(screen.getByText("Next"));

    expect(screen.getByText("Step 2 content")).toBeInTheDocument();
  });

  it("shows checkmark on completed steps", () => {
    const { container } = render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    // Advance to step 2 (step 1 becomes completed)
    fireEvent.click(screen.getByText("Next"));

    // Check SVG exists in the completed step circle (Lucide Check renders as SVG)
    const svgs = container.querySelectorAll("svg.lucide-check");
    expect(svgs.length).toBeGreaterThan(0);
  });

  it("disables Next when step isValid returns false", () => {
    const steps = createSteps([{ isValid: () => false }]);
    render(<StepWizard steps={steps} onComplete={vi.fn()} />);

    const nextButton = screen.getByText("Next");
    expect(nextButton).toBeDisabled();
  });

  it("preserves previous step content on Back", () => {
    render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);

    // Advance to step 2
    fireEvent.click(screen.getByText("Next"));
    expect(screen.getByText("Step 2 content")).toBeInTheDocument();

    // Go back
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Step 1 content")).toBeInTheDocument();
  });

  it("calls onComplete on last step Next click", () => {
    const onComplete = vi.fn();
    render(<StepWizard steps={createSteps()} onComplete={onComplete} />);

    // Advance through all steps
    fireEvent.click(screen.getByText("Next")); // step 1 → 2
    fireEvent.click(screen.getByText("Next")); // step 2 → 3

    // On last step, button should show actionLabel (default "Next")
    fireEvent.click(screen.getByText("Next")); // complete

    expect(onComplete).toHaveBeenCalled();
  });

  it("uses custom actionLabel on last step", () => {
    render(
      <StepWizard
        steps={createSteps()}
        onComplete={vi.fn()}
        actionLabel="Deploy"
      />
    );

    // Advance to last step
    fireEvent.click(screen.getByText("Next")); // step 1 → 2
    fireEvent.click(screen.getByText("Next")); // step 2 → 3

    expect(screen.getByText("Deploy")).toBeInTheDocument();
  });

  it("shows spinner when isSubmitting is true", () => {
    const steps: WizardStep[] = [
      { id: "only", label: "Only Step", component: <div>Content</div>, isValid: () => true },
    ];
    const { container } = render(
      <StepWizard steps={steps} onComplete={vi.fn()} isSubmitting={true} actionLabel="Deploy" />
    );

    // Spinner should be visible (Loader2 with animate-spin)
    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });

  it("hides step indicator for single step", () => {
    const steps: WizardStep[] = [
      { id: "only", label: "Only Step", component: <div>Content</div> },
    ];
    render(<StepWizard steps={steps} onComplete={vi.fn()} />);

    // Should not render step circles
    expect(screen.queryByText("Only Step")).not.toBeInTheDocument();
    // But content should render
    expect(screen.getByText("Content")).toBeInTheDocument();
  });

  it("has data-testid for integration tests", () => {
    render(<StepWizard steps={createSteps()} onComplete={vi.fn()} />);
    expect(screen.getByTestId("step-wizard")).toBeInTheDocument();
  });
});
