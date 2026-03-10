// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { AlertTriangle, ChevronDown, ChevronUp, RefreshCw } from "lucide-react";
import type { Instance } from "@/types/rgd";

interface GitOpsDriftBannerProps {
  instance: Instance;
}

export function GitOpsDriftBanner({ instance }: GitOpsDriftBannerProps) {
  const [showDesiredSpec, setShowDesiredSpec] = useState(false);

  const deploymentMode = instance.deploymentMode || instance.labels?.["knodex.io/deployment-mode"];
  if (!instance.gitopsDrift || !instance.desiredSpec || (deploymentMode !== "gitops" && deploymentMode !== "hybrid")) {
    return null;
  }

  return (
    <div className="rounded-lg border border-status-warning bg-status-warning/10 p-4 space-y-3">
      <div className="flex items-start gap-3">
        <RefreshCw className="h-5 w-5 text-status-warning shrink-0 mt-0.5 animate-spin" style={{ animationDuration: "3s" }} />
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 text-status-warning" />
            <h4 className="text-sm font-medium text-status-warning">
              GitOps Drift Detected
            </h4>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Waiting for ArgoCD/Flux to reconcile — the live instance has not yet been updated
            to match the desired spec pushed to Git.
          </p>
        </div>
      </div>

      {/* Toggle for desired spec comparison */}
      <button
        onClick={() => setShowDesiredSpec(!showDesiredSpec)}
        className="flex items-center gap-1.5 text-xs text-status-warning hover:text-status-warning/80 transition-colors pl-8"
      >
        {showDesiredSpec ? (
          <ChevronUp className="h-3.5 w-3.5" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5" />
        )}
        {showDesiredSpec ? "Hide desired spec (Git)" : "Show desired spec (Git)"}
      </button>

      {showDesiredSpec && (
        <div className="pl-8 space-y-2">
          <p className="text-xs text-muted-foreground">
            This is the spec that was pushed to Git. The live cluster spec (shown above) will
            match once the GitOps tool syncs the change.
          </p>
          <pre className="text-xs font-mono text-muted-foreground overflow-x-auto rounded border border-border bg-secondary/50 p-3">
            {JSON.stringify(instance.desiredSpec, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}
