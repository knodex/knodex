// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { KeyRound } from "lucide-react";
import type { SecretRef } from "@/types/secret";
import { Badge } from "@/components/ui/badge";

interface CatalogSecretsTabProps {
  secretRefs: SecretRef[];
}

export function CatalogSecretsTab({ secretRefs }: CatalogSecretsTabProps) {
  if (secretRefs.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <KeyRound className="h-5 w-5 text-muted-foreground" />
        <div>
          <h3 className="text-sm font-medium text-foreground">Required Secrets</h3>
          <p className="text-xs text-muted-foreground">
            Kubernetes Secrets this RGD references via externalRef
          </p>
        </div>
      </div>

      <div className="space-y-3">
        {secretRefs.map((ref) => (
          <SecretRefCard key={ref.id} secretRef={ref} />
        ))}
      </div>
    </div>
  );
}

function SecretRefCard({ secretRef }: { secretRef: SecretRef }) {
  const displayName = secretRef.externalRefId ?? secretRef.id.replace(/^\d+-/, "");

  return (
    <div
      className="rounded-lg border border-border bg-card p-4"
      data-testid={`catalog-secret-ref-${secretRef.id}`}
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-sm font-medium text-foreground font-mono truncate">
            {displayName}
          </span>
          {secretRef.type === "fixed" && (
            <Badge variant="secondary" className="text-[10px] shrink-0">fixed</Badge>
          )}
          {secretRef.type === "dynamic" && (
            <Badge variant="secondary" className="text-[10px] shrink-0">dynamic</Badge>
          )}
          {secretRef.type === "provided" && (
            <Badge variant="secondary" className="text-[10px] shrink-0">user-provided</Badge>
          )}
        </div>

        {secretRef.description && (
          <p className="text-sm text-muted-foreground mb-3">
            {secretRef.description}
          </p>
        )}

        {secretRef.type === "fixed" && (
          <dl className="space-y-1 text-xs">
            <div className="flex gap-2">
              <dt className="text-muted-foreground w-20 shrink-0">Name</dt>
              <dd className="text-foreground font-mono truncate">{secretRef.name || "—"}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-muted-foreground w-20 shrink-0">Namespace</dt>
              <dd className="text-foreground font-mono truncate">{secretRef.namespace || "—"}</dd>
            </div>
          </dl>
        )}

        {secretRef.type === "dynamic" && (
          <dl className="space-y-1 text-xs">
            {secretRef.nameExpr && (
              <div className="flex gap-2">
                <dt className="text-muted-foreground w-20 shrink-0">Name</dt>
                <dd className="text-foreground font-mono truncate bg-secondary px-1.5 py-0.5 rounded">
                  {secretRef.nameExpr}
                </dd>
              </div>
            )}
            {secretRef.namespaceExpr && (
              <div className="flex gap-2">
                <dt className="text-muted-foreground w-20 shrink-0">Namespace</dt>
                <dd className="text-foreground font-mono truncate bg-secondary px-1.5 py-0.5 rounded">
                  {secretRef.namespaceExpr}
                </dd>
              </div>
            )}
          </dl>
        )}
      </div>
    </div>
  );
}
