// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useState, Suspense } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router-dom";
import { RGDDetailView } from "@/components/detail";
import { useRGD } from "@/hooks/useRGDs";
import { useCanI } from "@/hooks/useCanI";
import { Loader2, AlertCircle } from "@/lib/icons";
import { lazyWithPreload } from "@/lib/lazy-preload";

// Lazy-load DeployModal — it pulls in the entire deploy form system (~675 lines)
// and is only rendered when the user clicks "Deploy"
const DeployModal = lazyWithPreload(() =>
  import("@/components/deploy/DeployModal").then(m => ({ default: m.DeployModal }))
);

export default function RGDDetailRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialTab = (searchParams.get("tab") || undefined) as "overview" | "resources" | "addons" | "depends-on" | "secrets" | "revisions" | undefined;
  const { allowed: canDeploy, isLoading: isLoadingCanDeploy, isError: isErrorCanDeploy } = useCanI('instances', 'create');

  const { data: rgd, isLoading, error } = useRGD(decodeURIComponent(rgdName || ''), undefined);

  const [deployModalOpen, setDeployModalOpen] = useState(false);

  const handleBack = useCallback(() => {
    navigate('/catalog');
  }, [navigate]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
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
    <>
      <RGDDetailView
        rgd={rgd}
        onBack={handleBack}
        onDeploy={canDeployRGD ? () => setDeployModalOpen(true) : undefined}
        initialTab={initialTab}
      />
      {deployModalOpen && (
        <Suspense fallback={null}>
          <DeployModal
            rgd={rgd}
            open={deployModalOpen}
            onClose={() => setDeployModalOpen(false)}
          />
        </Suspense>
      )}
    </>
  );
}
