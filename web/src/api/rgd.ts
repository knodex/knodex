// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type {
  CatalogRGD,
  RGDListResponse,
  RGDListParams,
  RGDFilterOptions,
  SchemaResponse,
  Instance,
  InstanceListParams,
  InstanceListResponse,
  CreateInstanceRequest,
  CreateInstanceResponse,
  UpdateInstanceRequest,
  UpdateInstanceResponse,
  ResourceGraph,
  PendingInstancesResponse,
  StuckInstancesResponse,
  CountResponse,
  GraphRevision,
  GraphRevisionList,
  RevisionDiff,
  ChildResourceResponse,
} from "@/types/rgd";
import { hasValidNamespace } from "@/types/rgd";
import { createLogger } from "@/lib/logger";

const log = createLogger("[API:Instance]");

/**
 * Validate instance scope invariant and log violations.
 * Non-blocking: invalid instances are logged but never filtered or rejected.
 */
function validateInstanceNamespace(instance: Pick<Instance, "name" | "namespace" | "isClusterScoped">): void {
  if (!hasValidNamespace(instance)) {
    const detail = `isClusterScoped=${instance.isClusterScoped}, namespace="${instance.namespace}"`;
    if (import.meta.env.DEV) {
      log.error(`Invalid scope state for instance "${instance.name}": ${detail}`);
    } else {
      log.warn(`Invalid scope state for instance "${instance.name}": ${detail}`);
    }
  }
}

/**
 * List RGDs with optional filters and pagination
 */
export async function listRGDs(params?: RGDListParams): Promise<RGDListResponse> {
  const queryParams: Record<string, string> = {};

  if (params?.namespace) {
    queryParams.namespace = params.namespace;
  }
  if (params?.category) {
    queryParams.category = params.category;
  }
  if (params?.tags && params.tags.length > 0) {
    queryParams.tags = params.tags.join(",");
  }
  if (params?.extendsKind) {
    queryParams.extendsKind = params.extendsKind;
  }
  if (params?.search) {
    queryParams.search = params.search;
  }
  if (params?.dependsOnKind) {
    queryParams.dependsOnKind = params.dependsOnKind;
  }
  if (params?.producesKind) {
    queryParams.producesKind = params.producesKind;
  }
  if (params?.producesGroup) {
    queryParams.producesGroup = params.producesGroup;
  }
  if (params?.status) {
    queryParams.status = params.status;
  }
  if (params?.page) {
    queryParams.page = String(params.page);
  }
  if (params?.pageSize) {
    queryParams.pageSize = String(params.pageSize);
  }
  if (params?.sortBy) {
    queryParams.sortBy = params.sortBy;
  }
  if (params?.sortOrder) {
    queryParams.sortOrder = params.sortOrder;
  }

  const response = await apiClient.get<RGDListResponse>("/v1/rgds", {
    params: queryParams,
  });
  return response.data;
}

/**
 * Get a single RGD by name
 */
export async function getRGD(name: string, namespace?: string): Promise<CatalogRGD> {
  const params = namespace ? { namespace } : {};
  const response = await apiClient.get<CatalogRGD>(`/v1/rgds/${encodeURIComponent(name)}`, {
    params,
  });
  return response.data;
}

/**
 * Get the CRD schema for an RGD
 */
export async function getRGDSchema(name: string, namespace?: string): Promise<SchemaResponse> {
  const params = namespace ? { namespace } : {};
  const response = await apiClient.get<SchemaResponse>(
    `/v1/rgds/${encodeURIComponent(name)}/schema`,
    { params }
  );
  return response.data;
}

/**
 * Get the internal resource graph for an RGD
 * Shows K8s resources (templates and externalRefs) within a single RGD
 */
export async function getRGDResourceGraph(name: string, namespace?: string): Promise<ResourceGraph> {
  const params = namespace ? { namespace } : {};
  const response = await apiClient.get<ResourceGraph>(
    `/v1/rgds/${encodeURIComponent(name)}/resources`,
    { params }
  );
  return response.data;
}

/**
 * Get the definition graph for an RGD
 * Uses the new /graph endpoint which includes collection metadata (isCollection, forEach, readyWhen)
 */
export async function getRGDDefinitionGraph(name: string, namespace?: string): Promise<ResourceGraph> {
  const params = namespace ? { namespace } : {};
  const response = await apiClient.get<ResourceGraph>(
    `/v1/rgds/${encodeURIComponent(name)}/graph`,
    { params }
  );
  return response.data;
}

/**
 * List all instances with optional filters and pagination
 */
export async function listInstances(params?: InstanceListParams): Promise<InstanceListResponse> {
  const queryParams: Record<string, string> = {};

  if (params?.namespace) {
    queryParams.namespace = params.namespace;
  }
  if (params?.rgdName) {
    queryParams.rgdName = params.rgdName;
  }
  if (params?.rgdNamespace) {
    queryParams.rgdNamespace = params.rgdNamespace;
  }
  if (params?.health) {
    queryParams.health = params.health;
  }
  if (params?.search) {
    queryParams.search = params.search;
  }
  if (params?.page) {
    queryParams.page = String(params.page);
  }
  if (params?.pageSize) {
    queryParams.pageSize = String(params.pageSize);
  }
  if (params?.sortBy) {
    queryParams.sortBy = params.sortBy;
  }
  if (params?.sortOrder) {
    queryParams.sortOrder = params.sortOrder;
  }

  const response = await apiClient.get<InstanceListResponse>("/v1/instances", {
    params: queryParams,
  });
  response.data.items.forEach(validateInstanceNamespace);
  return response.data;
}

/**
 * Build the K8s-aligned URL path for an instance.
 * Namespaced: /v1/namespaces/{ns}/instances/{kind}/{name}
 * Cluster-scoped: /v1/instances/{kind}/{name}
 */
export function instancePath(namespace: string, kind: string, name: string): string {
  const encodedKind = encodeURIComponent(kind);
  const encodedName = encodeURIComponent(name);
  if (namespace) {
    return `/v1/namespaces/${encodeURIComponent(namespace)}/instances/${encodedKind}/${encodedName}`;
  }
  return `/v1/instances/${encodedKind}/${encodedName}`;
}

/**
 * Get a single instance by namespace, kind, and name
 */
export async function getInstance(namespace: string, kind: string, name: string): Promise<Instance> {
  const response = await apiClient.get<Instance>(instancePath(namespace, kind, name));
  validateInstanceNamespace(response.data);
  return response.data;
}

/**
 * Delete an instance by namespace, kind, and name
 */
export async function deleteInstance(namespace: string, kind: string, name: string): Promise<void> {
  await apiClient.delete(instancePath(namespace, kind, name));
}

/**
 * Update the spec of an existing instance
 */
export async function updateInstanceSpec(
  namespace: string,
  kind: string,
  name: string,
  request: UpdateInstanceRequest
): Promise<UpdateInstanceResponse> {
  const response = await apiClient.patch<UpdateInstanceResponse>(
    instancePath(namespace, kind, name),
    request
  );
  return response.data;
}

/**
 * List instances of a specific RGD
 */
export async function listRGDInstances(
  rgdName: string,
  rgdNamespace?: string
): Promise<InstanceListResponse> {
  return listInstances({
    rgdName,
    rgdNamespace,
  });
}

/**
 * Create a new instance of an RGD.
 * Uses K8s-aligned path: /v1/namespaces/{ns}/instances/{kind} or /v1/instances/{kind}
 */
export async function createInstance(
  kind: string,
  request: CreateInstanceRequest
): Promise<CreateInstanceResponse> {
  const encodedKind = encodeURIComponent(kind);
  const url = request.namespace
    ? `/v1/namespaces/${encodeURIComponent(request.namespace)}/instances/${encodedKind}`
    : `/v1/instances/${encodedKind}`;
  const response = await apiClient.post<CreateInstanceResponse>(url, request);
  return response.data;
}

export interface PreflightResponse {
  valid: boolean;
  message?: string;
}

/**
 * Run a Kubernetes server-side dry-run for an instance without creating it.
 * Catches admission webhook violations (e.g. Gatekeeper) on the KRO instance resource.
 */
export async function preflightInstance(
  kind: string,
  request: CreateInstanceRequest
): Promise<PreflightResponse> {
  const encodedKind = encodeURIComponent(kind);
  const url = request.namespace
    ? `/v1/namespaces/${encodeURIComponent(request.namespace)}/instances/${encodedKind}/preflight`
    : `/v1/instances/${encodedKind}/preflight`;
  const response = await apiClient.post<PreflightResponse>(url, request);
  return response.data;
}

/**
 * Get all pending GitOps instances (pushed but not yet synced)
 */
export async function getPendingInstances(): Promise<PendingInstancesResponse> {
  const response = await apiClient.get<PendingInstancesResponse>(
    "/v1/instances/pending"
  );
  return response.data;
}

/**
 * Get all stuck GitOps instances (in pending state for too long)
 */
export async function getStuckInstances(): Promise<StuckInstancesResponse> {
  const response = await apiClient.get<StuckInstancesResponse>(
    "/v1/instances/stuck"
  );
  return response.data;
}

/**
 * Get total count of RGDs accessible to the user
 * Lightweight endpoint for displaying badge counts without fetching full list
 */
export async function getRGDCount(): Promise<CountResponse> {
  const response = await apiClient.get<CountResponse>("/v1/rgds/count");
  return response.data;
}

/**
 * Get total count of instances accessible to the user
 * Lightweight endpoint for displaying badge counts without fetching full list
 */
export async function getInstanceCount(): Promise<CountResponse> {
  const response = await apiClient.get<CountResponse>("/v1/instances/count");
  return response.data;
}

/**
 * Get a single GraphRevision by RGD name and revision number (includes snapshot)
 */
export async function getRGDRevision(name: string, revision: number): Promise<GraphRevision> {
  const response = await apiClient.get<GraphRevision>(
    `/v1/rgds/${encodeURIComponent(name)}/revisions/${revision}`,
  );
  return response.data;
}

/**
 * Get GraphRevision history for an RGD
 */
export async function getRGDRevisions(name: string): Promise<GraphRevisionList> {
  const response = await apiClient.get<GraphRevisionList>(
    `/v1/rgds/${encodeURIComponent(name)}/revisions`
  );
  return response.data;
}

/**
 * Get available filter options (projects, tags, categories) for the catalog.
 * Categories are filtered by the user's Casbin authorization — only authorized
 * categories are returned.
 */
export async function getRGDFilters(): Promise<RGDFilterOptions> {
  const response = await apiClient.get<RGDFilterOptions>("/v1/rgds/filters");
  return response.data;
}

/**
 * Get the structured diff between two revisions of an RGD
 */
export async function getRGDRevisionDiff(
  name: string,
  rev1: number,
  rev2: number,
): Promise<RevisionDiff> {
  const response = await apiClient.get<RevisionDiff>(
    `/v1/rgds/${encodeURIComponent(name)}/revisions/${rev1}/diff/${rev2}`,
  );
  return response.data;
}

/**
 * Get child resources for an instance, grouped by node-id.
 * Uses KRO labels to discover all resources created by the instance.
 */
export async function getInstanceChildren(
  namespace: string,
  kind: string,
  name: string,
): Promise<ChildResourceResponse> {
  const response = await apiClient.get<ChildResourceResponse>(
    `${instancePath(namespace, kind, name)}/children`,
  );
  return response.data;
}
