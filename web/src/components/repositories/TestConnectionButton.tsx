// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { AlertCircle, CheckCircle2, Loader2 } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import type { TestConnectionResponse } from "@/types/repository";

interface TestConnectionButtonProps {
  onTest: () => void;
  isTesting: boolean;
  isDisabled: boolean;
  testResult: TestConnectionResponse | null;
}

export function TestConnectionButton({
  onTest,
  isTesting,
  isDisabled,
  testResult,
}: TestConnectionButtonProps) {
  return (
    <div>
      <Button
        type="button"
        variant="outline"
        onClick={onTest}
        disabled={isTesting || isDisabled}
        className="w-full"
      >
        {isTesting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {isTesting ? "Testing Connection..." : "Test Connection"}
      </Button>

      {testResult && (
        <div
          className={`mt-3 p-3 rounded-md flex items-start gap-2 ${
            testResult.valid
              ? "bg-status-success/10 text-status-success"
              : "bg-status-error/10 text-status-error"
          }`}
        >
          {testResult.valid ? (
            <CheckCircle2 className="h-5 w-5 flex-shrink-0 mt-0.5" />
          ) : (
            <AlertCircle className="h-5 w-5 flex-shrink-0 mt-0.5" />
          )}
          <p className="text-sm">{testResult.message}</p>
        </div>
      )}
    </div>
  );
}
