// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

export interface Secret {
  name: string;
  namespace: string;
  keys: string[];
  createdAt: string;
  labels?: Record<string, string>;
}

export interface SecretDetail {
  name: string;
  namespace: string;
  data: Record<string, string>;
  createdAt: string;
  labels?: Record<string, string>;
}

export interface CreateSecretRequest {
  name: string;
  namespace: string;
  data: Record<string, string>;
}

export interface UpdateSecretRequest {
  namespace: string;
  data: Record<string, string>;
}

export interface SecretListResponse {
  items: Secret[];
  totalCount: number;
  continue?: string;
  hasMore: boolean;
}

export interface DeleteSecretResponse {
  deleted: boolean;
  warnings?: string[];
}

/**
 * SecretRef represents a secret reference from an externalRef resource.
 * "fixed" — hardcoded name/namespace literals.
 * "dynamic" — name/namespace computed from other resources (non-input CEL expressions).
 * "provided" — passthrough input: user supplies the secret name/namespace at deploy time.
 */
export interface SecretRef {
  /** "fixed", "dynamic", or "provided" (user-supplied at deploy time) */
  type: "fixed" | "dynamic" | "provided";
  /** Literal secret name (for fixed refs) */
  name?: string;
  /** Literal secret namespace (for fixed refs) */
  namespace?: string;
  /** CEL expression for the secret name (for dynamic refs) */
  nameExpr?: string;
  /** CEL expression for the secret namespace (for dynamic refs) */
  namespaceExpr?: string;
  /** Resource ID within the RGD (e.g., "0-Secret") */
  id: string;
  /** Semantic identifier matching the externalRef field name in the schema (e.g., "dbSecret") */
  externalRefId?: string;
  /** Human-readable description of the secret's purpose, from the RGD schema */
  description?: string;
}
