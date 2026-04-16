// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CollectionDefinitionNode } from "./CollectionDefinitionNode";
import type { Iterator } from "@/types/rgd";

// Mock @xyflow/react
vi.mock("@xyflow/react", () => ({
  Handle: () => null,
  Position: { Left: "left", Right: "right" },
}));

function makeProps(overrides: Record<string, unknown> = {}) {
  const defaultData = {
    label: "Pod",
    apiVersion: "v1",
    kind: "Pod",
    isTemplate: true,
    isConditional: false,
    hasExternalRef: false,
    isCollection: true,
    forEach: [
      {
        name: "worker",
        expression: "schema.spec.workers",
        source: "schema" as const,
        sourcePath: "spec.workers",
        dimensionIndex: 0,
      },
    ] as Iterator[],
    readyWhen: ["each.status.conditions.exists(c, c.type == 'Ready')"],
    resourceId: "workerPods",
    ...overrides,
  };

  return {
    id: "workerPods",
    type: "collection" as const,
    data: defaultData,
    // NodeProps fields that React Flow provides
    selected: false,
    isConnectable: true,
    zIndex: 0,
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    dragging: false,
    dragHandle: undefined,
    parentId: undefined,
    deletable: true,
    selectable: true,
    width: 200,
    height: 120,
  } as Parameters<typeof CollectionDefinitionNode>[0];
}

describe("CollectionDefinitionNode", () => {
  it("renders kind and resource ID", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);
    expect(screen.getByText(/Pod — workerPods/)).toBeInTheDocument();
  });

  it("renders forEach badge", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);
    expect(screen.getByText("forEach")).toBeInTheDocument();
  });

  it("renders apiVersion", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);
    expect(screen.getByText("v1")).toBeInTheDocument();
  });

  it("shows dynamic badge when all iterators are resource-sourced", () => {
    const resourceIterators: Iterator[] = [
      {
        name: "replica",
        expression: "status.replicas",
        source: "resource",
        sourcePath: "status.replicas",
        dimensionIndex: 0,
      },
    ];
    render(
      <CollectionDefinitionNode
        {...makeProps({ forEach: resourceIterators })}
      />
    );
    expect(screen.getByText("dynamic")).toBeInTheDocument();
  });

  it("does not show dynamic badge when iterators are schema-sourced", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);
    expect(screen.queryByText("dynamic")).not.toBeInTheDocument();
  });

  it("expands preview on click showing forEach details", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);
    // Initially no forEach details visible
    expect(screen.queryByText("worker:")).not.toBeInTheDocument();

    // Click to expand
    const card = screen.getByText(/Pod — workerPods/).closest("[class*='cursor-pointer']")!;
    fireEvent.click(card);

    // Now forEach details should be visible
    expect(screen.getByText("worker:")).toBeInTheDocument();
    expect(screen.getByText("schema.spec.workers")).toBeInTheDocument();
  });

  it("shows readyWhen in expanded preview", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);

    // Click to expand
    const card = screen.getByText(/Pod — workerPods/).closest("[class*='cursor-pointer']")!;
    fireEvent.click(card);

    expect(screen.getByText("readyWhen:")).toBeInTheDocument();
    expect(
      screen.getByText("each.status.conditions.exists(c, c.type == 'Ready')")
    ).toBeInTheDocument();
  });

  it("shows includeWhen in expanded preview when conditional", () => {
    render(
      <CollectionDefinitionNode
        {...makeProps({
          isConditional: true,
          conditionExpr: "schema.spec.enableWorkers == true",
        })}
      />
    );

    // Click to expand
    const card = screen.getByText(/Pod — workerPods/).closest("[class*='cursor-pointer']")!;
    fireEvent.click(card);

    expect(screen.getByText("includeWhen:")).toBeInTheDocument();
    expect(
      screen.getByText("schema.spec.enableWorkers == true")
    ).toBeInTheDocument();
  });

  it("renders source badge with correct label", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);

    // Click to expand
    const card = screen.getByText(/Pod — workerPods/).closest("[class*='cursor-pointer']")!;
    fireEvent.click(card);

    // Schema source badge shows "S"
    expect(screen.getByText("S")).toBeInTheDocument();
  });

  it("collapses preview on second click", () => {
    render(<CollectionDefinitionNode {...makeProps()} />);

    const card = screen.getByText(/Pod — workerPods/).closest("[class*='cursor-pointer']")!;

    // Expand
    fireEvent.click(card);
    expect(screen.getByText("worker:")).toBeInTheDocument();

    // Collapse
    fireEvent.click(card);
    expect(screen.queryByText("worker:")).not.toBeInTheDocument();
  });
});
