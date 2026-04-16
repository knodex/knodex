// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ScopeIndicator } from "./ScopeIndicator";
import { hasValidNamespace } from "@/types/rgd";

function renderIndicator(props: Parameters<typeof ScopeIndicator>[0]) {
  return render(
    <TooltipProvider>
      <ScopeIndicator {...props} />
    </TooltipProvider>
  );
}

describe("ScopeIndicator", () => {
  describe("cluster-scoped (badge variant)", () => {
    it("renders Cluster-Scoped text with Globe icon", () => {
      renderIndicator({ isClusterScoped: true });
      expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
    });

    it("has aria-label on Globe icon for screen readers", () => {
      renderIndicator({ isClusterScoped: true, variant: "badge" });
      expect(screen.getByLabelText("Cluster-scoped resource")).toBeInTheDocument();
    });
  });

  describe("cluster-scoped (compact variant)", () => {
    it("renders Globe icon without text", () => {
      renderIndicator({ isClusterScoped: true, variant: "compact" });
      expect(screen.getByLabelText("Cluster-scoped resource")).toBeInTheDocument();
      expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    });
  });

  describe("namespaced (compact variant)", () => {
    it("renders namespace text without Globe icon", () => {
      renderIndicator({ isClusterScoped: false, namespace: "my-ns", variant: "compact" });
      expect(screen.getByText("my-ns")).toBeInTheDocument();
      expect(screen.queryByRole("img")).not.toBeInTheDocument();
      expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    });
  });

  describe("cluster-scoped (inline variant)", () => {
    it("renders Cluster-Scoped text", () => {
      renderIndicator({ isClusterScoped: true, variant: "inline" });
      expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
      expect(screen.getByLabelText("Cluster-scoped resource")).toBeInTheDocument();
    });
  });

  describe("cluster-scoped (text variant)", () => {
    it("renders Cluster-Scoped text without Globe icon", () => {
      renderIndicator({ isClusterScoped: true, variant: "text" });
      expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
      expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });
  });

  describe("namespaced (badge variant)", () => {
    it("renders namespace text", () => {
      renderIndicator({ isClusterScoped: false, namespace: "my-namespace" });
      expect(screen.getByText("my-namespace")).toBeInTheDocument();
      expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    });

    it("renders dash when namespace is empty", () => {
      renderIndicator({ isClusterScoped: false, namespace: "" });
      expect(screen.getByText("—")).toBeInTheDocument();
    });
  });

  describe("namespaced (inline variant)", () => {
    it("renders namespace text", () => {
      renderIndicator({ isClusterScoped: false, namespace: "production", variant: "inline" });
      expect(screen.getByText("production")).toBeInTheDocument();
    });

    it("renders dash when namespace is empty", () => {
      renderIndicator({ isClusterScoped: false, namespace: "", variant: "inline" });
      expect(screen.getByText("—")).toBeInTheDocument();
    });

    it("renders dash when namespace is undefined", () => {
      renderIndicator({ isClusterScoped: false, variant: "inline" });
      expect(screen.getByText("—")).toBeInTheDocument();
    });
  });

  describe("namespaced (text variant)", () => {
    it("renders Namespaced text", () => {
      renderIndicator({ isClusterScoped: false, variant: "text" });
      expect(screen.getByText("Namespaced")).toBeInTheDocument();
    });
  });

  describe("defaults", () => {
    it("treats undefined isClusterScoped as namespaced", () => {
      renderIndicator({ namespace: "default" });
      expect(screen.getByText("default")).toBeInTheDocument();
      expect(screen.queryByText("Cluster-Scoped")).not.toBeInTheDocument();
    });

    it("uses badge variant by default", () => {
      renderIndicator({ isClusterScoped: true });
      expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
    });
  });
});

describe("hasValidNamespace", () => {
  it("returns true for cluster-scoped with empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: true, namespace: "" })).toBe(true);
  });

  it("returns false for cluster-scoped with non-empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: true, namespace: "oops" })).toBe(false);
  });

  it("returns true for namespaced with non-empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: false, namespace: "default" })).toBe(true);
  });

  it("returns false for namespaced with empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: false, namespace: "" })).toBe(false);
  });

  it("returns true for undefined isClusterScoped with non-empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: undefined, namespace: "prod" })).toBe(true);
  });

  it("returns false for undefined isClusterScoped with empty namespace", () => {
    expect(hasValidNamespace({ isClusterScoped: undefined, namespace: "" })).toBe(false);
  });
});
