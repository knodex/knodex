// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { CatalogPage } from "@/components/catalog";
import { RGDDetailView } from "@/components/detail";
import { DeployPage } from "@/components/deploy";
import { useRGD } from "@/hooks/useRGDs";
import { useCanI } from "@/hooks/useCanI";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import type { CatalogRGD } from "@/types/rgd";
import { Loader2, AlertCircle } from "lucide-react";
import { logger } from "@/lib/logger";

export function CatalogRoute() {
  const navigate = useNavigate();

  const handleRGDClick = useCallback((rgd: CatalogRGD) => {
    navigate(`/catalog/${encodeURIComponent(rgd.name)}`);
  }, [navigate]);

  return <CatalogPage onRGDClick={handleRGDClick} />;
}

export function RGDDetailRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const navigate = useNavigate();
  // Real-time permission check via backend Casbin enforcer
  const { allowed: canDeploy, isLoading: isLoadingCanDeploy, isError: isErrorCanDeploy } = useCanI('instances', 'create');

  // Fetch RGD by name from the cache or API
  const { data: rgd, isLoading, error } = useRGD(decodeURIComponent(rgdName || ''), undefined);

  const handleBack = useCallback(() => {
    navigate('/catalog');
  }, [navigate]);

  const handleDeploy = useCallback(() => {
    navigate(`/catalog/${encodeURIComponent(rgdName!)}/deploy`);
  }, [navigate, rgdName]);

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

  return (
    <RGDDetailView
      rgd={rgd}
      onBack={handleBack}
      onDeploy={isLoadingCanDeploy || isErrorCanDeploy || canDeploy ? handleDeploy : undefined}
    />
  );
}

export function DeployRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const navigate = useNavigate();
  const { announce } = useAnnouncements();

  // Fetch RGD by name
  const { data: rgd, isLoading, error } = useRGD(decodeURIComponent(rgdName || ''), undefined);

  const handleBack = useCallback(() => {
    navigate(`/catalog/${encodeURIComponent(rgdName!)}`);
  }, [navigate, rgdName]);

  const handleDeploySuccess = useCallback((instanceName: string, namespace: string) => {
    logger.debug("[Deploy] Success:", instanceName, namespace);
    announce(`Successfully deployed ${instanceName} to ${namespace}`, "polite");
    // Navigate to instances page
    navigate('/instances');
  }, [navigate, announce]);

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

  return (
    <DeployPage
      rgd={rgd}
      onBack={handleBack}
      onDeploySuccess={handleDeploySuccess}
    />
  );
}
