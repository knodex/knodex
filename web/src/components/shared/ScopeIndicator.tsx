// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Globe, Box } from "@/lib/icons";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { hasValidNamespace } from "@/types/rgd";

export type ScopeVariant = "badge" | "inline" | "compact" | "text";

interface ScopeIndicatorProps {
  isClusterScoped?: boolean;
  namespace?: string;
  variant?: ScopeVariant;
}

/**
 * Shared scope indicator component.
 *
 * Validates the namespace/isClusterScoped invariant at the rendering boundary
 * and warns in development when an invalid combination is detected.
 *
 * Variants:
 * - `badge` (default): Globe icon + "Cluster-Scoped" text or namespace (for cards/lists)
 * - `inline`: Badge with background (for detail views)
 * - `compact`: Globe icon only with tooltip (for tight spaces like RGDCard header)
 * - `text`: Plain text "Cluster" or "Namespaced" (for filters/overview details)
 */
export function ScopeIndicator({
  isClusterScoped,
  namespace,
  variant = "badge",
}: ScopeIndicatorProps) {
  if (import.meta.env.DEV && namespace !== undefined) {
    if (!hasValidNamespace({ isClusterScoped, namespace })) {
      console.warn(
        `[ScopeIndicator] Invalid scope state: isClusterScoped=${String(isClusterScoped)}, namespace="${namespace}". ` +
        "Cluster-scoped instances must have empty namespace; namespaced instances must have non-empty namespace."
      );
    }
  }

  if (!isClusterScoped) {
    return <NamespacedDisplay namespace={namespace} variant={variant} />;
  }

  switch (variant) {
    case "compact":
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium text-violet-600 dark:text-violet-400 bg-violet-500/10">
              <Globe className="h-3 w-3" role="img" aria-label="Cluster-scoped resource" />
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>Cluster-Scoped Resource</p>
          </TooltipContent>
        </Tooltip>
      );

    case "inline":
      return (
        <span className="inline-flex items-center gap-1.5 text-sm font-medium text-violet-600 dark:text-violet-400">
          <Globe className="h-3.5 w-3.5" role="img" aria-label="Cluster-scoped resource" />
          Cluster-Scoped
        </span>
      );

    case "text":
      return (
        <span className="text-violet-600 dark:text-violet-400 font-medium">
          Cluster-Scoped
        </span>
      );

    case "badge":
    default:
      return (
        <span className="inline-flex items-center gap-1 text-xs font-medium text-violet-600 dark:text-violet-400">
          <Globe className="h-3 w-3" role="img" aria-label="Cluster-scoped resource" />
          Cluster-Scoped
        </span>
      );
  }
}

function NamespacedDisplay({
  namespace,
  variant,
}: {
  namespace?: string;
  variant: ScopeVariant;
}) {
  if (variant === "text") {
    return <span className="text-foreground font-medium">Namespaced</span>;
  }

  if (variant === "inline") {
    return (
      <span className="inline-flex items-center gap-1 text-sm text-muted-foreground">
        <Box className="h-3.5 w-3.5" />
        <span className="font-mono">{namespace || "—"}</span>
      </span>
    );
  }

  // badge and compact: plain namespace text
  return (
    <span className="text-xs text-muted-foreground font-mono truncate">
      {namespace || "—"}
    </span>
  );
}
