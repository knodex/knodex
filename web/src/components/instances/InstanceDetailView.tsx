// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import yaml from "js-yaml";
import { FileCode } from "@/lib/icons";
import { CopyButton } from "@/components/ui/copy-button";
import { InstanceStatusCard } from "./InstanceStatusCard";
import { EditInstanceSpecDialog } from "./EditInstanceSpecDialog";
import { DeleteInstanceDialog } from "./DeleteInstanceDialog";
import { GitOpsDriftBanner } from "./GitOpsDriftBanner";
import type { Instance } from "@/types/rgd";
import { GitStatusDisplay } from "./GitStatusDisplay";
import { DeploymentTimeline } from "./DeploymentTimeline";
import { InstanceAddOns } from "./InstanceAddOns";
import { InstanceExternalRefs } from "./InstanceExternalRefs";
import { InstanceChildResources } from "./InstanceChildResources";
import { InstanceEvents } from "./InstanceEvents";
import { TabBar } from "@/components/shared/TabBar";
import { RevisionDiffDrawer } from "./RevisionDiffDrawer";
import { InstanceHeaderCard } from "./InstanceHeaderCard";
import { InstanceActionButtons } from "./InstanceActionButtons";
import { InstanceConditionsCard } from "./InstanceConditionsCard";
import { InstanceResourcesSummaryCard, InstanceRecentActivityCard } from "./InstanceOverviewRail";
import { useInstancePermissions } from "./hooks/useInstancePermissions";
import { useInstanceDialogs } from "./hooks/useInstanceDialogs";
import { useInstanceDeletion } from "./hooks/useInstanceDeletion";
import { useInstanceTabs } from "./hooks/useInstanceTabs";
import { useInstanceMetadata } from "./hooks/useInstanceMetadata";
import { useInstanceChildren } from "@/hooks/useInstances";
import { useInstanceEvents as useInstanceK8sEvents } from "@/hooks/useHistory";
import { useInstanceTimeline } from "@/hooks/useHistory";
import { apiGroupOf } from "@/lib/instancePath";

/** Spec viewer with copy button */
function SpecViewer({ spec }: { spec: Record<string, unknown> }) {
  const text = yaml.dump(spec, { lineWidth: 120, noRefs: true });
  const fieldCount = Object.keys(spec).length;

  return (
    <div className="rounded-lg border border-[var(--border-default)] bg-[var(--surface-primary)] overflow-hidden">
      <div className="flex items-center gap-2.5 px-4 py-3 border-b border-[var(--border-subtle)]">
        <FileCode className="h-4 w-4 text-[var(--text-muted)]" />
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Spec</h3>
        {fieldCount > 0 && (
          <span className="text-xs text-[var(--text-muted)]">
            {fieldCount} field{fieldCount === 1 ? "" : "s"}
          </span>
        )}
        <CopyButton
          text={text}
          label="Copy"
          variant="ghost"
          className="ml-auto h-7 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          iconClassName="h-3 w-3"
        />
      </div>
      {/* recessed surface mirrors the redesign's nested-area treatment */}
      <div className="p-4 overflow-x-auto bg-[var(--surface-bg)]">
        <pre className="text-xs leading-relaxed font-mono text-[var(--text-secondary)]" data-testid="spec-content">
          {text}
        </pre>
      </div>
    </div>
  );
}

interface InstanceDetailViewProps {
  instance: Instance;
  onDeleted?: () => void;
}

export function InstanceDetailView({
  instance,
  onDeleted,
}: InstanceDetailViewProps) {
  const metadata = useInstanceMetadata(instance);
  const permissions = useInstancePermissions(instance, metadata.parentRGD);
  const dialogs = useInstanceDialogs();
  const { handleDelete, deleteInstance } = useInstanceDeletion(instance, () => {
    dialogs.setShowDeleteDialog(false);
    onDeleted?.();
  });
  const group = apiGroupOf(instance.apiVersion);

  // Overview rail + rollup data (React Query dedupes with the tab components)
  const { data: childrenData } = useInstanceChildren(group, instance.namespace, instance.kind, instance.name);
  const { data: eventsData } = useInstanceK8sEvents(group, instance.namespace, instance.kind, instance.name);
  const { data: timelineData } = useInstanceTimeline(group, instance.namespace, instance.kind, instance.name);

  const resourceGroups = childrenData?.groups ?? [];
  const resourcesTotal = resourceGroups.reduce((sum, g) => sum + g.count, 0);
  const resourcesReady = resourceGroups.reduce((sum, g) => sum + g.readyCount, 0);
  const resourcesFailing = resourceGroups
    .flatMap((g) => g.resources)
    .filter((r) => r.health === "Unhealthy" || r.health === "Degraded").length;
  const events = eventsData?.events ?? [];
  const eventsWarnings = events.filter((e) => e.type === "Warning").length;
  const conditions = instance.conditions ?? [];
  const conditionsPassing = conditions.filter((c) => c.status === "True").length;
  const lastReconciled =
    conditions.reduce<string | undefined>(
      (latest, c) =>
        c.lastTransitionTime && (!latest || c.lastTransitionTime > latest)
          ? c.lastTransitionTime
          : latest,
      undefined
    ) ?? instance.updatedAt;

  const { tabs, activeTab: effectiveTab, setActiveTab } = useInstanceTabs(
    instance,
    {
      events: metadata.eventsCount,
      externalRefs: metadata.externalRefCount,
      resourcesReady,
      resourcesTotal,
      history: timelineData?.timeline?.length ?? 0,
    },
    metadata.hasSpec,
  );

  return (
    <div className="space-y-0 animate-fade-in">
      {/* ── Header: identity + actions + meta chips + health rollup ── */}
      <InstanceHeaderCard
        instance={instance}
        parentRGD={metadata.parentRGD}
        canReadRGD={permissions.canReadRGD}
        kroState={metadata.kroState}
        isGitOps={metadata.isGitOps}
        rollup={{
          conditionsPassing,
          conditionsTotal: conditions.length,
          resourcesReady,
          resourcesTotal,
          resourcesFailing,
          eventsCount: events.length,
          eventsWarnings,
          lastReconciled,
        }}
        actions={
          <InstanceActionButtons
            instanceUrl={metadata.instanceUrl}
            canUpdate={permissions.canUpdate}
            isLoadingCanUpdate={permissions.isLoadingCanUpdate}
            isErrorCanUpdate={permissions.isErrorCanUpdate}
            canDelete={permissions.canDelete}
            isLoadingCanDelete={permissions.isLoadingCanDelete}
            isErrorCanDelete={permissions.isErrorCanDelete}
            isTerminal={metadata.isTerminal}
            isDeleting={metadata.isDeleting}
            kroState={metadata.kroState}
            onEdit={() => dialogs.setShowEditDialog(true)}
            onDelete={() => dialogs.setShowDeleteDialog(true)}
          />
        }
        onRevisionClick={() => dialogs.setShowRevisionDrawer(true)}
        onSelectTab={setActiveTab}
      />

      {/* Tabs */}
      <div className="mt-6">
        <TabBar tabs={tabs} activeTab={effectiveTab} onChange={setActiveTab} />
      </div>

      {/* Tab content */}
      <div id={`panel-${effectiveTab}`} className="min-h-[300px] mt-6" key={effectiveTab} role="tabpanel" aria-labelledby={`tab-${effectiveTab}`}>
        {effectiveTab === "status" && (
          <div className="space-y-4">
            <GitOpsDriftBanner instance={instance} />
            {metadata.isGitOps && (
              <GitStatusDisplay
                deploymentMode={instance.deploymentMode}
                gitInfo={instance.gitInfo}
                annotations={instance.annotations}
                reconciliationSuspended={instance.reconciliationSuspended}
              />
            )}
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-4 items-start">
              <div className="space-y-4 min-w-0">
                {conditions.length > 0 && <InstanceConditionsCard conditions={conditions} />}
                {instance.status && <InstanceStatusCard status={instance.status} />}
              </div>
              <div className="space-y-4">
                <InstanceResourcesSummaryCard
                  groups={resourceGroups}
                  totalCount={resourcesTotal}
                  onViewAll={() => setActiveTab("children")}
                />
                <InstanceRecentActivityCard
                  events={events}
                  onViewAll={() => setActiveTab("events")}
                />
              </div>
            </div>
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
          <DeploymentTimeline group={group} namespace={instance.namespace} kind={instance.kind} name={instance.name} />
        )}
        {effectiveTab === "events" && (
          <InstanceEvents group={group} namespace={instance.namespace} kind={instance.kind} name={instance.name} />
        )}
        {effectiveTab === "external-refs" && (
          <InstanceExternalRefs instance={instance} />
        )}
        {effectiveTab === "children" && (
          <InstanceChildResources group={group} namespace={instance.namespace} kind={instance.kind} name={instance.name} />
        )}
        {effectiveTab === "spec" && instance.spec && Object.keys(instance.spec).length > 0 && (
          <SpecViewer spec={instance.spec} />
        )}
      </div>

      {/* Edit Spec Dialog */}
      <EditInstanceSpecDialog
        instance={instance}
        open={dialogs.showEditDialog}
        onOpenChange={dialogs.setShowEditDialog}
      />

      {/* Delete Instance Dialog */}
      <DeleteInstanceDialog
        instance={instance}
        isOpen={dialogs.showDeleteDialog}
        onConfirm={handleDelete}
        onCancel={() => {
          dialogs.setShowDeleteDialog(false);
          deleteInstance.reset();
        }}
        isDeleting={deleteInstance.isPending}
        error={deleteInstance.error}
      />

      {/* Revision Diff Drawer */}
      {permissions.canReadRGD && metadata.parentRGD?.lastIssuedRevision ? (
        <RevisionDiffDrawer
          rgdName={instance.rgdName}
          currentRevision={metadata.parentRGD.lastIssuedRevision}
          open={dialogs.showRevisionDrawer}
          onOpenChange={dialogs.setShowRevisionDrawer}
        />
      ) : null}
    </div>
  );
}
