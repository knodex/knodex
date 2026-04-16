// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Suspense } from "react";
import { useIsMobile } from "@/hooks/useIsMobile";
import { DeployDisabledRoute } from "@/lib/route-preloads";
import { PageSkeleton } from "@/components/ui/page-skeleton";

interface MobileDeployGuardProps {
  children: React.ReactNode;
}

/**
 * Wraps deploy routes — shows the deploy-disabled page on mobile viewports.
 */
export function MobileDeployGuard({ children }: MobileDeployGuardProps) {
  const isMobile = useIsMobile();

  if (isMobile) {
    return (
      <Suspense fallback={<PageSkeleton />}>
        <DeployDisabledRoute />
      </Suspense>
    );
  }

  return <>{children}</>;
}
