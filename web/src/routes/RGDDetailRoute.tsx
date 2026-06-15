// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router-dom";
import { RGDDetailView, RGDDetailSkeleton } from "@/components/detail";
import { useRGD } from "@/hooks/useRGDs";
import { useCanI } from "@/hooks/useCanI";
import { AlertCircle } from "@/lib/icons";

export default function RGDDetailRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialTab = (searchParams.get("tab") || undefined) as "overview" | "resources" | "addons" | "depends-on" | "secrets" | "revisions" | undefined;
  const { allowed: canDeploy, isLoading: isLoadingCanDeploy, isError: isErrorCanDeploy } = useCanI('instances', 'create');

  const { data: rgd, isLoading, error } = useRGD(decodeURIComponent(rgdName || ''), undefined);

  const handleBack = useCallback(() => {
    navigate('/catalog');
  }, [navigate]);

  if (isLoading) {
    // Skeleton matches RGDDetailView's header + tab bar + tabpanel shape so
    // the page doesn't jump when the RGD fetch resolves.
    return <RGDDetailSkeleton />;
  }

  if (error || !rgd) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <AlertCircle className="h-12 w-12 text-destructive" />
        <div className="text-center">
          <h2 className="text-lg font-semibold">RGD Not Found</h2>
          <p className="text-sm text-muted-foreground">
            The RGD "{rgdName}" could not be found.
          </p>
        </div>
        <button
          onClick={() => navigate('/catalog')}
          className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90"
        >
          Back to Catalog
        </button>
      </div>
    );
  }

  const canDeployRGD = !isErrorCanDeploy && (isLoadingCanDeploy || canDeploy);

  return (
    <RGDDetailView
      rgd={rgd}
      onBack={handleBack}
      onDeploy={
        canDeployRGD
          ? () => navigate(`/deploy/${encodeURIComponent(rgdName || "")}`)
          : undefined
      }
      initialTab={initialTab}
    />
  );
}
