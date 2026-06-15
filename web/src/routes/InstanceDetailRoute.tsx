// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { InstanceDetailView } from "@/components/instances";
import { InstanceDetailSkeleton } from "@/components/instances/instance-detail-skeleton";
import { useInstance } from "@/hooks/useInstances";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import { AlertCircle } from "@/lib/icons";

/**
 * URL params for /instances/:group/:version/((:namespace/:kind/:name)|(:kind/:name)).
 * Cluster-scoped routes do not bind :namespace; we detect that variant by its
 * absence rather than by an empty-string sentinel, matching the K8s API convention.
 */
type InstanceRouteParams = {
  group: string;
  version: string;
  namespace?: string;
  kind: string;
  name: string;
};

export default function InstanceDetailRoute() {
  const { group, namespace, kind, name } = useParams<InstanceRouteParams>();
  const navigate = useNavigate();
  const { announce } = useAnnouncements();

  const decodedGroup = decodeURIComponent(group || "");
  const decodedNamespace = namespace ? decodeURIComponent(namespace) : "";
  const decodedKind = decodeURIComponent(kind || "");
  const decodedName = decodeURIComponent(name || "");

  const { data: instance, isLoading, error } = useInstance(
    decodedGroup,
    decodedNamespace,
    decodedKind,
    decodedName,
  );

  const handleInstanceDeleted = useCallback(() => {
    announce("Instance deleted successfully", "polite");
    navigate('/instances');
  }, [navigate, announce]);

  if (isLoading) {
    // Skeleton mirrors InstanceDetailView's exact layout (header card + tab
    // bar + tabpanel) so the page doesn't reflow when data lands.
    return <InstanceDetailSkeleton />;
  }

  if (error || !instance) {
    const identity = decodedNamespace
      ? `${decodedGroup}/${decodedNamespace}/${decodedKind}/${decodedName}`
      : `${decodedGroup}/${decodedKind}/${decodedName}`;
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <AlertCircle className="h-12 w-12 text-destructive" />
        <div className="text-center">
          <h2 className="text-lg font-semibold">Instance Not Found</h2>
          <p className="text-sm text-muted-foreground">
            The instance "{identity}" could not be found.
          </p>
        </div>
        <button
          onClick={() => navigate('/instances')}
          className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90"
        >
          Back to Instances
        </button>
      </div>
    );
  }

  return (
    <InstanceDetailView
      instance={instance}
      onDeleted={handleInstanceDeleted}
    />
  );
}
