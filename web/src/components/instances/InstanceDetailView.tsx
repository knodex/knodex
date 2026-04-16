// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import {
  Code,
  Copy,
  Check,
} from "@/lib/icons";
import { InstanceStatusCard } from "./InstanceStatusCard";
import { EditInstanceSpecDialog } from "./EditInstanceSpecDialog";
import { DeleteInstanceDialog } from "./DeleteInstanceDialog";
import { GitOpsDriftBanner } from "./GitOpsDriftBanner";
import { Button } from "@/components/ui/button";
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
import { InstanceMetadataSection } from "./InstanceMetadataSection";
import { useInstancePermissions } from "./hooks/useInstancePermissions";
import { useInstanceDialogs } from "./hooks/useInstanceDialogs";
import { useInstanceDeletion } from "./hooks/useInstanceDeletion";
import { useInstanceTabs } from "./hooks/useInstanceTabs";
import { useInstanceMetadata } from "./hooks/useInstanceMetadata";

/** Spec viewer with copy button */
function SpecViewer({ spec }: { spec: Record<string, unknown> }) {
  const [copied, setCopied] = useState(false);
  const json = JSON.stringify(spec, null, 2);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(json).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [json]);

  return (
    <div className="rounded-lg border overflow-hidden" style={{ borderColor: "var(--border-default)", background: "var(--surface-primary)" }}>
      <div className="flex items-center justify-between px-4 py-2 border-b" style={{ borderColor: "var(--border-subtle)", background: "var(--surface-bg)" }}>
        <div className="flex items-center gap-2">
          <Code className="h-3.5 w-3.5 text-[var(--text-muted)]" />
          <span className="text-xs font-medium text-[var(--text-secondary)]">Instance Spec</span>
        </div>
        <Button variant="ghost" size="sm" onClick={handleCopy} className="h-7 gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]">
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
          {copied ? "Copied" : "Copy"}
        </Button>
      </div>
      <div className="p-4 overflow-x-auto">
        <pre className="text-xs leading-relaxed font-mono text-[var(--text-secondary)]" data-testid="spec-content">
          {json}
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
  const { tabs, activeTab: effectiveTab, setActiveTab } = useInstanceTabs(
    instance,
    metadata.eventsCount,
    metadata.externalRefCount,
    metadata.hasSpec,
  );

  return (
    <div className="space-y-0 animate-fade-in">
      {/* ── Instance Details card ── */}
      <div className="rounded-lg border" style={{ borderColor: "var(--border-default)", background: "var(--surface-primary)" }}>
        {/* Title bar + actions */}
        <div className="flex items-center justify-between px-6 py-4" style={{ borderBottom: "1px solid var(--border-subtle)" }}>
          <h2 className="text-sm font-medium text-[var(--text-primary)]">Instance Details</h2>
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
        </div>

        {/* Identity + key metadata — side by side */}
        <InstanceHeaderCard
          instance={instance}
          parentRGD={metadata.parentRGD}
          canReadRGD={permissions.canReadRGD}
          kroState={metadata.kroState}
          onRevisionClick={() => dialogs.setShowRevisionDrawer(true)}
        />

        {/* Source row — git info or direct mode indicator */}
        <InstanceMetadataSection instance={instance} isGitOps={metadata.isGitOps} />
      </div>

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
            {(instance.status || (instance.conditions && instance.conditions.length > 0)) && (
              <InstanceStatusCard
                status={instance.status}
                conditions={instance.conditions}
              />
            )}
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
        {effectiveTab === "events" && (
          <InstanceEvents namespace={instance.namespace} kind={instance.kind} name={instance.name} />
        )}
        {effectiveTab === "external-refs" && (
          <InstanceExternalRefs instance={instance} />
        )}
        {effectiveTab === "children" && (
          <InstanceChildResources namespace={instance.namespace} kind={instance.kind} name={instance.name} />
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
