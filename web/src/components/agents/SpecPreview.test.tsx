// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SpecPreview } from "./SpecPreview";

const RGD_YAML = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
    - id: redis
      template:
        apiVersion: redis.example.io/v1
        kind: RedisCluster`;

describe("SpecPreview", () => {
  it("renders the structured RGD view by default", () => {
    render(<SpecPreview yamlBlock={RGD_YAML} />);

    expect(screen.getByTestId("spec-structured-view")).toBeInTheDocument();
    expect(screen.getByTestId("spec-rgd-name")).toHaveTextContent("webapp-stack");
    expect(screen.getByTestId("spec-schema-kind")).toHaveTextContent("WebAppStack");
    expect(screen.getByTestId("spec-schema-apiversion")).toHaveTextContent("v1alpha1");

    const table = screen.getByTestId("spec-resources-table");
    expect(table).toHaveTextContent("deployment");
    expect(table).toHaveTextContent("Deployment");
    expect(table).toHaveTextContent("RedisCluster");
    expect(table).toHaveTextContent("redis.example.io/v1");
  });

  it("toggles to the raw YAML view and back", async () => {
    const user = userEvent.setup();
    render(<SpecPreview yamlBlock={RGD_YAML} />);

    await user.click(screen.getByTestId("spec-tab-yaml"));
    expect(screen.getByTestId("spec-yaml-view")).toHaveTextContent("kind: ResourceGraphDefinition");
    expect(screen.queryByTestId("spec-structured-view")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("spec-tab-structured"));
    expect(screen.getByTestId("spec-structured-view")).toBeInTheDocument();
  });

  it("falls back to a generic key/value view for non-RGD specs", () => {
    render(<SpecPreview yamlBlock={"kind: Deployment\nmetadata:\n  name: web"} />);

    expect(screen.getByTestId("spec-generic-view")).toBeInTheDocument();
    expect(screen.queryByTestId("spec-structured-view")).not.toBeInTheDocument();
    expect(screen.getByText("kind")).toBeInTheDocument();
    expect(screen.getByText("Deployment")).toBeInTheDocument();
  });

  it("renders the raw block with a parse notice when YAML is invalid", () => {
    render(<SpecPreview yamlBlock={"kind: [unclosed"} />);

    expect(screen.getByTestId("spec-parse-notice")).toBeInTheDocument();
    expect(screen.getByTestId("spec-yaml-view")).toHaveTextContent("kind: [unclosed");
    // No toggle — there is nothing structured to show.
    expect(screen.queryByTestId("spec-tab-structured")).not.toBeInTheDocument();
  });

  it("shows an empty-resources message when the spec has none", () => {
    render(
      <SpecPreview yamlBlock={"kind: ResourceGraphDefinition\nmetadata:\n  name: empty"} />
    );

    expect(screen.getByText("No resources in this spec.")).toBeInTheDocument();
  });

  // --- Story 50.2 AC #2: requirement → resource traceability view ---

  const ANNOTATED_RGD_YAML = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          annotations:
            knodex.io/generated-from: "a web app"
    - id: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          annotations:
            knodex.io/generated-from: "a web app"
    - id: redis
      template:
        apiVersion: redis.example.io/v1
        kind: RedisCluster
        metadata:
          annotations:
            knodex.io/generated-from: "with redis"`;

  it("renders traceability groups mapping resources to their requirement", () => {
    render(<SpecPreview yamlBlock={ANNOTATED_RGD_YAML} />);

    const section = screen.getByTestId("spec-traceability");
    expect(section).toHaveTextContent("Generated from requirements");

    const groups = screen.getAllByTestId("spec-traceability-group");
    expect(groups).toHaveLength(2);

    // First group: "a web app" → deployment + service.
    expect(groups[0]).toHaveTextContent("a web app");
    expect(groups[0]).toHaveTextContent("deployment");
    expect(groups[0]).toHaveTextContent("Service");
    expect(groups[0]).not.toHaveTextContent("RedisCluster");

    // Second group: "with redis" → redis only.
    expect(groups[1]).toHaveTextContent("with redis");
    expect(groups[1]).toHaveTextContent("RedisCluster");
  });

  it("labels unannotated resources as 'No requirement recorded'", () => {
    render(<SpecPreview yamlBlock={RGD_YAML} />);

    const section = screen.getByTestId("spec-traceability");
    expect(section).toHaveTextContent("No requirement recorded");
    const groups = screen.getAllByTestId("spec-traceability-group");
    expect(groups).toHaveLength(1);
  });

  it("hides the traceability section when the spec has zero resources", () => {
    render(
      <SpecPreview yamlBlock={"kind: ResourceGraphDefinition\nmetadata:\n  name: empty"} />
    );

    expect(screen.queryByTestId("spec-traceability")).not.toBeInTheDocument();
  });

  it("does not render traceability in the generic non-RGD branch", () => {
    render(<SpecPreview yamlBlock={"kind: Deployment\nmetadata:\n  name: web"} />);

    expect(screen.queryByTestId("spec-traceability")).not.toBeInTheDocument();
  });
});
