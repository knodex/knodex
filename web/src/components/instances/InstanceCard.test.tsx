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
  it("does not show RGD status warning for active parent", () => {
    renderCard(createTestInstance({ rgdStatus: "Active" }));
    expect(screen.queryByText("RGD Inactive")).not.toBeInTheDocument();
  });

  it("shows RGD Inactive warning when parent RGD is inactive", () => {
    renderCard(createTestInstance({ rgdStatus: "Inactive" }));
    expect(screen.getByText("RGD Inactive")).toBeInTheDocument();
  });

  it("does not show warning when rgdStatus is undefined", () => {
    renderCard(createTestInstance({ rgdStatus: undefined }));
    expect(screen.queryByText("RGD Inactive")).not.toBeInTheDocument();
  });

  it("does not show warning when rgdStatus is empty string", () => {
    renderCard(createTestInstance({ rgdStatus: "" }));
    expect(screen.queryByText("RGD Inactive")).not.toBeInTheDocument();
  });
});
