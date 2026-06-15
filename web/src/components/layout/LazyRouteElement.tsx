// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Suspense, type ComponentType, type ReactElement, type ReactNode } from "react";
import { RouteErrorBoundary } from "@/components/ui/route-error-boundary";
import { PageSkeleton } from "@/components/ui/page-skeleton";

interface LazyRouteElementProps {
  component: ComponentType;
  /** Optional layout/guard rendered INSIDE RouteErrorBoundary and OUTSIDE Suspense
   *  (mirrors the existing SettingsLayout / MobileDeployGuard placement). */
  wrapper?: ComponentType<{ children: ReactNode }>;
}

export function LazyRouteElement({ component: Component, wrapper: Wrapper }: LazyRouteElementProps): ReactElement {
  const inner = (
    <Suspense fallback={<PageSkeleton />}>
      <Component />
    </Suspense>
  );
  return <RouteErrorBoundary>{Wrapper ? <Wrapper>{inner}</Wrapper> : inner}</RouteErrorBoundary>;
}
