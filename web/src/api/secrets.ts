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
 * List secrets across the user's accessible namespaces. The optional
 * `namespace` filter narrows to a single namespace; the server still
 * enforces membership and returns an empty list for unauthorized
 * namespaces (matches Instances). No project param: secrets are
 * namespace-keyed under the unified Casbin model.
 */
export async function listSecrets(
  options?: { namespace?: string; limit?: number; continue?: string },
): Promise<SecretListResponse> {
  const response = await apiClient.get<SecretListResponse>("/v1/secrets", {
    params: options,
  });
  return response.data;
}

/**
 * Create a new secret in `namespace`. Casbin object emitted by the
 * server middleware: `secrets/{namespace}/{name}`.
 */
export async function createSecret(namespace: string, req: CreateSecretRequest): Promise<Secret> {
  const response = await apiClient.post<Secret>(
    `/v1/namespaces/${encodeURIComponent(namespace)}/secrets`,
    req,
  );
  return response.data;
}

/**
 * Check if a secret exists without fetching its data (HEAD request).
 * Resolves normally on 200, rejects on 404 (not found / no namespace access).
 */
export async function checkSecretExists(name: string, namespace: string): Promise<void> {
  await apiClient.head(
    `/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`,
  );
}

/**
 * Get a single secret with full data (values included).
 */
export async function getSecret(name: string, namespace: string): Promise<SecretDetail> {
  const response = await apiClient.get<SecretDetail>(
    `/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`,
  );
  return response.data;
}

/**
 * Update an existing secret's data and (optionally) typed metadata.
 *
 * `metadata` semantics mirror the server contract: undefined leaves the
 * existing labels/annotations untouched; a present object is a full
 * replacement of the three metadata fields, with empty strings clearing
 * them. The form layer is responsible for sending `metadata` only when
 * the user actually touched those fields.
 */
export async function updateSecret(
  name: string,
  namespace: string,
  req: UpdateSecretRequest,
): Promise<Secret> {
  const body: UpdateSecretRequest =
    req.metadata !== undefined
      ? { data: req.data, metadata: req.metadata }
      : { data: req.data };
  const response = await apiClient.put<Secret>(
    `/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`,
    body,
  );
  return response.data;
}

/**
 * Delete a secret. The response carries any warnings about Instances
 * that still reference this secret (best-effort scan).
 */
export async function deleteSecret(name: string, namespace: string): Promise<DeleteSecretResponse> {
  const response = await apiClient.delete<DeleteSecretResponse>(
    `/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`,
  );
  return response.data;
}
