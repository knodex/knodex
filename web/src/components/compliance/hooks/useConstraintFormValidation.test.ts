// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { renderHook } from "@testing-library/react";
import { useConstraintFormValidation, baseConstraintFormSchema } from "./useConstraintFormValidation";
import type { ConstraintTemplate } from "@/types/compliance";

describe("useConstraintFormValidation", () => {
  const template: ConstraintTemplate = {
    name: "k8srequiredlabels",
    kind: "K8sRequiredLabels",
    description: "Requires specified labels on resources",
    rego: "",
    parameters: {
      properties: {
        labels: {
          type: "array",
          items: { type: "string" },
          default: ["team"],
        },
      },
    },
    labels: {},
    createdAt: "2024-01-01T00:00:00Z",
  };

  it("returns parsedSchema for template with parameters", () => {
    const { result } = renderHook(() => useConstraintFormValidation(template));
    expect(result.current.parsedSchema).toBeTruthy();
    expect(result.current.parsedSchema?.properties).toHaveProperty("labels");
  });

  it("returns canUseFormMode true for renderable schema", () => {
    const { result } = renderHook(() => useConstraintFormValidation(template));
    expect(result.current.canUseFormMode).toBe(true);
  });

  it("returns formSchema extending base schema", () => {
    const { result } = renderHook(() => useConstraintFormValidation(template));
    expect(result.current.formSchema).toBeDefined();
  });

  it("returns defaultValues with enforcement deny", () => {
    const { result } = renderHook(() => useConstraintFormValidation(template));
    expect(result.current.defaultValues.enforcementAction).toBe("deny");
    expect(result.current.defaultValues.name).toBe("");
    expect(result.current.defaultValues.matchKinds).toEqual([{ apiGroups: [], kinds: [] }]);
  });

  it("returns defaultValues with parameter defaults from schema", () => {
    const { result } = renderHook(() => useConstraintFormValidation(template));
    expect(result.current.defaultValues.params).toBeDefined();
  });

  it("handles template without parameters", () => {
    const noParamsTemplate: ConstraintTemplate = {
      ...template,
      parameters: undefined,
    };
    const { result } = renderHook(() => useConstraintFormValidation(noParamsTemplate));
    expect(result.current.parsedSchema).toBeNull();
    expect(result.current.canUseFormMode).toBe(false);
    expect(result.current.defaultValues.parametersRaw).toBe("{}");
  });
});

describe("baseConstraintFormSchema", () => {
  it("validates valid constraint name", () => {
    const result = baseConstraintFormSchema.safeParse({
      name: "my-constraint",
      enforcementAction: "deny",
    });
    expect(result.success).toBe(true);
  });

  it("rejects empty name", () => {
    const result = baseConstraintFormSchema.safeParse({
      name: "",
      enforcementAction: "deny",
    });
    expect(result.success).toBe(false);
  });

  it("rejects name starting with hyphen", () => {
    const result = baseConstraintFormSchema.safeParse({
      name: "-invalid",
      enforcementAction: "deny",
    });
    expect(result.success).toBe(false);
  });

  it("rejects name exceeding 253 chars", () => {
    const result = baseConstraintFormSchema.safeParse({
      name: "a".repeat(254),
      enforcementAction: "deny",
    });
    expect(result.success).toBe(false);
  });
});
