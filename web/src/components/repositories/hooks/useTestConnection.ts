// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect } from "react";
import type { UseFormWatch } from "react-hook-form";
import type {
  AuthType,
  TestConnectionRequest,
  TestConnectionResponse,
} from "@/types/repository";
import { buildAuthPayload } from "./buildAuthPayload";
import type { RepositoryFormData } from "./types";

/**
 * Hook encapsulating connection testing logic, result state, and error handling.
 */
export function useTestConnection(
  watch: UseFormWatch<RepositoryFormData>,
  selectedAuthType: AuthType,
  onTestConnection?: (data: TestConnectionRequest) => Promise<TestConnectionResponse>
) {
  const [testResult, setTestResult] = useState<TestConnectionResponse | null>(null);
  const [isTesting, setIsTesting] = useState(false);

  // Reset test result when auth type changes
  useEffect(() => {
    setTestResult(null);
  }, [selectedAuthType]);

  const handleTestConnection = useCallback(async () => {
    if (!onTestConnection) return;

    const formData = watch();
    setIsTesting(true);
    setTestResult(null);

    try {
      const request: TestConnectionRequest = {
        repoURL: formData.repoURL,
        authType: formData.authType,
        ...buildAuthPayload(formData),
      };

      const result = await onTestConnection(request);
      setTestResult(result);
    } catch (error) {
      setTestResult({
        valid: false,
        message: error instanceof Error ? error.message : "Connection test failed",
      });
    } finally {
      setIsTesting(false);
    }
  }, [onTestConnection, watch]);

  return { testResult, isTesting, handleTestConnection };
}
