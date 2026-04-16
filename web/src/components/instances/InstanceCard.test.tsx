// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { InstanceCard } from "./InstanceCard";
import type { Instance } from "@/types/rgd";

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-instance",
    namespace: "default",
    rgdName: "my-rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "TestResource",
    health: "Healthy",
    conditions: [],
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    uid: "test-uid",
    ...overrides,
  };
}

function renderCard(instance: Instance) {
  return render(
    <TooltipProvider>
      <InstanceCard instance={instance} />
    </TooltipProvider>
  );
}

describe("InstanceCard", () => {
  it("shows Cluster-Scoped indicator for cluster-scoped instances", () => {
    renderCard(createTestInstance({ isClusterScoped: true, namespace: "" }));
    expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
  });

  it("shows namespace for namespace-scoped instances", () => {
    renderCard(createTestInstance({ isClusterScoped: false, namespace: "my-namespace" }));
    expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    expect(screen.getByText("my-namespace")).toBeInTheDocument();
  });

  it("shows namespace when isClusterScoped is undefined", () => {
    renderCard(createTestInstance({ namespace: "default" }));
    expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
  });
});
