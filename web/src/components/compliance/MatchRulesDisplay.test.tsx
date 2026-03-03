import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MatchRulesDisplay } from "./MatchRulesDisplay";
import type { ConstraintMatch } from "@/types/compliance";

describe("MatchRulesDisplay", () => {
  const fullMatch: ConstraintMatch = {
    kinds: [
      { apiGroups: [""], kinds: ["Pod", "ConfigMap"] },
      { apiGroups: ["apps"], kinds: ["Deployment", "StatefulSet"] },
    ],
    namespaces: ["default", "production", "staging"],
    scope: "Namespaced",
  };

  describe("full variant", () => {
    it("displays all resource kinds (AC-CON-08)", () => {
      render(<MatchRulesDisplay match={fullMatch} variant="full" />);

      expect(screen.getByText("Pod")).toBeInTheDocument();
      expect(screen.getByText("ConfigMap")).toBeInTheDocument();
      expect(screen.getByText("Deployment")).toBeInTheDocument();
      expect(screen.getByText("StatefulSet")).toBeInTheDocument();
    });

    it("displays namespaces", () => {
      render(<MatchRulesDisplay match={fullMatch} variant="full" />);

      expect(screen.getByText("default")).toBeInTheDocument();
      expect(screen.getByText("production")).toBeInTheDocument();
      expect(screen.getByText("staging")).toBeInTheDocument();
    });

    it("displays scope", () => {
      render(<MatchRulesDisplay match={fullMatch} variant="full" />);

      expect(screen.getByText("Namespaced")).toBeInTheDocument();
    });

    it("shows API groups for non-core resources", () => {
      render(<MatchRulesDisplay match={fullMatch} variant="full" />);

      // Should show apps group for Deployment/StatefulSet
      expect(screen.getByText("apps")).toBeInTheDocument();
    });
  });

  describe("compact variant", () => {
    it("shows summarized kind information", () => {
      const { container } = render(<MatchRulesDisplay match={fullMatch} variant="compact" />);

      // In compact mode, should show a summary of kinds
      expect(container.textContent).toContain("Pod");
      expect(container.textContent).toContain("ConfigMap");
    });

    it("shows summarized namespace information", () => {
      const { container } = render(<MatchRulesDisplay match={fullMatch} variant="compact" />);

      // Should show namespace info in compact form
      expect(container.textContent).toContain("default");
    });
  });

  describe("empty/missing data handling", () => {
    it("handles empty match object", () => {
      const emptyMatch: ConstraintMatch = {
        kinds: [],
        namespaces: [],
        scope: "*",
      };
      render(<MatchRulesDisplay match={emptyMatch} variant="full" />);

      // Should show "Matches all resources" when no specific rules
      expect(screen.getByText("Matches all resources")).toBeInTheDocument();
    });

    it("handles missing kinds", () => {
      const match: ConstraintMatch = {
        kinds: undefined as unknown as [],
        namespaces: ["default"],
        scope: "*",
      };
      render(<MatchRulesDisplay match={match} variant="full" />);

      // Should still show the namespaces
      expect(screen.getByText("default")).toBeInTheDocument();
    });

    it("handles missing namespaces", () => {
      const match: ConstraintMatch = {
        kinds: [{ apiGroups: [""], kinds: ["Pod"] }],
        namespaces: undefined as unknown as [],
        scope: "*",
      };
      render(<MatchRulesDisplay match={match} variant="full" />);

      // Should show the kinds
      expect(screen.getByText("Pod")).toBeInTheDocument();
    });

    it("handles empty kinds array", () => {
      const match: ConstraintMatch = {
        kinds: [],
        namespaces: ["default"],
        scope: "*",
      };
      render(<MatchRulesDisplay match={match} variant="full" />);

      // Should show the namespaces
      expect(screen.getByText("default")).toBeInTheDocument();
    });

    it("handles empty namespaces array", () => {
      const match: ConstraintMatch = {
        namespaces: [],
        kinds: [{ apiGroups: [""], kinds: ["Pod"] }],
        scope: "*",
      };
      render(<MatchRulesDisplay match={match} variant="full" />);

      // Should show the kinds
      expect(screen.getByText("Pod")).toBeInTheDocument();
    });
  });

  describe("cluster-scoped resources", () => {
    it("handles Cluster scope", () => {
      const clusterMatch: ConstraintMatch = {
        kinds: [{ apiGroups: [""], kinds: ["Namespace", "Node"] }],
        namespaces: [],
        scope: "Cluster",
      };

      render(<MatchRulesDisplay match={clusterMatch} variant="full" />);

      expect(screen.getByText("Cluster")).toBeInTheDocument();
    });

    it("handles wildcard scope with kinds", () => {
      const wildcardMatch: ConstraintMatch = {
        kinds: [{ apiGroups: ["*"], kinds: ["*"] }],
        namespaces: [],
        scope: "*",
      };

      const { container } = render(<MatchRulesDisplay match={wildcardMatch} variant="full" />);

      // Should show the wildcard kinds
      expect(container.textContent).toContain("*");
    });
  });

  describe("single items", () => {
    it("displays single kind without pluralization", () => {
      const singleKind: ConstraintMatch = {
        kinds: [{ apiGroups: [""], kinds: ["Pod"] }],
        namespaces: [],
        scope: "*",
      };

      render(<MatchRulesDisplay match={singleKind} variant="full" />);

      expect(screen.getByText("Pod")).toBeInTheDocument();
    });

    it("displays single namespace", () => {
      const singleNs: ConstraintMatch = {
        kinds: [{ apiGroups: [""], kinds: ["Pod"] }],
        namespaces: ["default"],
        scope: "*",
      };

      render(<MatchRulesDisplay match={singleNs} variant="full" />);

      expect(screen.getByText("default")).toBeInTheDocument();
    });
  });

  it("applies custom className", () => {
    const { container } = render(
      <MatchRulesDisplay match={fullMatch} className="custom-class" />
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("defaults to full variant", () => {
    render(<MatchRulesDisplay match={fullMatch} />);

    // Should show section headers by default
    expect(screen.getByText("Namespaces")).toBeInTheDocument();
    expect(screen.getByText("Resource Kinds")).toBeInTheDocument();
  });
});
