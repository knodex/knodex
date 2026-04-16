// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Rocket, ArrowRight, LayoutGrid } from "@/lib/icons";
import { useDeployContextStore } from "@/stores/deploy-context-store";
import apiClient from "@/api/client";

interface RelatedRGD {
  name: string;
  displayName: string;
  description: string;
  relationship: string;
}

interface ContinuationPanelProps {
  rgdName: string;
  instanceName: string;
  namespace: string;
  kind: string;
}

export function ContinuationPanel({
  rgdName,
  instanceName,
  namespace,
  kind,
}: ContinuationPanelProps) {
  const navigate = useNavigate();
  const { stackProgress, startChain, advanceChain } = useDeployContextStore();
  const [relatedRgds, setRelatedRgds] = useState<RelatedRGD[]>([]);

  // Fetch related RGDs
  useEffect(() => {
    apiClient
      .get<{ related: RelatedRGD[] }>(`/v1/rgds/${encodeURIComponent(rgdName)}/related`)
      .then((res) => setRelatedRgds(res.data.related ?? []))
      .catch(() => setRelatedRgds([]));
  }, [rgdName]);

  const handleViewInstance = useCallback(() => {
    navigate(`/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(instanceName)}`);
  }, [navigate, namespace, kind, instanceName]);

  const handleDeployNext = useCallback(
    (nextRgdName: string) => {
      // Start or advance chain
      if (!stackProgress) {
        startChain(rgdName, relatedRgds.length + 1);
      } else {
        advanceChain();
      }
      navigate(`/deploy/${encodeURIComponent(nextRgdName)}`);
    },
    [navigate, rgdName, relatedRgds.length, stackProgress, startChain, advanceChain]
  );

  const handleBackToCatalog = useCallback(() => {
    useDeployContextStore.getState().clearContext();
    navigate("/catalog");
  }, [navigate]);

  return (
    <div className="space-y-4 pt-4" data-testid="continuation-panel">
      {/* Stack progress indicator */}
      {stackProgress && (
        <p className="text-xs text-center" style={{ color: "var(--text-muted)" }}>
          {stackProgress.current} of {stackProgress.total} in your stack
        </p>
      )}

      {/* Related RGDs */}
      {relatedRgds.length > 0 && (
        <div>
          <h3 className="text-sm font-medium mb-3" style={{ color: "var(--text-primary)" }}>
            What&apos;s next?
          </h3>
          <div className="space-y-2">
            {relatedRgds.map((rgd) => (
              <div
                key={rgd.name}
                className="flex items-center justify-between rounded-lg border p-3"
                style={{
                  backgroundColor: "rgba(255,255,255,0.02)",
                  borderColor: "rgba(255,255,255,0.08)",
                }}
              >
                <div className="flex items-center gap-3 min-w-0">
                  <Rocket className="h-4 w-4 shrink-0" style={{ color: "var(--text-muted)" }} />
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate" style={{ color: "var(--text-primary)" }}>
                      {rgd.displayName || rgd.name}
                    </p>
                    <p className="text-xs truncate" style={{ color: "var(--text-muted)" }}>
                      Usually deployed with {rgdName}
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => handleDeployNext(rgd.name)}
                  className="flex items-center gap-1 px-3 py-1 rounded-md text-xs font-medium shrink-0"
                  style={{ backgroundColor: "var(--brand-primary)", color: "var(--surface-bg)" }}
                >
                  Deploy Next
                  <ArrowRight className="h-3 w-3" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Action buttons */}
      <div className="flex items-center justify-center gap-3 pt-2">
        <button
          type="button"
          onClick={handleViewInstance}
          className="px-4 py-2 rounded-md text-sm font-medium"
          style={{ backgroundColor: "var(--brand-primary)", color: "var(--surface-bg)" }}
        >
          View Instance
        </button>
        <button
          type="button"
          onClick={handleBackToCatalog}
          className="flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium border"
          style={{ color: "var(--text-secondary)", borderColor: "rgba(255,255,255,0.1)" }}
        >
          <LayoutGrid className="h-3.5 w-3.5" />
          Back to Catalog
        </button>
      </div>
    </div>
  );
}
