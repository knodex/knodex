// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useState, useMemo } from "react";
import {
  ArrowLeft,
  Box,
  Pencil,
  Trash2,
  Clock,
  AlertCircle,
  Activity,
  Code,
  Puzzle,
} from "lucide-react";
import { InstanceStatusCard } from "./InstanceStatusCard";
import { EditInstanceSpecDialog } from "./EditInstanceSpecDialog";
import { GitOpsDriftBanner } from "./GitOpsDriftBanner";
import { Button } from "@/components/ui/button";
import type { Instance } from "@/types/rgd";
import { HealthBadge } from "./HealthBadge";
import { GitStatusDisplay } from "./GitStatusDisplay";
import { StatusTimeline } from "./StatusTimeline";
import { DeploymentTimeline } from "./DeploymentTimeline";
import { InstanceAddOns } from "./InstanceAddOns";
import { InstanceDependsOn } from "./InstanceDependsOn";
import { useDeleteInstance } from "@/hooks/useInstances";
import { useCanI } from "@/hooks/useCanI";
import { useRGDList } from "@/hooks/useRGDs";
import { INSTANCE_ID_ANNOTATION } from "@/types/rgd";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab, ConditionalTab } from "@/hooks/useDynamicTabs";
import { TabBar } from "@/components/shared/TabBar";

type TabId = "status" | "addons" | "deployment-history" | "spec";

const BASE_TABS: Tab<TabId>[] = [
  { id: "status", label: "Status", icon: <Activity className="h-4 w-4" /> },
  { id: "deployment-history", label: "Deployment History", icon: <Clock className="h-4 w-4" /> },
];

interface InstanceDetailViewProps {
  instance: Instance;
  onBack: () => void;
  onDeleted?: () => void;
}

export function InstanceDetailView({
  instance,
  onBack,
  onDeleted,
}: InstanceDetailViewProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);

  const deleteInstance = useDeleteInstance();
  // Real-time permission checks via backend Casbin enforcer
  const { allowed: canDelete, isLoading: isLoadingCanDelete, isError: isErrorCanDelete } = useCanI('instances', 'delete', instance.namespace || '-');
  const { allowed: canUpdate, isLoading: isLoadingCanUpdate, isError: isErrorCanUpdate } = useCanI('instances', 'update', instance.namespace || '-');

  // Fetch add-ons count for tab visibility (React Query deduplicates with InstanceAddOns)
  const { data: addOnsData } = useRGDList(
    instance.kind ? { extendsKind: instance.kind, pageSize: 100 } : undefined
  );
  const addOnsCount = addOnsData?.totalCount ?? 0;

  // Build conditional tabs
  const hasSpec = !!(instance.spec && Object.keys(instance.spec).length > 0);

  const conditionalTabs = useMemo<ConditionalTab<TabId>[]>(() => [
    {
      condition: addOnsCount > 0,
      tab: { id: "addons", label: `Add-ons (${addOnsCount})`, icon: <Puzzle className="h-4 w-4" /> },
      position: 1,
    },
    {
      condition: hasSpec,
      tab: { id: "spec", label: "Spec", icon: <Code className="h-4 w-4" /> },
    },
  ], [addOnsCount, hasSpec]);

  const { tabs, activeTab: effectiveTab, setActiveTab } = useDynamicTabs(BASE_TABS, conditionalTabs, "status" as TabId);

  const handleDelete = useCallback(async () => {
    try {
      await deleteInstance.mutateAsync({
        namespace: instance.namespace,
        kind: instance.kind,
        name: instance.name,
      });
      onDeleted?.();
    } catch {
      // Error handled by mutation
    }
  }, [deleteInstance, instance.namespace, instance.kind, instance.name, onDeleted]);

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-4">
          <Button
            variant="ghost"
            size="sm"
            onClick={onBack}
            className="gap-1.5 text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
            Back
          </Button>
        </div>
        <div className="flex items-center gap-2">
          {/* Edit Spec button — shown optimistically during loading (same pattern as delete) */}
          {(isLoadingCanUpdate || isErrorCanUpdate || canUpdate) && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowEditDialog(true)}
              className="gap-1.5"
            >
              <Pencil className="h-3.5 w-3.5" />
              Edit Spec
            </Button>
          )}
          {/* Delete button — only shown if user has delete permission (optimistic during loading) */}
          {(isLoadingCanDelete || isErrorCanDelete || canDelete) && (
            <>
              {!showDeleteConfirm ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowDeleteConfirm(true)}
                  className="gap-1.5 text-destructive border-destructive/30 hover:bg-destructive/10"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  Delete
                </Button>
            ) : (
              <div className="flex flex-col gap-2 p-3 rounded-lg border border-destructive/30 bg-destructive/5">
                <div className="flex items-center gap-2 text-sm text-destructive">
                  <AlertCircle className="h-4 w-4 shrink-0" />
                  <span className="font-medium">Delete "{instance.name}"?</span>
                </div>
                <p className="text-xs text-destructive/80 pl-6">
                  This action cannot be undone.
                </p>
                <div className="flex items-center gap-2 pl-6 pt-1">
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={handleDelete}
                    disabled={deleteInstance.isPending}
                  >
                    {deleteInstance.isPending ? "Deleting..." : "Yes, delete"}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setShowDeleteConfirm(false)}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}
            </>
          )}
        </div>
      </div>

      {/* Instance info card */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-start gap-4 mb-6">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-secondary text-muted-foreground">
            <Box className="h-6 w-6" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-3 mb-1">
              <h1 className="text-xl font-semibold text-foreground truncate">
                {instance.name}
              </h1>
              <HealthBadge health={instance.health} />
            </div>
            <p className="text-sm text-muted-foreground font-mono">
              {instance.namespace}
            </p>
          </div>
        </div>

        {/* Metadata grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 py-4 border-y border-border">
          <div>
            <span className="text-xs text-muted-foreground block mb-1">RGD</span>
            <span className="text-sm font-mono text-foreground">
              {instance.rgdName}
            </span>
          </div>
          <div>
            <span className="text-xs text-muted-foreground block mb-1">Kind</span>
            <span className="text-sm font-mono text-foreground">
              {instance.kind}
            </span>
          </div>
          <div>
            <span className="text-xs text-muted-foreground block mb-1">
              API Version
            </span>
            <span className="text-sm font-mono text-foreground">
              {instance.apiVersion}
            </span>
          </div>
          <div>
            <span className="text-xs text-muted-foreground block mb-1">
              Created
            </span>
            <span className="text-sm text-foreground flex items-center gap-1.5">
              <Clock className="h-3.5 w-3.5" />
              {new Date(instance.createdAt).toLocaleString()}
            </span>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <TabBar tabs={tabs} activeTab={effectiveTab} onChange={setActiveTab} />

      {/* Tab content */}
      <div id={`panel-${effectiveTab}`} className="min-h-[300px]" role="tabpanel" aria-labelledby={`tab-${effectiveTab}`}>
        {effectiveTab === "status" && (
          <div className="space-y-6">
            <GitStatusDisplay
              deploymentMode={instance.deploymentMode}
              gitInfo={instance.gitInfo}
              annotations={instance.annotations}
            />
            <GitOpsDriftBanner instance={instance} />
            {(instance.deploymentMode === "gitops" || instance.deploymentMode === "hybrid") && (
              <StatusTimeline
                instanceId={instance.annotations?.[INSTANCE_ID_ANNOTATION]}
              />
            )}
            {(instance.status || (instance.conditions && instance.conditions.length > 0)) && (
              <InstanceStatusCard
                status={instance.status}
                conditions={instance.conditions}
              />
            )}
            {/* Depends On (externalRef dependencies) — rendered within the Status tab */}
            <InstanceDependsOn instance={instance} />
          </div>
        )}
        {effectiveTab === "addons" && instance.kind && (
          <InstanceAddOns
            kind={instance.kind}
            instanceName={instance.name}
            instanceNamespace={instance.namespace}
          />
        )}
        {effectiveTab === "deployment-history" && (
          <DeploymentTimeline namespace={instance.namespace} kind={instance.kind} name={instance.name} />
        )}
        {effectiveTab === "spec" && instance.spec && Object.keys(instance.spec).length > 0 && (
          <div className="rounded-lg border border-border bg-card p-4">
            <pre className="text-xs font-mono text-muted-foreground overflow-x-auto" data-testid="spec-content">
              {JSON.stringify(instance.spec, null, 2)}
            </pre>
          </div>
        )}
      </div>

      {/* Delete error */}
      {deleteInstance.isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 flex items-center gap-3">
          <AlertCircle className="h-5 w-5 text-destructive shrink-0" />
          <div>
            <p className="text-sm font-medium text-destructive">
              Failed to delete instance
            </p>
            <p className="text-xs text-destructive/80">
              {deleteInstance.error instanceof Error
                ? deleteInstance.error.message
                : "An unexpected error occurred"}
            </p>
          </div>
        </div>
      )}

      {/* Edit Spec Dialog */}
      <EditInstanceSpecDialog
        instance={instance}
        open={showEditDialog}
        onOpenChange={setShowEditDialog}
      />
    </div>
  );
}
