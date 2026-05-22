// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { FormProvider, useForm } from "react-hook-form";
import type { FormProperty } from "@/types/rgd";
import type { DeployTab } from "@/lib/build-tabs";
import { SchemaTab } from "./SchemaTab";

function Wrapper({
  children,
  defaultValues = {},
}: {
  children: React.ReactNode;
  defaultValues?: Record<string, unknown>;
}) {
  const methods = useForm({ defaultValues });
  return <FormProvider {...methods}>{children}</FormProvider>;
}

describe("SchemaTab", () => {
  const tabWithEnabledToggle: DeployTab = {
    id: "azurePostgresql",
    kind: "schema",
    label: "Azure Postgresql",
    properties: {
      enabled: { type: "boolean" } as FormProperty,
      version: { type: "string" } as FormProperty,
      storageGB: { type: "integer" } as FormProperty,
    },
    propertyOrder: ["enabled", "version", "storageGB"],
  };

  it("hides peer fields when enabled is false (feature-toggle regression)", () => {
    render(
      <Wrapper defaultValues={{ azurePostgresql: { enabled: false } }}>
        <SchemaTab tab={tabWithEnabledToggle} />
      </Wrapper>
    );

    expect(
      screen.getByTestId("field-azurePostgresql.enabled")
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("field-azurePostgresql.version")
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("field-azurePostgresql.storageGB")
    ).not.toBeInTheDocument();
  });

  it("shows peer fields when enabled is true", () => {
    render(
      <Wrapper defaultValues={{ azurePostgresql: { enabled: true } }}>
        <SchemaTab tab={tabWithEnabledToggle} />
      </Wrapper>
    );

    expect(
      screen.getByTestId("field-azurePostgresql.enabled")
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("field-azurePostgresql.version")
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("field-azurePostgresql.storageGB")
    ).toBeInTheDocument();
  });

  it("renders all fields when the tab has no enabled boolean (no gating)", () => {
    const tabNoToggle: DeployTab = {
      id: "networking",
      kind: "schema",
      label: "Networking",
      properties: {
        cidr: { type: "string" } as FormProperty,
        subnetPrefix: { type: "string" } as FormProperty,
      },
      propertyOrder: ["cidr", "subnetPrefix"],
    };

    render(
      <Wrapper>
        <SchemaTab tab={tabNoToggle} />
      </Wrapper>
    );

    expect(screen.getByTestId("field-networking.cidr")).toBeInTheDocument();
    expect(
      screen.getByTestId("field-networking.subnetPrefix")
    ).toBeInTheDocument();
  });
});
