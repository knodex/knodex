// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Build a frontend route URL for an instance detail page.
 *
 * Routes mirror Kubernetes API ordering — group first, then optional namespace.
 *   Namespaced:     /instances/group/{group}/ns/{namespace}/{kind}/{name}
 *   Cluster-scoped: /instances/group/{group}/cluster/{kind}/{name}
 *
 * Group is derived from apiVersion (`{group}/{version}`). Knodex only indexes
 * Kro-spawned CRDs which always declare a non-empty apiGroup, and the backend
 * rejects empty group at the route level (`IsValidAPIGroup`). Producing a URL
 * with an empty group segment would route to a 400, so this function throws
 * instead of silently emitting a broken link.
 */
export function buildInstanceRoute(args: {
  apiVersion: string;
  namespace?: string;
  kind: string;
  name: string;
}): string {
  const group = apiGroupOf(args.apiVersion);
  if (!group) {
    throw new Error(
      `buildInstanceRoute: apiVersion ${JSON.stringify(args.apiVersion)} has no apiGroup; ` +
        `Knodex only supports CRDs with a non-empty group.`,
    );
  }
  const g = encodeURIComponent(group);
  const k = encodeURIComponent(args.kind);
  const n = encodeURIComponent(args.name);
  if (args.namespace) {
    const ns = encodeURIComponent(args.namespace);
    return `/instances/group/${g}/ns/${ns}/${k}/${n}`;
  }
  return `/instances/group/${g}/cluster/${k}/${n}`;
}

/**
 * Extract the K8s API group from an apiVersion string.
 *   "apps.example.com/v1" -> "apps.example.com"
 *   "v1"                  -> ""        (core group)
 */
export function apiGroupOf(apiVersion: string): string {
  const slash = apiVersion.indexOf("/");
  return slash === -1 ? "" : apiVersion.slice(0, slash);
}
