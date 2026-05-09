// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * SSO Provider types for the Settings UI.
 * ClientSecret is write-only — the backend never returns it.
 */

/** Allowed values for OAuth2 token endpoint authentication. */
export type TokenEndpointAuthMethod = "client_secret_basic" | "none";

/**
 * Optional explicit OIDC endpoints. When all three are set, the server
 * skips /.well-known/openid-configuration discovery and uses these directly.
 * Required for IdPs that serve incomplete discovery documents (e.g., Supabase
 * GoTrue, which omits authorization_endpoint). All-or-nothing — partial input
 * is rejected by the API.
 */
export interface ExplicitOIDCEndpoints {
  authorizationURL?: string;
  tokenURL?: string;
  jwksURL?: string;
}

/** SSO provider as returned by the API (no secret) */
export interface SSOProvider extends ExplicitOIDCEndpoints {
  name: string;
  issuerURL: string;
  clientID: string;
  redirectURL: string;
  scopes: string[];
  tokenEndpointAuthMethod?: TokenEndpointAuthMethod;
}

/** Request body for creating a new SSO provider */
export interface CreateSSOProviderRequest extends ExplicitOIDCEndpoints {
  name: string;
  issuerURL: string;
  clientID: string;
  clientSecret: string;
  redirectURL: string;
  scopes: string[];
  tokenEndpointAuthMethod?: TokenEndpointAuthMethod;
}

/** Request body for updating an existing SSO provider */
export interface UpdateSSOProviderRequest extends ExplicitOIDCEndpoints {
  issuerURL: string;
  clientID: string;
  clientSecret?: string;
  redirectURL: string;
  scopes: string[];
  tokenEndpointAuthMethod?: TokenEndpointAuthMethod;
}
