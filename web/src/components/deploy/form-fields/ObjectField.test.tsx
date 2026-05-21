// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { FormProvider, useForm } from "react-hook-form";
import type { FormProperty } from "@/types/rgd";
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

  it("renders a bold section header and all children directly", () => {
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
    // Bold header text visible
    expect(screen.getByText("Bastion")).toBeInTheDocument();
    // No collapsible button, no Advanced Configuration toggle
    expect(screen.queryByRole("button", { name: /Bastion/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Advanced Configuration/i)).not.toBeInTheDocument();
    // Child fields visible directly
    expect(screen.getByTestId("field-bastion.enabled")).toBeInTheDocument();
    expect(screen.getByTestId("field-bastion.subnetPrefix")).toBeInTheDocument();
  });

  it("shows nested advanced children directly without a toggle", () => {
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

    render(
      <Wrapper defaultValues={{ bastion: { enabled: true } }}>
        <ObjectField
          name="bastion"
          label="Bastion"
          property={propertyWithAdvanced}
          depth={0}
          deploymentNamespace="default"
        />
      </Wrapper>
    );

    // No toggle; all children including "advanced" are directly visible
    expect(screen.queryByText(/Advanced Configuration/i)).not.toBeInTheDocument();
    expect(screen.getByTestId("field-bastion.enabled")).toBeInTheDocument();
    expect(screen.getByTestId("field-bastion.subnetPrefix")).toBeInTheDocument();
    // "advanced" renders as a nested ObjectField (bold header) with its own testid
    expect(screen.getByTestId("field-bastion.advanced")).toBeInTheDocument();
  });

  it("hides peer fields when enabled is false", () => {
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

    render(
      <Wrapper defaultValues={{ bastion: { enabled: false } }}>
        <ObjectField
          name="bastion"
          label="Bastion"
          property={propertyWithAdvanced}
          depth={0}
          deploymentNamespace="default"
        />
      </Wrapper>
    );

    // "enabled" checkbox always visible
    expect(screen.getByTestId("field-bastion.enabled")).toBeInTheDocument();
    // Peer fields hidden when disabled
    expect(screen.queryByTestId("field-bastion.subnetPrefix")).not.toBeInTheDocument();
    expect(screen.queryByTestId("field-bastion.advanced")).not.toBeInTheDocument();
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

    // All fields visible immediately, no button needed
    expect(screen.getByTestId("field-service.name")).toBeInTheDocument();
    expect(screen.getByTestId("field-service.port")).toBeInTheDocument();
  });
});
