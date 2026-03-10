// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResourceGraphView } from "./ResourceGraphView";
import type { ResourceGraph } from "@/types/rgd";

// Mock @xyflow/react to avoid DOM measurement issues in tests
vi.mock("@xyflow/react", () => ({
  ReactFlow: ({ children }: { children?: React.ReactNode }) => (
    <div className="react-flow" data-testid="react-flow-mock">
      {children}
    </div>
  ),
  Background: () => <div data-testid="react-flow-background" />,
  Controls: () => <div data-testid="react-flow-controls" />,
  MiniMap: () => <div data-testid="react-flow-minimap" />,
  useNodesState: (initialNodes: unknown[]) => [initialNodes, vi.fn(), vi.fn()],
  useEdgesState: (initialEdges: unknown[]) => [initialEdges, vi.fn(), vi.fn()],
  MarkerType: { ArrowClosed: "arrowclosed" },
  Position: { Left: "left", Right: "right" },
  Handle: () => null,
}));

const emptyGraph: ResourceGraph = {
  rgdName: "test-rgd",
  rgdNamespace: "default",
  resources: [],
  edges: [],
};

const simpleGraph: ResourceGraph = {
  rgdName: "test-rgd",
  rgdNamespace: "default",
  resources: [
    {
      id: "deployment-1",
      apiVersion: "apps/v1",
      kind: "Deployment",
      isTemplate: true,
      isConditional: false,
      dependsOn: [],
    },
    {
      id: "service-1",
      apiVersion: "v1",
      kind: "Service",
      isTemplate: true,
      isConditional: false,
      dependsOn: ["deployment-1"],
    },
  ],
  edges: [
    {
      from: "service-1",
      to: "deployment-1",
      type: "reference",
    },
  ],
};

const complexGraph: ResourceGraph = {
  rgdName: "complex-rgd",
  rgdNamespace: "default",
  resources: [
    {
      id: "deployment-1",
      apiVersion: "apps/v1",
      kind: "Deployment",
      isTemplate: true,
      isConditional: false,
      dependsOn: [],
    },
    {
      id: "service-1",
      apiVersion: "v1",
      kind: "Service",
      isTemplate: true,
      isConditional: false,
      dependsOn: ["deployment-1"],
    },
    {
      id: "configmap-1",
      apiVersion: "v1",
      kind: "ConfigMap",
      isTemplate: false,
      isConditional: false,
      dependsOn: [],
      externalRef: {
        apiVersion: "v1",
        kind: "ConfigMap",
        nameExpr: "existing-config",
        usesSchemaSpec: false,
      },
    },
    {
      id: "secret-1",
      apiVersion: "v1",
      kind: "Secret",
      isTemplate: true,
      isConditional: true,
      conditionExpr: "spec.enableSSL == true",
      dependsOn: [],
    },
  ],
  edges: [
    {
      from: "service-1",
      to: "deployment-1",
      type: "reference",
    },
    {
      from: "deployment-1",
      to: "configmap-1",
      type: "reference",
    },
  ],
};

describe("ResourceGraphView", () => {
  describe("empty state", () => {
    it("renders empty state when no resources", () => {
      render(<ResourceGraphView resourceGraph={emptyGraph} />);
      expect(screen.getByText("No resources defined")).toBeInTheDocument();
      expect(
        screen.getByText("This RGD doesn't have any resources in its spec")
      ).toBeInTheDocument();
    });

    it("shows AlertCircle icon in empty state", () => {
      const { container } = render(
        <ResourceGraphView resourceGraph={emptyGraph} />
      );
      const icon = container.querySelector("svg");
      expect(icon).toBeInTheDocument();
    });
  });

  describe("graph statistics", () => {
    it("displays total resource count", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      // Find the resources span and check it contains the count
      const resourcesText = screen.getByText(/resources/);
      expect(resourcesText).toBeInTheDocument();
      expect(resourcesText.textContent).toContain("2");
    });

    it("displays template count", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      // Find the templates span and check it contains the count
      const templatesText = screen.getByText(/templates/);
      expect(templatesText).toBeInTheDocument();
      expect(templatesText.textContent).toContain("2");
    });

    it("displays external ref count when present", () => {
      render(<ResourceGraphView resourceGraph={complexGraph} />);
      // Find the external refs span and check it contains the count
      const externalRefsText = screen.getByText(/external refs/);
      expect(externalRefsText).toBeInTheDocument();
      expect(externalRefsText.textContent).toContain("1");
    });

    it("displays conditional count when present", () => {
      render(<ResourceGraphView resourceGraph={complexGraph} />);
      // Find the conditional span and check it contains the count
      const conditionalText = screen.getByText(/conditional/);
      expect(conditionalText).toBeInTheDocument();
      expect(conditionalText.textContent).toContain("1");
    });

    it("does not display external refs when none exist", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      expect(screen.queryByText("external refs")).not.toBeInTheDocument();
    });

    it("does not display conditional when none exist", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      expect(screen.queryByText("conditional")).not.toBeInTheDocument();
    });
  });

  describe("graph legend", () => {
    it("renders all legend items", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      expect(screen.getByText("Template")).toBeInTheDocument();
      expect(screen.getByText("ExternalRef")).toBeInTheDocument();
      expect(screen.getByText("Conditional (includeWhen)")).toBeInTheDocument();
      expect(screen.getByText("Dependency")).toBeInTheDocument();
    });

    it("renders conditional badge in legend", () => {
      render(<ResourceGraphView resourceGraph={simpleGraph} />);
      const conditionalBadge = screen.getByText("?");
      expect(conditionalBadge).toBeInTheDocument();
      expect(conditionalBadge).toHaveClass("bg-graph-conditional/20");
    });
  });

  describe("graph rendering", () => {
    it("renders ReactFlow container", () => {
      const { container } = render(
        <ResourceGraphView resourceGraph={simpleGraph} />
      );
      const flowContainer = container.querySelector(".react-flow");
      expect(flowContainer).toBeInTheDocument();
    });

    it("renders graph with correct height", () => {
      const { container } = render(
        <ResourceGraphView resourceGraph={simpleGraph} />
      );
      const graphContainer = container.querySelector(".h-\\[400px\\]");
      expect(graphContainer).toBeInTheDocument();
    });
  });

  describe("statistics calculation", () => {
    it("calculates correct statistics for complex graph", () => {
      render(<ResourceGraphView resourceGraph={complexGraph} />);

      // Total resources (4 resources)
      const resourcesText = screen.getByText(/resources/);
      expect(resourcesText.textContent).toContain("4");

      // Templates (3 templates: deployment, service, secret)
      const templatesText = screen.getByText(/templates/);
      expect(templatesText.textContent).toContain("3");

      // External refs (1: configmap)
      const externalText = screen.getByText(/external refs/);
      expect(externalText.textContent).toContain("1");

      // Conditional (1: secret)
      const conditionalText = screen.getByText(/conditional/);
      expect(conditionalText.textContent).toContain("1");
    });

    it("handles single resource graph", () => {
      const singleResourceGraph: ResourceGraph = {
        rgdName: "single-rgd",
        rgdNamespace: "default",
        resources: [
          {
            id: "deployment-1",
            apiVersion: "apps/v1",
            kind: "Deployment",
            isTemplate: true,
            isConditional: false,
            dependsOn: [],
          },
        ],
        edges: [],
      };

      render(<ResourceGraphView resourceGraph={singleResourceGraph} />);
      const resourcesText = screen.getByText(/resources/);
      expect(resourcesText.textContent).toContain("1");
    });
  });

  describe("edge cases", () => {
    it("handles graph with only external refs", () => {
      const externalOnlyGraph: ResourceGraph = {
        rgdName: "external-rgd",
        rgdNamespace: "default",
        resources: [
          {
            id: "configmap-1",
            apiVersion: "v1",
            kind: "ConfigMap",
            isTemplate: false,
            isConditional: false,
            dependsOn: [],
            externalRef: {
              apiVersion: "v1",
              kind: "ConfigMap",
              nameExpr: "existing-config",
              usesSchemaSpec: false,
            },
          },
        ],
        edges: [],
      };

      render(<ResourceGraphView resourceGraph={externalOnlyGraph} />);
      // Templates should show 0
      const templatesText = screen.getByText(/templates/);
      expect(templatesText.textContent).toContain("0");

      // External refs should show 1
      const externalText = screen.getByText(/external refs/);
      expect(externalText.textContent).toContain("1");
    });

    it("handles graph with only conditional resources", () => {
      const conditionalOnlyGraph: ResourceGraph = {
        rgdName: "conditional-rgd",
        rgdNamespace: "default",
        resources: [
          {
            id: "secret-1",
            apiVersion: "v1",
            kind: "Secret",
            isTemplate: true,
            isConditional: true,
            conditionExpr: "spec.enableSSL == true",
            dependsOn: [],
          },
        ],
        edges: [],
      };

      render(<ResourceGraphView resourceGraph={conditionalOnlyGraph} />);
      // Conditional should show 1
      const conditionalText = screen.getByText(/conditional/);
      expect(conditionalText.textContent).toContain("1");
    });

    it("handles graph with multiple edges", () => {
      const multiEdgeGraph: ResourceGraph = {
        rgdName: "multi-edge-rgd",
        rgdNamespace: "default",
        resources: [
          {
            id: "deployment-1",
            apiVersion: "apps/v1",
            kind: "Deployment",
            isTemplate: true,
            isConditional: false,
            dependsOn: [],
          },
          {
            id: "service-1",
            apiVersion: "v1",
            kind: "Service",
            isTemplate: true,
            isConditional: false,
            dependsOn: ["deployment-1"],
          },
          {
            id: "ingress-1",
            apiVersion: "networking.k8s.io/v1",
            kind: "Ingress",
            isTemplate: true,
            isConditional: false,
            dependsOn: ["service-1"],
          },
        ],
        edges: [
          {
            from: "service-1",
            to: "deployment-1",
            type: "reference",
          },
          {
            from: "ingress-1",
            to: "service-1",
            type: "reference",
          },
        ],
      };

      const { container } = render(
        <ResourceGraphView resourceGraph={multiEdgeGraph} />
      );
      const flowContainer = container.querySelector(".react-flow");
      expect(flowContainer).toBeInTheDocument();
    });
  });
});
