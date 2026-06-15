// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type { RGDListResponse } from "@/types/rgd";

/** kagent presence status (Story 49.1). */
export type AgentsStatus = "ready" | "not_installed" | "degraded";

/**
 * Response from GET /api/v1/agents/status.
 * Null check fields mean the check was short-circuited or indeterminate.
 */
export interface AgentsStatusResponse {
  status: AgentsStatus;
  crdPresent: boolean | null;
  controllerHealthy: boolean | null;
  message: string;
}

/**
 * Get kagent presence status for the Agents workspace.
 * The server always responds 200 — degraded is a payload, never a 5xx.
 */
export async function getAgentsStatus(): Promise<AgentsStatusResponse> {
  const response = await apiClient.get<AgentsStatusResponse>("/v1/agents/status");
  return response.data;
}

/**
 * Resolved AI model identity for an agent (Story 50.4). Provider + model name
 * only — never an API key, endpoint, or secret reference (NFR-A4/A7). Absent
 * when the model cannot be resolved.
 */
export interface AgentModel {
  provider: string;
  name: string;
}

/** A deployed kagent Agent CR visible to the caller (Story 49.2). */
export interface InstalledAgent {
  name: string;
  namespace: string;
  /** Agent spec.description; empty string when absent. */
  description: string;
  /** RFC3339 creation timestamp; empty string when unknown. */
  createdAt: string;
  /** Resolved AI model (Story 50.4); omitted when unresolvable. */
  model?: AgentModel;
  /**
   * Bound spec.declarative.modelConfig name; omitted for non-declarative
   * agents. The model editor pre-selects this EXACT config — two configs that
   * share a {provider, model} pair can't be told apart by `model` alone.
   */
  modelConfig?: string;
}

/**
 * Response from GET /api/v1/agents — one Casbin-scoped list of the caller's
 * agents (Story 53.1 collapsed the former hub/installed buckets into this single
 * list). Always present (default []).
 */
export interface AgentsResponse {
  agents: InstalledAgent[];
}

/**
 * List the caller's agents. The server filters to the caller's Casbin-accessible
 * namespaces — a user with no project access gets an empty list with 200, never
 * an error.
 */
export async function listAgents(): Promise<AgentsResponse> {
  const response = await apiClient.get<AgentsResponse>("/v1/agents");
  return response.data;
}

/**
 * A kagent ModelConfig the agent in {namespace} can be repointed at — name plus
 * the resolved {provider, model} label. No secret reference ever reaches the
 * client (NFR-A4): the dropdown only needs a name to select and a label to show.
 */
export interface ModelConfigSummary {
  name: string;
  provider: string;
  model: string;
}

/** Response from GET /api/v1/agents/{namespace}/{name}/modelconfigs. */
export interface ModelConfigsResponse {
  modelConfigs: ModelConfigSummary[];
}

/**
 * List the ModelConfigs available to an agent's namespace. The {name} segment
 * scopes authorization to that agent via the Casbin namespace check — the pool
 * itself spans the namespace.
 */
export async function getModelConfigs(
  namespace: string,
  name: string
): Promise<ModelConfigsResponse> {
  const response = await apiClient.get<ModelConfigsResponse>(
    `/v1/agents/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/modelconfigs`
  );
  return response.data;
}

/**
 * A model in the Models tab (Story 53.4): a kagent ModelConfig surfaced as
 * identity + resolved {provider, model}. No secret reference ever reaches the
 * client (NFR-A4).
 */
export interface ModelSummary {
  name: string;
  namespace: string;
  provider: string;
  model: string;
}

/** Response from GET /api/v1/agents/models. */
export interface ModelsResponse {
  models: ModelSummary[];
}

/**
 * Create-model request. `apiKey` is write-only — the server mints a Secret from
 * it and never echoes it back.
 */
export interface CreateModelRequest {
  name: string;
  provider: string;
  model: string;
  namespace: string;
  apiKey: string;
}

/**
 * List the caller's Casbin-accessible models. The server filters to accessible
 * namespaces — a user with no access gets an empty list with 200, never an error.
 */
export async function listModels(): Promise<ModelsResponse> {
  const response = await apiClient.get<ModelsResponse>("/v1/agents/models");
  return response.data;
}

/**
 * List agent-template RGDs, discovered by schema.kind == KnodexAgentTemplate
 * (not the catalog annotation). Returns the same envelope as the RGD catalog,
 * so the Deploy action can reuse the standard deploy flow (/deploy/{name}).
 */
export async function listAgentTemplates(): Promise<RGDListResponse> {
  const response = await apiClient.get<RGDListResponse>("/v1/agents/templates");
  return response.data;
}

/**
 * Create a model: the server orchestrates an API-key Secret + a
 * KnodexAgentModelConfig instance (KRO reconciles it into a kagent ModelConfig).
 * Returns the summary —
 * never the apiKey.
 */
export async function createModel(req: CreateModelRequest): Promise<ModelSummary> {
  const response = await apiClient.post<ModelSummary>("/v1/agents/models", req);
  return response.data;
}

/**
 * Repoint an agent at a different ModelConfig. The server patches ONLY
 * spec.declarative.modelConfig — systemMessage/tools/type are never touched —
 * and echoes the new resolved {provider, name} so the badge updates immediately.
 */
export async function patchAgentModel(
  namespace: string,
  name: string,
  modelConfig: string
): Promise<AgentModel> {
  const response = await apiClient.patch<AgentModel>(
    `/v1/agents/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/model`,
    { modelConfig }
  );
  return response.data;
}
