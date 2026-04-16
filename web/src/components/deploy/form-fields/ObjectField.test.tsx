// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { FormProvider, useForm } from "react-hook-form";
import type { FormProperty, AdvancedSection } from "@/types/rgd";
import { ObjectField } from "./ObjectField";

function Wrapper({ children, defaultValues = {} }: { children: React.ReactNode; defaultValues?: Record<string, unknown> }) {
  const methods = useForm({ defaultValues });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

describe("ObjectField", () => {
  const baseProperty: FormProperty = {
    type: "object",
    properties: {
      enabled: { type: "boolean" } as FormProperty,
      subnetPrefix: { type: "string" } as FormProperty,
    },
  };

  it("renders all children normally without inlineAdvancedSection", () => {
    render(
      <Wrapper defaultValues={{ bastion: { enabled: true } }}>
        <ObjectField
          name="bastion"
          label="Bastion"
          property={baseProperty}
          depth={0}
          deploymentNamespace="default"
        />
      </Wrapper>
    );

    expect(screen.getByTestId("field-bastion")).toBeInTheDocument();
    expect(screen.queryByText(/Advanced Configuration/i)).not.toBeInTheDocument();
  });

  it("renders AdvancedConfigToggle wrapping advanced children when inlineAdvancedSection is set and enabled", () => {
    const propertyWithAdvanced: FormProperty = {
      type: "object",
      properties: {
        enabled: { type: "boolean" } as FormProperty,
        subnetPrefix: { type: "string" } as FormProperty,
        advanced: {
          type: "object",
          properties: {
            asoCredentialSecretName: { type: "string" } as FormProperty,
          },
        } as FormProperty,
      },
    };

    const inlineSection: AdvancedSection = {
      path: "bastion.advanced",
      affectedProperties: ["bastion.advanced.asoCredentialSecretName"],
    };

    render(
      <Wrapper defaultValues={{ bastion: { enabled: true } }}>
        <ObjectField
          name="bastion"
          label="Bastion"
          property={propertyWithAdvanced}
          depth={0}
          deploymentNamespace="default"
          inlineAdvancedSection={inlineSection}
        />
      </Wrapper>
    );

    // Should render the AdvancedConfigToggle
    expect(screen.getByText(/Advanced Configuration/i)).toBeInTheDocument();
    // Should NOT render "advanced" as a plain collapsible sibling
    expect(screen.queryByTestId("field-bastion.advanced")).not.toBeInTheDocument();
    // Regular children must still be present above the toggle
    expect(screen.getByTestId("field-bastion.enabled")).toBeInTheDocument();
    expect(screen.getByTestId("field-bastion.subnetPrefix")).toBeInTheDocument();
  });

  it("hides peer fields and advanced section when enabled is false", () => {
    const propertyWithAdvanced: FormProperty = {
      type: "object",
      properties: {
        enabled: { type: "boolean" } as FormProperty,
        subnetPrefix: { type: "string" } as FormProperty,
        advanced: {
          type: "object",
          properties: {
            asoCredentialSecretName: { type: "string" } as FormProperty,
          },
        } as FormProperty,
      },
    };

    const inlineSection: AdvancedSection = {
      path: "bastion.advanced",
      affectedProperties: ["bastion.advanced.asoCredentialSecretName"],
    };

    render(
      <Wrapper defaultValues={{ bastion: { enabled: false } }}>
        <ObjectField
          name="bastion"
          label="Bastion"
          property={propertyWithAdvanced}
          depth={0}
          deploymentNamespace="default"
          inlineAdvancedSection={inlineSection}
        />
      </Wrapper>
    );

    // Enabled toggle is always visible
    expect(screen.getByTestId("field-bastion.enabled")).toBeInTheDocument();
    // Peer fields and advanced section are hidden
    expect(screen.queryByTestId("field-bastion.subnetPrefix")).not.toBeInTheDocument();
    expect(screen.queryByText(/Advanced Configuration/i)).not.toBeInTheDocument();
  });

  it("shows all fields for objects without enabled toggle pattern", () => {
    const simpleProperty: FormProperty = {
      type: "object",
      properties: {
        name: { type: "string" } as FormProperty,
        port: { type: "integer" } as FormProperty,
      },
    };

    render(
      <Wrapper>
        <ObjectField
          name="service"
          label="Service"
          property={simpleProperty}
          depth={0}
          deploymentNamespace="default"
        />
      </Wrapper>
    );

    expect(screen.getByTestId("field-service.name")).toBeInTheDocument();
    expect(screen.getByTestId("field-service.port")).toBeInTheDocument();
  });
});
