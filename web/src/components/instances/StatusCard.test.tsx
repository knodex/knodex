// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { StatusCard } from "./StatusCard";
import { StatusCardSkeleton } from "./StatusCardSkeleton";
import type { Instance } from "@/types/rgd";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-instance",
    namespace: "default",
    rgdName: "my-rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "AKSCluster",
    health: "Healthy",
    conditions: [],
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    uid: "test-uid",
    labels: { "knodex.io/project": "alpha" },
    ...overrides,
  };
}

function renderStatusCard(instance: Instance, onClick?: (i: Instance) => void) {
  return render(
    <MemoryRouter>
      <StatusCard instance={instance} onClick={onClick} />
    </MemoryRouter>
  );
}

beforeEach(() => {
  mockNavigate.mockClear();
});

describe("StatusCard", () => {
  describe("health state rendering (AC #1)", () => {
    it.each([
      ["Healthy", "Status: healthy"],
      ["Degraded", "Status: warning"],
      ["Unhealthy", "Status: error"],
      ["Progressing", "Status: progressing"],
      ["Unknown", "Status: unknown"],
    ] as const)("renders %s health as correct StatusIndicator status", (health, expectedLabel) => {
      renderStatusCard(createTestInstance({ health }));
      expect(screen.getByRole("status")).toHaveAttribute("aria-label", expectedLabel);
    });
  });

  describe("instance metadata display (AC #1)", () => {
    it("displays instance name", () => {
      renderStatusCard(createTestInstance({ name: "web-app-prod" }));
      expect(screen.getByText("web-app-prod")).toBeInTheDocument();
    });

    it("displays kind badge in monospace", () => {
      renderStatusCard(createTestInstance({ kind: "AKSCluster" }));
      const badge = screen.getByText("AKSCluster");
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveClass("font-mono");
    });

    it("displays namespace for namespaced instances", () => {
      renderStatusCard(createTestInstance({ namespace: "production" }));
      expect(screen.getByText("production")).toBeInTheDocument();
    });

    it("displays cluster label for cluster-scoped instances", () => {
      renderStatusCard(
        createTestInstance({ isClusterScoped: true, namespace: "" })
      );
      expect(screen.getByText("cluster")).toBeInTheDocument();
    });

    it("displays relative age", () => {
      renderStatusCard(createTestInstance());
      // createdAt is "now" so should show "just now"
      expect(screen.getByText("just now")).toBeInTheDocument();
    });

    it("displays service URL when present in annotations", () => {
      renderStatusCard(
        createTestInstance({
          annotations: { "knodex.io/url": "https://app.example.com" },
        })
      );
      const link = screen.getByRole("link");
      expect(link).toHaveAttribute("href", "https://app.example.com");
      expect(link).toHaveAttribute("target", "_blank");
      expect(screen.getByText("app.example.com")).toBeInTheDocument();
    });

    it("omits URL section when no URL is available", () => {
      renderStatusCard(createTestInstance());
      expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });

    it("renders malformed URL without crashing", () => {
      renderStatusCard(
        createTestInstance({
          annotations: { "knodex.io/url": "http://" },
        })
      );
      const link = screen.getByRole("link");
      expect(link).toHaveAttribute("href", "http://");
      // safeHostname falls back to raw string for malformed URLs
      expect(link).toBeInTheDocument();
    });
  });

  describe("left-border accent (AC #4)", () => {
    it("applies rose left-border for Unhealthy instances", () => {
      renderStatusCard(createTestInstance({ health: "Unhealthy" }));
      const card = screen.getByTestId("status-card");
      expect(card.style.borderLeftColor).toBe("var(--status-error)");
      expect(card).toHaveClass("border-l-2");
    });

    it("applies amber left-border for Degraded instances", () => {
      renderStatusCard(createTestInstance({ health: "Degraded" }));
      const card = screen.getByTestId("status-card");
      expect(card.style.borderLeftColor).toBe("var(--status-warning)");
      expect(card).toHaveClass("border-l-2");
    });

    it("does not apply left-border accent for Healthy instances", () => {
      renderStatusCard(createTestInstance({ health: "Healthy" }));
      const card = screen.getByTestId("status-card");
      expect(card.style.borderLeftColor).toBe("");
      expect(card).not.toHaveClass("border-l-2");
    });

    it("does not apply left-border accent for Progressing instances", () => {
      renderStatusCard(createTestInstance({ health: "Progressing" }));
      const card = screen.getByTestId("status-card");
      expect(card.style.borderLeftColor).toBe("");
      expect(card).not.toHaveClass("border-l-2");
    });
  });

  describe("click navigation (AC #5)", () => {
    it("navigates to detail page for namespaced instance", () => {
      renderStatusCard(
        createTestInstance({ namespace: "prod", kind: "AKSCluster", name: "cluster-1" })
      );
      fireEvent.click(screen.getByTestId("status-card"));
      expect(mockNavigate).toHaveBeenCalledWith(
        "/instances/prod/AKSCluster/cluster-1"
      );
    });

    it("navigates to detail page for cluster-scoped instance", () => {
      renderStatusCard(
        createTestInstance({
          isClusterScoped: true,
          namespace: "",
          kind: "GlobalPolicy",
          name: "my-policy",
        })
      );
      fireEvent.click(screen.getByTestId("status-card"));
      // cluster-scoped: namespace is empty string, still included in URL
      expect(mockNavigate).toHaveBeenCalledWith(
        "/instances//GlobalPolicy/my-policy"
      );
    });

    it("calls onClick prop when provided instead of navigating", () => {
      const handleClick = vi.fn();
      const instance = createTestInstance();
      renderStatusCard(instance, handleClick);
      fireEvent.click(screen.getByTestId("status-card"));
      expect(handleClick).toHaveBeenCalledWith(instance);
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it("navigates on Enter key press", () => {
      renderStatusCard(createTestInstance({ namespace: "ns", kind: "K", name: "n" }));
      fireEvent.keyDown(screen.getByTestId("status-card"), { key: "Enter" });
      expect(mockNavigate).toHaveBeenCalledWith("/instances/ns/K/n");
    });

    it("navigates on Space key press", () => {
      renderStatusCard(createTestInstance({ namespace: "ns", kind: "K", name: "n" }));
      fireEvent.keyDown(screen.getByTestId("status-card"), { key: " " });
      expect(mockNavigate).toHaveBeenCalledWith("/instances/ns/K/n");
    });
  });

  describe("card styling (AC #2, #3)", () => {
    it("has correct base surface classes", () => {
      renderStatusCard(createTestInstance());
      const card = screen.getByTestId("status-card");
      expect(card).toHaveClass("bg-[var(--surface-primary)]");
      expect(card).toHaveClass("rounded-[var(--radius-token-lg)]");
      expect(card).toHaveClass("border-[var(--border-default)]");
    });

    it("has hover transition classes", () => {
      renderStatusCard(createTestInstance());
      const card = screen.getByTestId("status-card");
      expect(card).toHaveClass("hover:shadow-[var(--shadow-card-hover)]");
      expect(card).toHaveClass("hover:translate-y-[-1px]");
      expect(card).toHaveClass("duration-200");
    });

    it("has accessible attributes", () => {
      renderStatusCard(createTestInstance({ name: "test-app" }));
      const card = screen.getByTestId("status-card");
      expect(card).toHaveAttribute("role", "button");
      expect(card).toHaveAttribute("tabindex", "0");
      expect(card).toHaveAttribute("aria-label", "View details for test-app");
    });
  });
});

describe("StatusCardSkeleton (AC #6)", () => {
  it("renders skeleton structure", () => {
    render(<StatusCardSkeleton />);
    expect(screen.getByTestId("status-card-skeleton")).toBeInTheDocument();
  });

  it("has correct surface classes matching StatusCard", () => {
    render(<StatusCardSkeleton />);
    const skeleton = screen.getByTestId("status-card-skeleton");
    expect(skeleton).toHaveClass("bg-[var(--surface-primary)]");
    expect(skeleton).toHaveClass("rounded-[var(--radius-token-lg)]");
    expect(skeleton).toHaveClass("border-[var(--border-default)]");
  });
});
