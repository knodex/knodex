// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { toast } from "sonner";
import type { UseFormReturn } from "react-hook-form";
import { useCreateConstraint } from "@/hooks/useCompliance";
import { isAlreadyExists } from "@/api/compliance";
import type { ConstraintTemplate } from "@/types/compliance";
import type { ConstraintFormValues } from "./useConstraintFormValidation";
import { cleanParameters, buildMatchRules } from "./constraintUtils";

/**
 * Hook encapsulating constraint form submission logic.
 * Returns { createConstraint, onSubmit } where onSubmit returns true on success.
 */
export function useConstraintSubmission(
  template: ConstraintTemplate,
  methods: UseFormReturn<ConstraintFormValues>,
  canUseFormMode: boolean,
  useFormMode: boolean,
  onSuccess?: (constraintName: string) => void
) {
  const createConstraint = useCreateConstraint();

  const onSubmit = useCallback(async (data: ConstraintFormValues): Promise<boolean> => {
    try {
      let parameters: Record<string, unknown> | undefined;

      if (canUseFormMode && useFormMode && data.params) {
        parameters = cleanParameters(data.params);
      } else if (data.parametersRaw && data.parametersRaw.trim() !== "{}") {
        try {
          parameters = JSON.parse(data.parametersRaw);
        } catch {
          methods.setError("parametersRaw", {
            type: "manual",
            message: "Invalid JSON format",
          });
          return false;
        }
      }

      const match = buildMatchRules(data);

      const result = await createConstraint.mutateAsync({
        name: data.name,
        templateName: template.name,
        enforcementAction: data.enforcementAction,
        match: match || undefined,
        parameters,
      });

      toast.success(`Constraint "${result.name}" created successfully`);
      onSuccess?.(result.name);
      return true;
    } catch (error) {
      if (isAlreadyExists(error)) {
        methods.setError("name", {
          type: "manual",
          message: `A constraint named "${data.name}" already exists`,
        });
      }
      return false;
    }
  }, [template.name, methods, canUseFormMode, useFormMode, createConstraint, onSuccess]);

  return { createConstraint, onSubmit };
}
