// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useRef, useState } from "react";
import type { UseFormReturn } from "react-hook-form";
import { preflightInstance } from "@/api/rgd";
import {
  validateCompliance,
  type ComplianceValidateViolation,
} from "@/api/compliance";
import type { FormSchema } from "@/types/rgd";

/** Recursively sort object keys so JSON.stringify produces a stable hash. */
function sortKeysDeep(value: unknown): unknown {
  if (value === null || value === undefined) return value;
  if (Array.isArray(value)) return value.map(sortKeysDeep);
  if (typeof value === "object") {
    const obj = value as Record<string, unknown>;
    const sorted: Record<string, unknown> = {};
    for (const k of Object.keys(obj).sort()) {
      sorted[k] = sortKeysDeep(obj[k]);
    }
    return sorted;
  }
  return value;
}

export interface UseComplianceValidationOptions {
  /** The active tab id — checks only run when this equals `"review"`. */
  activeTabId: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  form: UseFormReturn<any>;
  schema: FormSchema;
  isClusterScoped: boolean;
  rgdName: string;
  /** Keys to strip from the spec payload before sending to the API. */
  reservedKeys: readonly string[];
}

export interface UseComplianceValidationResult {
  complianceResult: "pass" | "warning" | "block";
  complianceViolations: ComplianceValidateViolation[];
  warningsAcknowledged: boolean;
  setWarningsAcknowledged: (v: boolean) => void;
  preflightValid: boolean;
  preflightMessage: string | undefined;
  isValidating: boolean;
  isPreflighting: boolean;
}

/**
 * Runs compliance policy validation and Kubernetes preflight checks whenever
 * the user is on the Review tab.  Debounces form-value changes (250 ms) and
 * short-circuits when a required namespace is not yet set.
 */
export function useComplianceValidation({
  activeTabId,
  form,
  schema,
  isClusterScoped,
  rgdName,
  reservedKeys,
}: UseComplianceValidationOptions): UseComplianceValidationResult {
  const [complianceResult, setComplianceResult] =
    useState<"pass" | "warning" | "block">("pass");
  const [complianceViolations, setComplianceViolations] = useState<
    ComplianceValidateViolation[]
  >([]);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const [preflightValid, setPreflightValid] = useState(true);
  const [preflightMessage, setPreflightMessage] = useState<string | undefined>();
  const [isValidating, setIsValidating] = useState(false);
  const [isPreflighting, setIsPreflighting] = useState(false);
  const lastFetchedHashRef = useRef<string>("");

  const reservedSet = useRef(new Set<string>(reservedKeys));
  // Keep the set in sync if reservedKeys reference changes (it's a const array
  // in practice, but defensively update it).
  useEffect(() => {
    reservedSet.current = new Set<string>(reservedKeys);
  }, [reservedKeys]);

  // Re-run compliance + preflight when entering Review tab or when form values change.
  // Using form.watch subscription (non-render) avoids re-rendering DeployPageContent on
  // every keystroke, which would propagate to GeneralTab and cause Radix Select's SlotClone
  // to create a new composeRefs function every render (triggering an infinite setState loop).
  useEffect(() => {
    if (activeTabId !== "review") return;

    const ac = new AbortController();
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;

    const runChecks = () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        const allValues = form.getValues() as Record<string, unknown>;
        const runHash = JSON.stringify(sortKeysDeep(allValues));
        if (runHash === lastFetchedHashRef.current) return;
        lastFetchedHashRef.current = runHash;

        const namespace =
          (form.getValues("namespace") as string | undefined) ?? "";
        const project = (form.getValues("project") as string | undefined) ?? "";

        const copy = { ...allValues };
        for (const k of reservedSet.current) delete copy[k];
        const spec = copy;

        // Short-circuit when the namespace is required but not yet picked.
        // The backend would otherwise post to a namespaced URL with no
        // namespace segment — the API returns "no matches", which the error
        // mapper used to surface as a misleading "RGD not registered" toast.
        // Surface the real reason instead and don't bother the server.
        if (!isClusterScoped && !namespace.trim()) {
          setIsValidating(false);
          setIsPreflighting(false);
          setComplianceResult("pass");
          setComplianceViolations([]);
          setPreflightValid(false);
          setPreflightMessage(
            "Namespace is required for this resource. Pick a target namespace before deploying."
          );
          lastFetchedHashRef.current = ""; // re-run once the user fills it in
          return;
        }

        setIsValidating(true);
        setIsPreflighting(true);
        setWarningsAcknowledged(false);

        void validateCompliance({
          rgdName,
          project,
          namespace: isClusterScoped ? "" : namespace || "",
          values: allValues,
        })
          .then((res) => {
            if (ac.signal.aborted) return;
            setComplianceResult(res.result);
            setComplianceViolations(res.violations);
          })
          .catch(() => {
            if (ac.signal.aborted) return;
            setComplianceResult("pass");
            setComplianceViolations([]);
          })
          .finally(() => {
            if (ac.signal.aborted) return;
            setIsValidating(false);
          });

        void preflightInstance(schema.group, schema.kind, {
          name: "preflight-check",
          namespace: isClusterScoped ? undefined : namespace || undefined,
          projectId: project,
          rgdName,
          spec,
        })
          .then((res) => {
            if (ac.signal.aborted) return;
            setPreflightValid(res.valid);
            setPreflightMessage(res.message);
          })
          .catch(() => {
            if (ac.signal.aborted) return;
            setPreflightValid(true);
            setPreflightMessage(undefined);
          })
          .finally(() => {
            if (ac.signal.aborted) return;
            setIsPreflighting(false);
          });
      }, 250);
    };

    // Run immediately when entering Review tab.
    runChecks();

    // Subscribe to form value changes while on Review tab (non-render subscription).
    const subscription = form.watch(() => {
      if (!ac.signal.aborted) runChecks();
    });

    return () => {
      clearTimeout(debounceTimer);
      ac.abort();
      subscription.unsubscribe();
    };
  }, [activeTabId, form, schema, isClusterScoped, rgdName]);

  return {
    complianceResult,
    complianceViolations,
    warningsAcknowledged,
    setWarningsAcknowledged,
    preflightValid,
    preflightMessage,
    isValidating,
    isPreflighting,
  };
}
