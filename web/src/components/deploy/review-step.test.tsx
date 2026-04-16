// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ReviewStep } from "./review-step";

describe("ReviewStep", () => {
  it("renders project and namespace", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
      />
    );

    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
  });

  it("renders form values", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{ image: "postgres:15", storage: "10Gi" }}
      />
    );

    expect(screen.getByText("image")).toBeInTheDocument();
    expect(screen.getByText("postgres:15")).toBeInTheDocument();
    expect(screen.getByText("storage")).toBeInTheDocument();
    expect(screen.getByText("10Gi")).toBeInTheDocument();
  });

  it("hides namespace for cluster-scoped RGDs", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace=""
        formValues={{}}
        isClusterScoped={true}
      />
    );

    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.queryByText("Namespace")).not.toBeInTheDocument();
  });

  it("shows warning banner for warning result", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
        complianceResult="warning"
        complianceViolations={[
          { policy: "require-registry", severity: "warning", message: "Use private registry" },
        ]}
      />
    );

    expect(screen.getByText("require-registry")).toBeInTheDocument();
    expect(screen.getByText("Use private registry")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("shows block banner for block result", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
        complianceResult="block"
        complianceViolations={[
          { policy: "require-labels", severity: "error", message: "Missing labels" },
        ]}
      />
    );

    expect(screen.getByText("require-labels")).toBeInTheDocument();
    expect(screen.getByText("Missing labels")).toBeInTheDocument();
  });

  it("does not show banner for pass result", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
        complianceResult="pass"
        complianceViolations={[]}
      />
    );

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("calls onEditStep when edit button is clicked", () => {
    const onEditStep = vi.fn();
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{ image: "postgres:15" }}
        onEditStep={onEditStep}
      />
    );

    fireEvent.click(screen.getByLabelText("Edit configuration"));
    expect(onEditStep).toHaveBeenCalledWith(1);
  });

  it("shows acknowledge checkbox for warnings", () => {
    const onAcknowledge = vi.fn();
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
        complianceResult="warning"
        complianceViolations={[
          { policy: "test", severity: "warning", message: "test warning" },
        ]}
        onAcknowledgeWarnings={onAcknowledge}
      />
    );

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    expect(onAcknowledge).toHaveBeenCalled();
  });

  it("shows empty state for no form values", () => {
    render(
      <ReviewStep
        project="alpha"
        namespace="default"
        formValues={{}}
      />
    );

    expect(screen.getByText("No configuration values")).toBeInTheDocument();
  });
});
