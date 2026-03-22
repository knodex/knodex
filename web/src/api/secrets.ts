// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type {
  Secret,
  SecretDetail,
  CreateSecretRequest,
  UpdateSecretRequest,
  SecretListResponse,
  DeleteSecretResponse,
} from "@/types/secret";

/**
 * List secrets for a project with optional pagination
 */
export async function listSecrets(
  project: string,
  options?: { limit?: number; continue?: string },
): Promise<SecretListResponse> {
  const response = await apiClient.get<SecretListResponse>("/v1/secrets", {
    params: { project, ...options },
  });
  return response.data;
}

/**
 * Create a new secret in a project
 */
export async function createSecret(project: string, req: CreateSecretRequest): Promise<Secret> {
  const response = await apiClient.post<Secret>("/v1/secrets", req, {
    params: { project },
  });
  return response.data;
}

/**
 * Check if a secret exists without fetching its data (HEAD request).
 * Resolves normally on 200, throws on 404 (not found) or other errors.
 */
export async function checkSecretExists(name: string, project: string, namespace: string): Promise<void> {
  await apiClient.head(`/v1/secrets/${encodeURIComponent(name)}`, {
    params: { project, namespace },
  });
}

/**
 * Get a single secret with full data (values included)
 */
export async function getSecret(name: string, project: string, namespace: string): Promise<SecretDetail> {
  const response = await apiClient.get<SecretDetail>(`/v1/secrets/${encodeURIComponent(name)}`, {
    params: { project, namespace },
  });
  return response.data;
}

/**
 * Update an existing secret's data
 */
export async function updateSecret(name: string, project: string, req: UpdateSecretRequest): Promise<Secret> {
  const { namespace, data } = req;
  const response = await apiClient.put<Secret>(`/v1/secrets/${encodeURIComponent(name)}`, { namespace, data }, {
    params: { project },
  });
  return response.data;
}

/**
 * Delete a secret
 */
export async function deleteSecret(name: string, project: string, namespace: string): Promise<DeleteSecretResponse> {
  const response = await apiClient.delete<DeleteSecretResponse>(`/v1/secrets/${encodeURIComponent(name)}`, {
    params: { project, namespace },
  });
  return response.data;
}
