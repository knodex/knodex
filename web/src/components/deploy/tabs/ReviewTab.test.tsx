// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FormProvider, useForm } from "react-hook-form";
import { ReviewTab } from "./ReviewTab";
import type { DeployTab } from "@/lib/build-tabs";
import type { ComplianceValidateViolation } from "@/api/compliance";

const SAMPLE_TABS: DeployTab[] = [
  { id: "general", kind: "general", label: "General" },
  { id: "review", kind: "review", label: "Review + Deploy" },
];

interface HarnessProps {
  complianceResult: "pass" | "warning" | "block";
  complianceViolations?: ComplianceValidateViolation[];
  warningsAcknowledged?: boolean;
  setWarningsAcknowledged?: (v: boolean) => void;
  preflightValid?: boolean;
  preflightMessage?: string;
}

function Harness({
  complianceResult,
  complianceViolations = [],
  warningsAcknowledged = false,
  setWarningsAcknowledged = () => undefined,
  preflightValid = true,
  preflightMessage,
}: HarnessProps) {
  const methods = useForm({
    defaultValues: {
      instanceName: "demo",
      project: "alpha",
      namespace: "alpha-dev",
      deploymentMode: "direct",
    },
  });
  return (
    <FormProvider {...methods}>
      <ReviewTab
        tabs={SAMPLE_TABS}
        onEditTab={() => undefined}
        complianceResult={complianceResult}
        complianceViolations={complianceViolations}
        warningsAcknowledged={warningsAcknowledged}
        setWarningsAcknowledged={setWarningsAcknowledged}
        preflightValid={preflightValid}
        preflightMessage={preflightMessage}
        isValidating={false}
        isPreflighting={false}
        isClusterScoped={false}
      />
    </FormProvider>
  );
}

const WARNING_VIOLATION: ComplianceValidateViolation = {
  policy: "policy/x",
  severity: "warning",
  message: "minor concern",
};

describe("ReviewTab", () => {
  it("hides acknowledgment + preflight banner on pass + valid", () => {
    render(<Harness complianceResult="pass" preflightValid />);
    expect(
      screen.queryByTestId("compliance-acknowledge-checkbox")
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("preflight-alert")).not.toBeInTheDocument();
  });

  it("shows acknowledgment checkbox for warning result and forwards toggles", () => {
    const setAck = vi.fn();
    render(
      <Harness
        complianceResult="warning"
        complianceViolations={[WARNING_VIOLATION]}
        setWarningsAcknowledged={setAck}
      />
    );
    const checkbox = screen.getByTestId("compliance-acknowledge-checkbox");
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(setAck).toHaveBeenCalledWith(true);
  });

  it("does not render checkbox when result is block", () => {
    render(
      <Harness
        complianceResult="block"
        complianceViolations={[
          { ...WARNING_VIOLATION, severity: "error" },
        ]}
      />
    );
    expect(
      screen.queryByTestId("compliance-acknowledge-checkbox")
    ).not.toBeInTheDocument();
  });

  it("omits namespace row in the General card when cluster-scoped", () => {
    function ClusterScopedHarness() {
      const methods = useForm({
        defaultValues: {
          instanceName: "demo",
          project: "alpha",
          deploymentMode: "direct",
        },
      });
      return (
        <FormProvider {...methods}>
          <ReviewTab
            tabs={SAMPLE_TABS}
            onEditTab={() => undefined}
            complianceResult="pass"
            complianceViolations={[]}
            warningsAcknowledged={false}
            setWarningsAcknowledged={() => undefined}
            preflightValid={true}
            isValidating={false}
            isPreflighting={false}
            isClusterScoped={true}
          />
        </FormProvider>
      );
    }
    render(<ClusterScopedHarness />);
    expect(screen.getByTestId("review-card-general")).toBeInTheDocument();
    expect(screen.queryByText("Namespace")).not.toBeInTheDocument();
  });

  it("renders preflight banner with the message when invalid", () => {
    render(
      <Harness
        complianceResult="pass"
        preflightValid={false}
        preflightMessage="kubernetes admission denied"
      />
    );
    const banner = screen.getByTestId("preflight-alert");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveTextContent("kubernetes admission denied");
  });
});
