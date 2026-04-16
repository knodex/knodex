// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DeployFormStepper } from "./DeployFormStepper";
import { DEPLOY_STEPS } from "./deploy-steps";

describe("DeployFormStepper", () => {
  beforeEach(() => {
    // Create mock section elements in the DOM for scrollIntoView
    DEPLOY_STEPS.forEach((step) => {
      const el = document.createElement("div");
      el.id = step.sectionId;
      el.scrollIntoView = vi.fn();
      document.body.appendChild(el);
    });
  });

  describe("vertical mode (default)", () => {
    it("renders all 4 step labels", () => {
      render(<DeployFormStepper activeStep={0} />);
      expect(screen.getByText("Instance Details")).toBeInTheDocument();
      expect(screen.getByText("Deployment Mode")).toBeInTheDocument();
      expect(screen.getByText("Configuration")).toBeInTheDocument();
      expect(screen.getByText("Review & Deploy")).toBeInTheDocument();
    });

    it("marks the active step with aria-current=step", () => {
      render(<DeployFormStepper activeStep={1} />);
      const buttons = screen.getAllByRole("button");
      expect(buttons[1]).toHaveAttribute("aria-current", "step");
      expect(buttons[0]).not.toHaveAttribute("aria-current");
    });

    it("scrolls to section on step click", () => {
      render(<DeployFormStepper activeStep={0} />);
      const configButton = screen.getByText("Configuration");
      fireEvent.click(configButton);
      const section = document.getElementById("deploy-configuration");
      expect(section?.scrollIntoView).toHaveBeenCalledWith({
        behavior: "smooth",
        block: "start",
      });
    });

    it("renders progress label", () => {
      render(<DeployFormStepper activeStep={0} />);
      expect(screen.getByText("Progress")).toBeInTheDocument();
    });

    it("has accessible nav landmark", () => {
      render(<DeployFormStepper activeStep={0} />);
      expect(screen.getByRole("navigation", { name: "Deploy form steps" })).toBeInTheDocument();
    });
  });

  describe("horizontal mode", () => {
    it("renders all 4 steps horizontally", () => {
      render(<DeployFormStepper horizontal activeStep={0} />);
      expect(screen.getByText("Instance Details")).toBeInTheDocument();
      expect(screen.getByText("Review & Deploy")).toBeInTheDocument();
    });

    it("marks active step with aria-current=step", () => {
      render(<DeployFormStepper horizontal activeStep={2} />);
      const buttons = screen.getAllByRole("button");
      expect(buttons[2]).toHaveAttribute("aria-current", "step");
    });

    it("scrolls to section on step click", () => {
      render(<DeployFormStepper horizontal activeStep={0} />);
      const reviewButton = screen.getByText("Review & Deploy");
      fireEvent.click(reviewButton);
      const section = document.getElementById("deploy-review");
      expect(section?.scrollIntoView).toHaveBeenCalledWith({
        behavior: "smooth",
        block: "start",
      });
    });
  });

  describe("step states", () => {
    it("shows past steps differently from future steps", () => {
      const { container } = render(<DeployFormStepper activeStep={2} />);
      const buttons = container.querySelectorAll("button");
      // Step 0 and 1 are past (index < activeStep)
      // Step 2 is active
      // Step 3 is future
      // Past steps should contain CheckCircle2 icon (svg)
      const pastStep = buttons[0];
      expect(pastStep.querySelector("svg")).toBeInTheDocument();
    });
  });

  describe("DEPLOY_STEPS export", () => {
    it("exports 4 steps with correct section IDs", () => {
      expect(DEPLOY_STEPS).toHaveLength(4);
      expect(DEPLOY_STEPS[0].sectionId).toBe("deploy-instance-details");
      expect(DEPLOY_STEPS[1].sectionId).toBe("deploy-deployment-mode");
      expect(DEPLOY_STEPS[2].sectionId).toBe("deploy-configuration");
      expect(DEPLOY_STEPS[3].sectionId).toBe("deploy-review");
    });
  });
});
