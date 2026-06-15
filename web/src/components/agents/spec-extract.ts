// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import yaml from "js-yaml";

/**
 * Pure helpers for extracting and parsing the YAML spec block from an
 * agent's response text (Story 50.1). Kept out of the component files so
 * react-refresh/only-export-components stays happy and the logic is
 * unit-testable in isolation.
 */

/**
 * Extract the first fenced ```yaml code block from `text`; falls back to the
 * first bare ``` block. Returns null when no fenced block exists — which IS
 * the AC #3 no-match path (the agent explained instead of generating).
 */
export function extractYamlBlock(text: string): string | null {
  if (!text) return null;
  const yamlFence = /```(?:yaml|yml)[ \t]*\r?\n([\s\S]*?)```/;
  const yamlMatch = yamlFence.exec(text);
  if (yamlMatch) {
    const block = yamlMatch[1].trim();
    return block.length > 0 ? block : null;
  }
  const bareFence = /```[ \t]*\r?\n([\s\S]*?)```/;
  const bareMatch = bareFence.exec(text);
  if (bareMatch) {
    const block = bareMatch[1].trim();
    return block.length > 0 ? block : null;
  }
  return null;
}

/**
 * Remove every fenced code block from `text`, leaving the agent's prose —
 * rendered as the explanation around the spec preview.
 */
export function stripCodeBlocks(text: string): string {
  if (!text) return "";
  return text.replace(/```[a-zA-Z]*[ \t]*\r?\n[\s\S]*?```/g, "").trim();
}

/**
 * Parse a YAML block into an object for the structured preview. Returns
 * null when the block is not valid YAML or not a mapping — the caller then
 * renders the raw-YAML-only preview with a parse notice (copy/download stay
 * available; the text block exists even if unparseable).
 */
export function parseSpec(block: string): Record<string, unknown> | null {
  try {
    const parsed = yaml.load(block);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return null;
  } catch {
    return null;
  }
}

/**
 * The traceability annotation linking a generated resource template to the
 * user requirement that produced it (Story 50.2). The server backfills it
 * deterministically after the A2A response — the web only ever reads it.
 */
export const GENERATED_FROM_ANNOTATION = "knodex.io/generated-from";

/** A resource row for the structured RGD preview table. */
export interface SpecResourceRow {
  id: string;
  kind: string;
  apiVersion: string;
  /** Value of the knodex.io/generated-from annotation ("" when absent). */
  generatedFrom: string;
}

/** Structured fields extracted from a parsed RGD spec. */
export interface RGDSpecSummary {
  /** metadata.name of the ResourceGraphDefinition. */
  name: string;
  /** spec.schema.kind — the produced instance kind. */
  schemaKind: string;
  /** spec.schema.apiVersion. */
  schemaApiVersion: string;
  /** spec.resources rows (id + template kind/apiVersion). */
  resources: SpecResourceRow[];
}

/**
 * True when the parsed object is a KRO ResourceGraphDefinition — the
 * structured-preview branch. Anything else falls back to the generic
 * key/value rendering.
 */
export function isRGDSpec(spec: Record<string, unknown>): boolean {
  return spec.kind === "ResourceGraphDefinition";
}

/**
 * Nil-safe extraction of the structured RGD summary. Agent output is
 * untrusted — every access is optional-chained and defaulted.
 */
export function summarizeRGDSpec(spec: Record<string, unknown>): RGDSpecSummary {
  const metadata = asRecord(spec.metadata);
  const specSection = asRecord(spec.spec);
  const schema = asRecord(specSection?.schema);

  const resources: SpecResourceRow[] = [];
  const rawResources = specSection?.resources;
  if (Array.isArray(rawResources)) {
    for (const raw of rawResources) {
      const resource = asRecord(raw);
      if (!resource) continue;
      const template = asRecord(resource.template);
      const annotations = asRecord(asRecord(template?.metadata)?.annotations);
      resources.push({
        id: asString(resource.id),
        kind: asString(template?.kind),
        apiVersion: asString(template?.apiVersion),
        generatedFrom: asString(annotations?.[GENERATED_FROM_ANNOTATION]),
      });
    }
  }

  return {
    name: asString(metadata?.name),
    schemaKind: asString(schema?.kind),
    schemaApiVersion: asString(schema?.apiVersion),
    resources,
  };
}

/** One requirement → resources group for the traceability view (AC #2). */
export interface TraceabilityGroup {
  /** The requirement text ("" = no annotation — defensive-only bucket). */
  requirement: string;
  resources: { id: string; kind: string }[];
}

/**
 * Group the summary's resources by their generated-from requirement,
 * preserving first-seen order. Resources without the annotation collect
 * under requirement "" (rendered as "No requirement recorded" — defensive
 * only; the server backfill makes it unreachable for real responses).
 */
export function groupTraceability(summary: RGDSpecSummary): TraceabilityGroup[] {
  const groups: TraceabilityGroup[] = [];
  const byRequirement = new Map<string, TraceabilityGroup>();
  for (const resource of summary.resources) {
    let group = byRequirement.get(resource.generatedFrom);
    if (!group) {
      group = { requirement: resource.generatedFrom, resources: [] };
      byRequirement.set(resource.generatedFrom, group);
      groups.push(group);
    }
    group.resources.push({ id: resource.id, kind: resource.kind });
  }
  return groups;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return null;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}
