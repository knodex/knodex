// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useConstraintSubmission } from "./useConstraintSubmission";
import type { ConstraintTemplate } from "@/types/compliance";
import type { ConstraintFormValues } from "./useConstraintFormValidation";

const mockMutateAsync = vi.fn();
const mockMutationReset = vi.fn();
vi.mock("@/hooks/useCompliance", () => ({
  useCreateConstraint: () => ({
    mutateAsync: mockMutateAsync,
    reset: mockMutationReset,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/api/compliance", () => ({
  isAlreadyExists: (err: unknown) => err instanceof Error && err.message.includes("already exists"),
}));

vi.mock("@/api/apiResources", () => ({
  getApiGroupValue: (g: string) => g === "core" ? "" : g,
}));

describe("useConstraintSubmission", () => {
  const template: ConstraintTemplate = {
    name: "k8srequiredlabels",
    kind: "K8sRequiredLabels",
    description: "",
    rego: "",
    parameters: {},
    labels: {},
    createdAt: "",
  };

  const mockMethods = {
    setError: vi.fn(),
  } as unknown as Parameters<typeof useConstraintSubmission>[1];

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutateAsync.mockResolvedValue({ name: "test-constraint" });
  });

  it("returns createConstraint and onSubmit", () => {
    const { result } = renderHook(() =>
      useConstraintSubmission(template, mockMethods, false, false)
    );
    expect(result.current.createConstraint).toBeDefined();
    expect(result.current.onSubmit).toBeInstanceOf(Function);
  });

  it("submits successfully and returns true", async () => {
    const onSuccess = vi.fn();
    const { result } = renderHook(() =>
      useConstraintSubmission(template, mockMethods, false, false, onSuccess)
    );

    const data: ConstraintFormValues = {
      name: "test-constraint",
      enforcementAction: "deny",
    };

    let success: boolean = false;
    await act(async () => {
      success = await result.current.onSubmit(data);
    });

    expect(success).toBe(true);
    expect(mockMutateAsync).toHaveBeenCalled();
    expect(onSuccess).toHaveBeenCalledWith("test-constraint");
  });

  it("returns false on mutation error", async () => {
    mockMutateAsync.mockRejectedValue(new Error("server error"));

    const { result } = renderHook(() =>
      useConstraintSubmission(template, mockMethods, false, false)
    );

    const data: ConstraintFormValues = {
      name: "test",
      enforcementAction: "deny",
    };

    let success: boolean = true;
    await act(async () => {
      success = await result.current.onSubmit(data);
    });

    expect(success).toBe(false);
  });
});
