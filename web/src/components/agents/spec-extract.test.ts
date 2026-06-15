// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import {
  extractYamlBlock,
  stripCodeBlocks,
  parseSpec,
  isRGDSpec,
  summarizeRGDSpec,
  groupTraceability,
  GENERATED_FROM_ANNOTATION,
} from "./spec-extract";

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

describe("extractYamlBlock", () => {
  it("extracts the first fenced yaml block", () => {
    const text = `Here is your spec:\n\n\`\`\`yaml\n${RGD_YAML}\n\`\`\`\n\nDeploy it via the Catalog.`;
    expect(extractYamlBlock(text)).toBe(RGD_YAML);
  });

  it("supports the yml fence alias", () => {
    const text = "```yml\nkind: Thing\n```";
    expect(extractYamlBlock(text)).toBe("kind: Thing");
  });

  it("falls back to the first bare fenced block", () => {
    const text = "Some prose\n```\nkind: Thing\nname: x\n```\nmore prose";
    expect(extractYamlBlock(text)).toBe("kind: Thing\nname: x");
  });

  it("prefers the yaml fence over an earlier bare fence", () => {
    const text = "```\nnot the spec\n```\n\n```yaml\nkind: Spec\n```";
    expect(extractYamlBlock(text)).toBe("kind: Spec");
  });

  it("returns null when no fenced block exists (the AC #3 no-match path)", () => {
    expect(
      extractYamlBlock("No matching CRDs found for: Redis — available options: Deployment, Service")
    ).toBeNull();
  });

  it("returns null for empty input and empty blocks", () => {
    expect(extractYamlBlock("")).toBeNull();
    expect(extractYamlBlock("```yaml\n```")).toBeNull();
  });

  it("takes only the FIRST yaml block when several exist", () => {
    const text = "```yaml\nkind: First\n```\n```yaml\nkind: Second\n```";
    expect(extractYamlBlock(text)).toBe("kind: First");
  });
});

describe("stripCodeBlocks", () => {
  it("removes fenced blocks and keeps the prose", () => {
    const text = `Intro prose.\n\n\`\`\`yaml\nkind: Spec\n\`\`\`\n\nOutro prose.`;
    expect(stripCodeBlocks(text)).toBe("Intro prose.\n\n\n\nOutro prose.".trim());
  });

  it("returns the full text when there are no blocks", () => {
    expect(stripCodeBlocks("just an explanation")).toBe("just an explanation");
  });

  it("handles empty input", () => {
    expect(stripCodeBlocks("")).toBe("");
  });
});

describe("parseSpec", () => {
  it("parses a valid YAML mapping", () => {
    const spec = parseSpec(RGD_YAML);
    expect(spec).not.toBeNull();
    expect(spec?.kind).toBe("ResourceGraphDefinition");
  });

  it("returns null for invalid YAML (graceful raw-text fallback)", () => {
    expect(parseSpec("kind: [unclosed")).toBeNull();
  });

  it("returns null for non-mapping YAML (scalar, list)", () => {
    expect(parseSpec("just a string")).toBeNull();
    expect(parseSpec("- a\n- b")).toBeNull();
  });
});

describe("isRGDSpec / summarizeRGDSpec", () => {
  it("recognizes a ResourceGraphDefinition", () => {
    const spec = parseSpec(RGD_YAML)!;
    expect(isRGDSpec(spec)).toBe(true);
  });

  it("rejects other kinds", () => {
    expect(isRGDSpec({ kind: "Deployment" })).toBe(false);
    expect(isRGDSpec({})).toBe(false);
  });

  it("summarizes name, schema and resources", () => {
    const summary = summarizeRGDSpec(parseSpec(RGD_YAML)!);
    expect(summary.name).toBe("webapp-stack");
    expect(summary.schemaKind).toBe("WebAppStack");
    expect(summary.schemaApiVersion).toBe("v1alpha1");
    expect(summary.resources).toEqual([
      { id: "deployment", kind: "Deployment", apiVersion: "apps/v1", generatedFrom: "" },
      { id: "redis", kind: "RedisCluster", apiVersion: "redis.example.io/v1", generatedFrom: "" },
    ]);
  });

  it("is nil-safe on malformed agent output (untrusted)", () => {
    const summary = summarizeRGDSpec({
      kind: "ResourceGraphDefinition",
      metadata: "not-an-object",
      spec: { resources: ["scalar", { id: 42, template: null }] },
    });
    expect(summary.name).toBe("");
    expect(summary.schemaKind).toBe("");
    expect(summary.resources).toEqual([
      { id: "", kind: "", apiVersion: "", generatedFrom: "" },
    ]);
  });

  it("extracts the generated-from annotation when present", () => {
    const spec = {
      kind: "ResourceGraphDefinition",
      spec: {
        resources: [
          {
            id: "deployment",
            template: {
              kind: "Deployment",
              metadata: { annotations: { [GENERATED_FROM_ANNOTATION]: "a web app" } },
            },
          },
          { id: "bare", template: { kind: "Service" } },
          {
            id: "weird",
            template: {
              kind: "ConfigMap",
              metadata: { annotations: { [GENERATED_FROM_ANNOTATION]: 42 } },
            },
          },
        ],
      },
    };
    const summary = summarizeRGDSpec(spec);
    expect(summary.resources[0].generatedFrom).toBe("a web app");
    // absent annotation → empty string; non-string annotation → empty string
    expect(summary.resources[1].generatedFrom).toBe("");
    expect(summary.resources[2].generatedFrom).toBe("");
  });
});

describe("groupTraceability", () => {
  const row = (id: string, kind: string, generatedFrom: string) => ({
    id,
    kind,
    apiVersion: "v1",
    generatedFrom,
  });

  it("groups multiple resources under one requirement", () => {
    const groups = groupTraceability({
      name: "x",
      schemaKind: "X",
      schemaApiVersion: "v1alpha1",
      resources: [
        row("deployment", "Deployment", "a web app"),
        row("service", "Service", "a web app"),
        row("redis", "RedisCluster", "with redis"),
      ],
    });
    expect(groups).toEqual([
      {
        requirement: "a web app",
        resources: [
          { id: "deployment", kind: "Deployment" },
          { id: "service", kind: "Service" },
        ],
      },
      { requirement: "with redis", resources: [{ id: "redis", kind: "RedisCluster" }] },
    ]);
  });

  it("preserves first-seen order even when requirements interleave", () => {
    const groups = groupTraceability({
      name: "x",
      schemaKind: "X",
      schemaApiVersion: "v1alpha1",
      resources: [
        row("a", "A", "second seen last"),
        row("b", "B", "first interleaved"),
        row("c", "C", "second seen last"),
      ],
    });
    expect(groups.map((g) => g.requirement)).toEqual([
      "second seen last",
      "first interleaved",
    ]);
    expect(groups[0].resources.map((r) => r.id)).toEqual(["a", "c"]);
  });

  it("collects resources without an annotation under the empty-string bucket", () => {
    const groups = groupTraceability({
      name: "x",
      schemaKind: "X",
      schemaApiVersion: "v1alpha1",
      resources: [row("a", "A", "req"), row("b", "B", "")],
    });
    expect(groups).toHaveLength(2);
    expect(groups[1].requirement).toBe("");
    expect(groups[1].resources).toEqual([{ id: "b", kind: "B" }]);
  });

  it("returns an empty array for a spec with no resources", () => {
    const groups = groupTraceability({
      name: "x",
      schemaKind: "X",
      schemaApiVersion: "v1alpha1",
      resources: [],
    });
    expect(groups).toEqual([]);
  });
});
