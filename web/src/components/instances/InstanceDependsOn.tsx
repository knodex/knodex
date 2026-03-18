// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { Link } from "react-router-dom";
import { Link2, ArrowRight } from "lucide-react";
import { useRGDResourceGraph } from "@/hooks/useRGDs";
import { useInstanceList } from "@/hooks/useInstances";
import type { Instance } from "@/types/rgd";
import { InstanceMiniCard } from "@/components/shared/InstanceMiniCard";

/** Page size for the batched instance query used to resolve dependency instances.
 * Must be large enough to contain all candidate instances in the target namespace(s).
 * When refs span multiple namespaces the query is unscoped, so this must cover
 * the full cluster instance count. 500 balances payload size vs truncation risk. */
const DEPENDENCY_RESOLUTION_PAGE_SIZE = 500;

interface InstanceDependsOnProps {
  instance: Instance;
}

interface ExternalRefEntry {
  id: string;
  name: string;
  namespace: string;
  kind?: string;
}

export function InstanceDependsOn({ instance }: InstanceDependsOnProps) {
  // Extract externalRef entries from instance spec
  const externalRefs = useMemo(() => {
    const refs: ExternalRefEntry[] = [];
    const extRefObj = (instance.spec?.externalRef as Record<string, unknown>) ?? {};

    for (const [id, value] of Object.entries(extRefObj)) {
      if (value && typeof value === "object") {
        const ref = value as Record<string, unknown>;
        const name = typeof ref.name === "string" ? ref.name : "";
        const namespace = typeof ref.namespace === "string" ? ref.namespace : "";
        if (name) {
          refs.push({ id, name, namespace });
        }
      }
    }
    return refs;
  }, [instance.spec]);

  // Fetch the RGD resource graph to resolve externalRef IDs to Kinds
  const { data: resourceGraph } = useRGDResourceGraph(
    instance.rgdName,
    instance.rgdNamespace
  );

  // Map externalRef IDs to their Kinds using the resource graph
  const refsWithKinds = useMemo(() => {
    if (!resourceGraph) return externalRefs;

    return externalRefs.map((ref) => {
      const extRefs = resourceGraph.resources.filter(
        (r) => r.externalRef && !r.isTemplate
      );
      // Match by externalRef ID as an exact path segment to avoid substring collisions
      // (e.g., ref id "db" must not match "mongodb" in the path)
      const matchingResource = extRefs.find((r) => {
        const schemaField = r.externalRef?.schemaField || "";
        const segments = schemaField.split(".");
        const refIdx = segments.indexOf("externalRef");
        return refIdx !== -1 && segments[refIdx + 1] === ref.id;
      });

      return {
        ...ref,
        kind: matchingResource?.kind || undefined,
      };
    });
  }, [externalRefs, resourceGraph]);

  // Collect unique namespaces for a single batched query instead of N+1 per-card queries.
  // We search by instance namespace (the primary scope) and use a large page size
  // to capture all candidate instances for client-side exact matching.
  const primaryNamespace = useMemo(() => {
    const namespaces = new Set(externalRefs.map((r) => r.namespace).filter(Boolean));
    // If all refs share one namespace, scope the query; otherwise fetch broadly
    if (namespaces.size === 1) return [...namespaces][0];
    return undefined;
  }, [externalRefs]);

  const { data: instanceData, isLoading: instancesLoading } = useInstanceList(
    externalRefs.length > 0
      ? { namespace: primaryNamespace, pageSize: DEPENDENCY_RESOLUTION_PAGE_SIZE }
      : undefined
  );

  // Build a lookup for resolved dependency instances
  const resolvedInstances = useMemo(() => {
    const map = new Map<string, Instance>();
    if (!instanceData?.items) return map;

    for (const ref of refsWithKinds) {
      const match = instanceData.items.find(
        (i) =>
          i.name === ref.name &&
          (!ref.namespace || i.namespace === ref.namespace) &&
          (!ref.kind || i.kind === ref.kind)
      );
      if (match) {
        map.set(ref.id, match);
      }
    }
    return map;
  }, [instanceData, refsWithKinds]);

  if (externalRefs.length === 0) {
    return null;
  }

  return (
    <div className="rounded-lg border border-border bg-card overflow-hidden">
      <div className="px-4 py-3 flex items-center gap-2 border-b border-border">
        <Link2 className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">
          Depends On ({externalRefs.length})
        </h3>
      </div>
      <div className="p-4 grid gap-3 sm:grid-cols-2">
        {refsWithKinds.map((ref) => {
          const depInstance = resolvedInstances.get(ref.id);
          return (
            <InstanceMiniCard
              key={ref.id}
              instance={depInstance}
              isLoading={instancesLoading}
              notFound={!instancesLoading && !depInstance}
              label={ref.name}
              kindLabel={ref.kind}
              namespaceLabel={ref.namespace}
              action={
                depInstance ? (
                  <Link
                    to={`/instances/${encodeURIComponent(depInstance.namespace)}/${encodeURIComponent(depInstance.kind)}/${encodeURIComponent(depInstance.name)}`}
                    className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-primary transition-colors"
                  >
                    View instance
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Link>
                ) : null
              }
            />
          );
        })}
      </div>
    </div>
  );
}
