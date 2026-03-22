// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { CatalogSecretsTab } from "./CatalogSecretsTab";
import type { SecretRef } from "@/types/secret";

const fixedRef: SecretRef = {
  type: "fixed",
  id: "0-Secret",
  externalRefId: "dbSecret",
  name: "my-db-secret",
  namespace: "production",
  description: "Name of the Kubernetes Secret containing database credentials",
};

const dynamicRef: SecretRef = {
  type: "dynamic",
  id: "1-Secret",
  externalRefId: "apiCert",
  nameExpr: "${schema.spec.externalRef.apiCert.name}",
  namespaceExpr: "${schema.spec.externalRef.apiCert.namespace}",
};

const refWithoutDescription: SecretRef = {
  type: "fixed",
  id: "2-Secret",
  externalRefId: "simpleSecret",
  name: "plain-secret",
  namespace: "default",
};

describe("CatalogSecretsTab", () => {
  it("renders null when secretRefs is empty", () => {
    const { container } = render(<CatalogSecretsTab secretRefs={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders section header for non-empty secretRefs", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef]} />);
    expect(screen.getByText("Required Secrets")).toBeInTheDocument();
  });

  it("shows externalRefId as card title (not raw ID)", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef]} />);
    expect(screen.getByText("dbSecret")).toBeInTheDocument();
    // Should NOT show "Secret" (the raw kind) as the card title
    expect(screen.queryByText("Secret")).not.toBeInTheDocument();
  });

  it("falls back to stripped ID when externalRefId is absent", () => {
    const ref: SecretRef = { ...fixedRef, externalRefId: undefined };
    render(<CatalogSecretsTab secretRefs={[ref]} />);
    // "0-Secret" strips to "Secret"
    expect(screen.getByText("Secret")).toBeInTheDocument();
  });

  it("renders description when present", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef]} />);
    expect(screen.getByText("Name of the Kubernetes Secret containing database credentials")).toBeInTheDocument();
  });

  it("does not render description element when absent", () => {
    render(<CatalogSecretsTab secretRefs={[refWithoutDescription]} />);
    // Should not render a per-card description paragraph (only the fixed section subtitle is present)
    const descParagraphs = document.querySelectorAll("p.text-sm.text-muted-foreground");
    expect(descParagraphs).toHaveLength(0);
  });

  it("renders type badge", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef]} />);
    expect(screen.getByText("fixed")).toBeInTheDocument();
  });

  describe("fixed secret ref", () => {
    it("renders literal name and namespace", () => {
      render(<CatalogSecretsTab secretRefs={[fixedRef]} />);
      expect(screen.getByText("my-db-secret")).toBeInTheDocument();
      expect(screen.getByText("production")).toBeInTheDocument();
    });

    it("shows em-dash when name is absent", () => {
      const ref: SecretRef = { ...fixedRef, name: undefined };
      render(<CatalogSecretsTab secretRefs={[ref]} />);
      const dashes = screen.getAllByText("—");
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });

    it("shows em-dash when namespace is absent", () => {
      const ref: SecretRef = { ...fixedRef, namespace: undefined };
      render(<CatalogSecretsTab secretRefs={[ref]} />);
      const dashes = screen.getAllByText("—");
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });
  });

  describe("dynamic secret ref", () => {
    it("renders CEL expression for name", () => {
      render(<CatalogSecretsTab secretRefs={[dynamicRef]} />);
      expect(screen.getByText("${schema.spec.externalRef.apiCert.name}")).toBeInTheDocument();
    });

    it("renders CEL expression for namespace", () => {
      render(<CatalogSecretsTab secretRefs={[dynamicRef]} />);
      expect(screen.getByText("${schema.spec.externalRef.apiCert.namespace}")).toBeInTheDocument();
    });

    it("renders dynamic type badge", () => {
      render(<CatalogSecretsTab secretRefs={[dynamicRef]} />);
      expect(screen.getByText("dynamic")).toBeInTheDocument();
    });
  });

  describe("provided secret ref", () => {
    const providedRef: SecretRef = {
      type: "provided",
      id: "3-Secret",
      externalRefId: "tlsCert",
      description: "TLS certificate provided by the user at deploy time",
    };

    it("renders user-provided type badge", () => {
      render(<CatalogSecretsTab secretRefs={[providedRef]} />);
      expect(screen.getByText("user-provided")).toBeInTheDocument();
    });

    it("does not render name/namespace details", () => {
      render(<CatalogSecretsTab secretRefs={[providedRef]} />);
      expect(screen.queryByText("Name")).not.toBeInTheDocument();
      expect(screen.queryByText("Namespace")).not.toBeInTheDocument();
    });
  });

  it("renders a card per secret ref", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef, dynamicRef]} />);
    expect(screen.getByTestId("catalog-secret-ref-0-Secret")).toBeInTheDocument();
    expect(screen.getByTestId("catalog-secret-ref-1-Secret")).toBeInTheDocument();
  });

  it("renders multiple secrets with distinct display names", () => {
    render(<CatalogSecretsTab secretRefs={[fixedRef, dynamicRef]} />);
    expect(screen.getByText("dbSecret")).toBeInTheDocument();
    expect(screen.getByText("apiCert")).toBeInTheDocument();
  });
});
