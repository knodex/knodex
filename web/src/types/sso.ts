// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * SSO Provider types for the Settings UI.
 * ClientSecret is write-only — the backend never returns it.
 */

/** SSO provider as returned by the API (no secret) */
export interface SSOProvider {
  name: string;
  issuerURL: string;
  clientID: string;
  redirectURL: string;
  scopes: string[];
}

/** Request body for creating a new SSO provider */
export interface CreateSSOProviderRequest {
  name: string;
  issuerURL: string;
  clientID: string;
  clientSecret: string;
  redirectURL: string;
  scopes: string[];
}

/** Request body for updating an existing SSO provider */
export interface UpdateSSOProviderRequest {
  issuerURL: string;
  clientID: string;
  clientSecret?: string;
  redirectURL: string;
  scopes: string[];
}
