// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type {
  CatalogRGD,
  RGDListResponse,
  RGDListParams,
  SchemaResponse,
  Instance,
  InstanceListParams,
  InstanceListResponse,
  CreateInstanceRequest,
  CreateInstanceResponse,
  UpdateInstanceRequest,
  UpdateInstanceResponse,
  ResourceGraph,
  StatusTimelineResponse,
  PendingInstancesResponse,
  StuckInstancesResponse,
  CountResponse,
} from "@/types/rgd";

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
  return response.data;
}

/**
 * Get a single instance by namespace, kind, and name
 */
export async function getInstance(namespace: string, kind: string, name: string): Promise<Instance> {
  const response = await apiClient.get<Instance>(
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * Delete an instance by namespace, kind, and name
 */
export async function deleteInstance(namespace: string, kind: string, name: string): Promise<void> {
  await apiClient.delete(
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`
  );
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
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`,
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
 * Create a new instance of an RGD
 */
export async function createInstance(
  request: CreateInstanceRequest
): Promise<CreateInstanceResponse> {
  const response = await apiClient.post<CreateInstanceResponse>(
    "/v1/instances",
    request
  );
  return response.data;
}

/**
 * Get status timeline for an instance by instance ID
 * Shows full deployment progression: PushedToGit → WaitingForSync → Creating → Ready
 */
export async function getInstanceStatusTimeline(
  instanceId: string
): Promise<StatusTimelineResponse> {
  const response = await apiClient.get<StatusTimelineResponse>(
    `/v1/instances/timeline/${encodeURIComponent(instanceId)}`
  );
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
