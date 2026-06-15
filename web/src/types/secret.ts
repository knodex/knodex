// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Secret rotation policy. Stored server-side as the
 * `knodex.io/rotation` label.
 */
export type SecretRotation = "manual" | "auto";

/**
 * Server-computed expiry status. Derived from
 * `metadata.expiresAt` with a 30-day "expiring soon" window.
 * Empty/absent means no expiration date is set.
 */
export type SecretStatus = "active" | "expiring-soon" | "expired";

/**
 * Typed metadata exposed alongside the raw labels map.
 * Mirrors the SecretMetadata wire shape produced by the server.
 *
 * On the underlying K8s Secret:
 *  - `rotation` is stored as the `knodex.io/rotation` label
 *  - `docsUrl`  is stored as the `knodex.io/docs-url` annotation
 *  - `expiresAt` is stored as the `knodex.io/expires-at` annotation (RFC3339)
 */
export interface SecretMetadata {
  rotation?: SecretRotation;
  docsUrl?: string;
  /** RFC3339 timestamp. The Create/Edit form models this as a date input
   *  and submits an end-of-day UTC timestamp. */
  expiresAt?: string;
}

export interface Secret {
  name: string;
  namespace: string;
  keys: string[];
  createdAt: string;
  updatedAt?: string;
  labels?: Record<string, string>;
  metadata?: SecretMetadata;
  status?: SecretStatus;
}

export interface SecretDetail {
  name: string;
  namespace: string;
  data: Record<string, string>;
  createdAt: string;
  updatedAt?: string;
  labels?: Record<string, string>;
  metadata?: SecretMetadata;
  status?: SecretStatus;
}

/**
 * Create payload. The namespace is carried in the URL path
 * (/api/v1/namespaces/{ns}/secrets), NOT in the body — mirroring Instances.
 */
export interface CreateSecretRequest {
  name: string;
  data: Record<string, string>;
  metadata?: SecretMetadata;
}

/**
 * Update semantics for `metadata`:
 *  - omitted/undefined → leave existing metadata on the K8s object untouched
 *  - present → treated as a full replacement; empty-string fields CLEAR the
 *    corresponding label/annotation on the server
 *
 * The namespace is carried in the URL path, NOT in the body.
 */
export interface UpdateSecretRequest {
  data: Record<string, string>;
  metadata?: SecretMetadata;
}

export interface SecretListResponse {
  items: Secret[];
  pageCount: number;
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
