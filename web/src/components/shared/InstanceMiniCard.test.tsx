// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { InstanceMiniCard } from "./InstanceMiniCard";
import type { Instance } from "@/types/rgd";

vi.mock("@/components/instances/HealthBadge", () => ({
  HealthBadge: ({ health }: { health: string }) => (
    <span data-testid="health-badge">{health}</span>
  ),
}));

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-instance",
    namespace: "production",
    rgdName: "test-rgd",
    rgdNamespace: "default",
    apiVersion: "kro.run/v1alpha1",
    kind: "WebApp",
    health: "Healthy",
    conditions: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderCard(props: Parameters<typeof InstanceMiniCard>[0]) {
  return render(
    <MemoryRouter>
      <InstanceMiniCard {...props} />
    </MemoryRouter>
  );
}

describe("InstanceMiniCard", () => {
  describe("loading state", () => {
    it("shows spinner and resolving text", () => {
      renderCard({
        isLoading: true,
        label: "my-db",
        action: null,
      });
      expect(screen.getByText("Resolving my-db...")).toBeInTheDocument();
    });
  });

  describe("not-found state", () => {
    it("shows alert and 'External resource' message", () => {
      renderCard({
        isLoading: false,
        notFound: true,
        label: "missing-service",
        action: null,
      });
      expect(screen.getByText("missing-service")).toBeInTheDocument();
      expect(screen.getByText("External resource")).toBeInTheDocument();
    });

    it("shows kind and namespace badges when provided", () => {
      renderCard({
        isLoading: false,
        notFound: true,
        label: "my-db",
        kindLabel: "PostgreSQL",
        namespaceLabel: "prod-ns",
        action: null,
      });
      expect(screen.getByText("PostgreSQL")).toBeInTheDocument();
      expect(screen.getByText("prod-ns")).toBeInTheDocument();
    });

    it("does not show kind/namespace badges when not provided", () => {
      renderCard({
        isLoading: false,
        notFound: true,
        label: "my-db",
        action: null,
      });
      // Only the label and "External resource" should show
      expect(screen.getByText("my-db")).toBeInTheDocument();
      expect(screen.getByText("External resource")).toBeInTheDocument();
    });
  });

  describe("loaded state", () => {
    it("renders instance name", () => {
      renderCard({
        instance: createTestInstance(),
        isLoading: false,
        action: <span>View</span>,
      });
      expect(screen.getByText("my-instance")).toBeInTheDocument();
    });

    it("renders health badge", () => {
      renderCard({
        instance: createTestInstance({ health: "Degraded" }),
        isLoading: false,
        action: null,
      });
      expect(screen.getByTestId("health-badge")).toHaveTextContent("Degraded");
    });

    it("renders kind and namespace badges", () => {
      renderCard({
        instance: createTestInstance({ kind: "AKSCluster", namespace: "infra" }),
        isLoading: false,
        action: null,
      });
      expect(screen.getByText("AKSCluster")).toBeInTheDocument();
      expect(screen.getByText("infra")).toBeInTheDocument();
    });

    it("renders the action slot", () => {
      renderCard({
        instance: createTestInstance(),
        isLoading: false,
        action: <button>Go to instance</button>,
      });
      expect(screen.getByText("Go to instance")).toBeInTheDocument();
    });
  });
});
