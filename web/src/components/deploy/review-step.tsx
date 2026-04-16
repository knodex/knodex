// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Pencil, ShieldAlert } from "@/lib/icons";
import type { ComplianceValidateViolation } from "@/api/compliance";
import { ComplianceBanner } from "./compliance-banner";
import { orderEntries } from "@/lib/order-properties";

interface ReviewStepProps {
  project: string;
  namespace: string;
  instanceName?: string;
  formValues: Record<string, unknown>;
  isClusterScoped?: boolean;
  complianceResult?: "pass" | "warning" | "block";
  complianceViolations?: ComplianceValidateViolation[];
  onAcknowledgeWarnings?: () => void;
  onEditStep?: (stepIndex: number) => void;
  /** Selected target cluster for multi-cluster deployments */
  clusterRef?: string;
  /** Display order for configuration values */
  propertyOrder?: string[];
  /** Preflight dry-run result — blocks deploy if true */
  preflightBlocked?: boolean;
  preflightMessage?: string;
}

export function ReviewStep({
  project,
  namespace,
  instanceName,
  formValues,
  isClusterScoped,
  complianceResult,
  complianceViolations,
  onAcknowledgeWarnings,
  onEditStep,
  clusterRef,
  propertyOrder,
  preflightBlocked,
  preflightMessage,
}: ReviewStepProps) {
  return (
    <div className="space-y-4" data-testid="review-step">
      <h3 className="text-sm font-medium text-[var(--text-primary)]">
        Deployment Summary
      </h3>

      {/* Project & Namespace */}
      <div className="space-y-2">
        {instanceName && (
          <div className="flex items-center justify-between rounded-md px-3 py-2 bg-white/[0.03]">
            <div>
              <span className="text-xs text-[var(--text-muted)]">Instance Name</span>
              <p className="text-sm font-mono text-[var(--text-primary)]">{instanceName}</p>
            </div>
            {onEditStep && (
              <button type="button" onClick={() => onEditStep(0)} className="p-1 text-[var(--text-muted)]" aria-label="Edit instance name">
                <Pencil className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}
        <div className="flex items-center justify-between rounded-md px-3 py-2 bg-white/[0.03]">
          <div>
            <span className="text-xs text-[var(--text-muted)]">Project</span>
            <p className="text-sm text-[var(--text-primary)]">{project || "—"}</p>
          </div>
          {onEditStep && (
            <button type="button" onClick={() => onEditStep(0)} className="p-1 text-[var(--text-muted)]" aria-label="Edit project">
              <Pencil className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {clusterRef && (
          <div className="flex items-center justify-between rounded-md px-3 py-2 bg-white/[0.03]">
            <div>
              <span className="text-xs text-[var(--text-muted)]">Cluster</span>
              <p className="text-sm text-[var(--text-primary)]">{clusterRef}</p>
            </div>
            {onEditStep && (
              <button type="button" onClick={() => onEditStep(0)} className="p-1 text-[var(--text-muted)]" aria-label="Edit cluster">
                <Pencil className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}

        {!isClusterScoped && (
          <div className="flex items-center justify-between rounded-md px-3 py-2 bg-white/[0.03]">
            <div>
              <span className="text-xs text-[var(--text-muted)]">Namespace</span>
              <p className="text-sm text-[var(--text-primary)]">{namespace || "—"}</p>
            </div>
            {onEditStep && (
              <button type="button" onClick={() => onEditStep(0)} className="p-1 text-[var(--text-muted)]" aria-label="Edit namespace">
                <Pencil className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}
      </div>

      {/* Configuration values */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-[var(--text-muted)]">Configuration</span>
          {onEditStep && (
            <button type="button" onClick={() => onEditStep(1)} className="text-xs flex items-center gap-1 text-[var(--brand-primary)]" aria-label="Edit configuration">
              <Pencil className="h-3 w-3" />
              Edit
            </button>
          )}
        </div>
        <div className="rounded-md p-3 space-y-1 bg-white/[0.03]">
          {Object.entries(formValues).length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">No configuration values</p>
          ) : (
            orderEntries(Object.entries(formValues), propertyOrder).map(([key, value]) => (
              <div key={key} className="flex items-center justify-between text-sm">
                <span className="text-[var(--text-secondary)]">{key}</span>
                <span className="font-mono text-xs text-[var(--text-primary)]">
                  {typeof value === "object" ? JSON.stringify(value) : String(value ?? "")}
                </span>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Preflight dry-run block */}
      {preflightBlocked && preflightMessage && (
        <div
          data-testid="preflight-alert"
          role="alert"
          className="rounded-lg p-4 border flex items-start gap-2"
          style={{ backgroundColor: "rgba(244,63,94,0.1)", borderColor: "rgba(244,63,94,0.5)" }}
        >
          <ShieldAlert className="h-5 w-5 shrink-0 mt-0.5" style={{ color: "#f43f5e" }} />
          <div>
            <p className="font-medium text-sm" style={{ color: "#f43f5e" }}>
              Deployment blocked by admission policy
            </p>
            <p className="text-sm mt-1" style={{ color: "#fda4af" }}>
              {preflightMessage}
            </p>
          </div>
        </div>
      )}

      {/* Compliance warnings/blocks */}
      {complianceResult && complianceResult !== "pass" && complianceViolations && complianceViolations.length > 0 && (
        <ComplianceBanner
          violations={complianceViolations}
          result={complianceResult}
          onAcknowledge={onAcknowledgeWarnings}
        />
      )}
    </div>
  );
}
