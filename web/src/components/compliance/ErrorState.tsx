// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { AlertTriangle, RefreshCw } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface ErrorStateProps {
  /** Error message to display */
  message: string;
  /** Optional detailed error description */
  details?: string;
  /** Callback function for retry action */
  onRetry?: () => void;
  /** Optional className for customization */
  className?: string;
  /** Whether the retry is in progress */
  isRetrying?: boolean;
}

/**
 * Error state component with retry button
 * AC-SHARED-04: Error state with retry button
 */
export function ErrorState({
  message,
  details,
  onRetry,
  className,
  isRetrying = false,
}: ErrorStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-12 text-center",
        className
      )}
    >
      <div className="rounded-full bg-destructive/10 p-4 mb-4">
        <AlertTriangle className="h-8 w-8 text-destructive" />
      </div>
      <h3 className="text-lg font-semibold text-destructive">{message}</h3>
      {details && (
        <p className="text-sm text-muted-foreground mt-1 max-w-[300px]">
          {details}
        </p>
      )}
      {onRetry && (
        <Button
          variant="outline"
          className="mt-4"
          onClick={onRetry}
          disabled={isRetrying}
        >
          <RefreshCw
            className={cn("h-4 w-4 mr-2", isRetrying && "animate-spin")}
          />
          {isRetrying ? "Retrying..." : "Try Again"}
        </Button>
      )}
    </div>
  );
}

export default ErrorState;
