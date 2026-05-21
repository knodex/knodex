// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useFormContext } from "react-hook-form";
import { Loader2, Pencil, ShieldAlert } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { orderEntries } from "@/lib/order-properties";
import type { DeployTab } from "@/lib/build-tabs";
import type { ComplianceValidateViolation } from "@/api/compliance";
import { ComplianceBanner } from "@/components/deploy/compliance-banner";

interface ReviewTabProps {
  tabs: DeployTab[];
  onEditTab: (id: string) => void;
  complianceResult: "pass" | "warning" | "block";
  complianceViolations: ComplianceValidateViolation[];
  warningsAcknowledged: boolean;
  setWarningsAcknowledged: (v: boolean) => void;
  preflightValid: boolean;
  preflightMessage?: string;
  isValidating: boolean;
  isPreflighting: boolean;
  isClusterScoped: boolean;
}

function renderValue(value: unknown): string {
  if (value === undefined || value === null) return "—";
  if (typeof value === "object") return JSON.stringify(value);
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
}

export function ReviewTab({
  tabs,
  onEditTab,
  complianceResult,
  complianceViolations,
  warningsAcknowledged,
  setWarningsAcknowledged,
  preflightValid,
  preflightMessage,
  isValidating,
  isPreflighting,
  isClusterScoped,
}: ReviewTabProps) {
  const { watch } = useFormContext();
  const values = watch() as Record<string, unknown>;

  const basicsRows = useMemo(() => {
    const rows: Array<[string, unknown]> = [
      ["Instance Name", values.instanceName],
      ["Project", values.project],
    ];
    if (!isClusterScoped) rows.push(["Namespace", values.namespace]);
    rows.push(["Deployment Mode", values.deploymentMode]);
    if (values.deploymentMode === "gitops" || values.deploymentMode === "hybrid") {
      rows.push(["Repository", values.repositoryId]);
      rows.push(["Branch", values.gitBranch]);
      rows.push(["Path", values.gitPath]);
    }
    return rows;
  }, [values, isClusterScoped]);

  const schemaCards = useMemo(
    () =>
      tabs.filter((t) => t.kind === "general" || t.kind === "schema"),
    [tabs]
  );

  return (
    <div className="space-y-4" data-testid="review-tab">
      {/* Basics card */}
      <SectionCard
        title="Basics"
        onEdit={() => onEditTab("basics")}
        testid="review-card-basics"
      >
        {basicsRows.map(([label, value]) => (
          <div
            key={label}
            className="flex items-center justify-between text-sm"
          >
            <span className="text-[var(--text-secondary)]">{label}</span>
            <span className="font-mono text-xs text-[var(--text-primary)]">
              {renderValue(value)}
            </span>
          </div>
        ))}
      </SectionCard>

      {/* Schema-driven cards */}
      {schemaCards.map((tab) => {
        const tabValues: Record<string, unknown> =
          tab.kind === "general"
            ? Object.fromEntries(
                Object.keys(tab.properties ?? {}).map((k) => [k, values[k]])
              )
            : (values[tab.id] as Record<string, unknown>) ?? {};
        const entries = orderEntries(
          Object.entries(tabValues),
          tab.propertyOrder
        );
        return (
          <SectionCard
            key={tab.id}
            title={tab.label}
            onEdit={() => onEditTab(tab.id)}
            testid={`review-card-${tab.id}`}
          >
            {entries.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">
                No values configured
              </p>
            ) : (
              entries.map(([key, value]) => (
                <div
                  key={key}
                  className="flex items-center justify-between text-sm"
                >
                  <span className="text-[var(--text-secondary)]">{key}</span>
                  <span className="font-mono text-xs text-[var(--text-primary)]">
                    {renderValue(value)}
                  </span>
                </div>
              ))
            )}
          </SectionCard>
        );
      })}

      {/* Validation status — spinner while running */}
      {(isValidating || isPreflighting) && (
        <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
          <Loader2 className="h-4 w-4 animate-spin" />
          {isValidating && isPreflighting
            ? "Validating compliance and admission policies…"
            : isValidating
              ? "Validating compliance…"
              : "Running admission preflight…"}
        </div>
      )}

      {/* Preflight error banner */}
      {!isPreflighting && !preflightValid && preflightMessage && (
        <PreflightAlert message={preflightMessage} />
      )}

      {/* Compliance banner */}
      <ComplianceBanner
        result={complianceResult}
        violations={complianceViolations}
        acknowledged={warningsAcknowledged}
        onAcknowledgedChange={setWarningsAcknowledged}
      />
    </div>
  );
}

function PreflightAlert({ message }: { message: string }) {
  // Distinguish real admission policy blocks from infrastructure/availability errors.
  const isAdmissionBlock =
    message.startsWith("Blocked by") ||
    message.includes("admission webhook") ||
    message.includes("Gatekeeper policy");

  const heading = isAdmissionBlock
    ? "Deployment blocked by admission policy"
    : "Preflight check failed";

  const hint = !isAdmissionBlock
    ? "This is an infrastructure or cluster availability issue, not a policy violation."
    : undefined;

  return (
    <div
      data-testid="preflight-alert"
      role="alert"
      className="rounded-lg p-4 border flex items-start gap-2"
      style={{
        backgroundColor: "rgba(244,63,94,0.1)",
        borderColor: "rgba(244,63,94,0.5)",
      }}
    >
      <ShieldAlert
        className="h-5 w-5 shrink-0 mt-0.5"
        style={{ color: "#f43f5e" }}
      />
      <div className="space-y-1">
        <p className="font-medium text-sm" style={{ color: "#f43f5e" }}>
          {heading}
        </p>
        <p className="text-sm" style={{ color: "#fda4af" }}>
          {message}
        </p>
        {hint && (
          <p className="text-xs" style={{ color: "#fda4af", opacity: 0.75 }}>
            {hint}
          </p>
        )}
      </div>
    </div>
  );
}

interface SectionCardProps {
  title: string;
  onEdit: () => void;
  testid: string;
  children: React.ReactNode;
}

function SectionCard({ title, onEdit, testid, children }: SectionCardProps) {
  return (
    <div
      data-testid={testid}
      className="rounded-md border p-4"
      style={{
        backgroundColor: "rgba(255,255,255,0.03)",
        borderColor: "rgba(255,255,255,0.06)",
      }}
    >
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-sm font-medium text-[var(--text-primary)]">
          {title}
        </h4>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onEdit}
          aria-label={`Edit ${title}`}
          data-testid={`${testid}-edit`}
        >
          <Pencil className="h-3.5 w-3.5" />
          Edit
        </Button>
      </div>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}
