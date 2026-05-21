// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Loader2, Eye } from "@/lib/icons";
import { Button } from "@/components/ui/button";

interface DeployActionFooterProps {
  onPrev: () => void;
  onNext: () => void;
  onDeploy: () => void;
  onGoToReview: () => void;
  isOnFirst: boolean;
  isOnReview: boolean;
  canDeploy: boolean;
  isSubmitting: boolean;
}

export function DeployActionFooter({
  onPrev,
  onNext,
  onDeploy,
  onGoToReview,
  isOnFirst,
  isOnReview,
  canDeploy,
  isSubmitting,
}: DeployActionFooterProps) {
  return (
    <div
      className="fixed bottom-0 left-0 right-0 z-10 flex items-center justify-between gap-3 border-t bg-background px-6 py-4 lg:left-[260px]"
      style={{ borderColor: "rgba(255,255,255,0.08)" }}
    >
      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="outline"
          onClick={onPrev}
          disabled={isOnFirst || isSubmitting}
          data-testid="deploy-footer-prev"
        >
          Previous
        </Button>

        {!isOnReview && (
          <Button
            type="button"
            variant="outline"
            onClick={onNext}
            disabled={isSubmitting}
            data-testid="deploy-footer-next"
          >
            Next
          </Button>
        )}
      </div>

      <div className="flex items-center gap-3">
        {isOnReview ? (
          <Button
            type="button"
            variant="default"
            onClick={onDeploy}
            disabled={!canDeploy || isSubmitting}
            data-testid="deploy-footer-deploy"
          >
            {isSubmitting && <Loader2 className="animate-spin" />}
            Deploy
          </Button>
        ) : (
          <Button
            type="button"
            variant="secondary"
            onClick={onGoToReview}
            disabled={isSubmitting}
            data-testid="deploy-footer-review"
          >
            <Eye className="h-4 w-4" />
            Review and Deploy
          </Button>
        )}
      </div>
    </div>
  );
}
