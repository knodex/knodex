import apiClient from './client';
import { logger } from '@/lib/logger';

export interface LocalAdminLoginRequest {
  username: string;
  password: string;
}

// LoginResponse matches the server's LoginResponse struct.
// The JWT is delivered via HttpOnly cookie (Set-Cookie header), not in the response body.
export interface LoginResponse {
  expiresAt: string;
  user: LoginUserInfo;
}

export interface LoginUserInfo {
  id: string;
  email: string;
  displayName?: string;
  projects?: string[];
  defaultProject?: string;
  groups?: string[];
  roles?: Record<string, string>;
  casbinRoles?: string[];
  permissions?: Record<string, boolean>;
}

/**
 * Log in with local admin credentials.
 * The JWT is set via HttpOnly cookie by the server.
 * Returns user info and expiration for Zustand store.
 */
export async function localAdminLogin(
  credentials: LocalAdminLoginRequest
): Promise<LoginResponse> {
  const response = await apiClient.post<LoginResponse>(
    '/v1/auth/local/login',
    credentials
  );
  return response.data;
}

/**
 * Initiate OIDC login flow
 * This redirects the browser to the OIDC provider
 */
export function initiateOIDCLogin(provider: string): void {
  const redirectUrl = '/auth/callback';
  window.location.href = `/api/v1/auth/oidc/login?provider=${provider}&redirect=${encodeURIComponent(redirectUrl)}`;
}

/**
 * Exchange an opaque auth code for a session cookie.
 * Used after OIDC callback redirect (code is single-use, 30s TTL).
 * The JWT is set via HttpOnly cookie by the server.
 * Returns user info and expiration for Zustand store.
 */
export async function exchangeAuthCode(code: string): Promise<LoginResponse> {
  const response = await apiClient.post<LoginResponse>('/v1/auth/token-exchange', { code });
  return response.data;
}

/**
 * Request a WebSocket ticket (single-use, 30s TTL).
 * Used to authenticate WebSocket connections without exposing JWT in URL.
 */
export async function getWebSocketTicket(): Promise<string> {
  const response = await apiClient.post<{ ticket: string; expiresAt: string }>('/v1/ws/ticket');
  return response.data.ticket;
}

/**
 * Logout and invalidate session
 */
export async function logout(): Promise<void> {
  await apiClient.post('/v1/auth/logout');
}

/**
 * Get available OIDC providers
 */
export interface OIDCProvider {
  name: string;
  display_name: string;
  enabled: boolean;
}

export async function getOIDCProviders(): Promise<OIDCProvider[]> {
  try {
    const response = await apiClient.get<{ providers: string[] }>('/v1/auth/oidc/providers');
    // Transform backend response (just provider names) to frontend format
    const providerNames = response.data.providers || [];
    return providerNames.map(name => ({
      name,
      display_name: formatProviderName(name),
      enabled: true
    }));
  } catch (error) {
    logger.error('[Auth] Failed to fetch OIDC providers:', error);
    return [];
  }
}

/**
 * Format provider name for display
 * e.g., "azuread" -> "Entra ID", "google" -> "Google"
 */
function formatProviderName(name: string): string {
  const nameMap: Record<string, string> = {
    'azuread': 'Entra ID',
    'entraid': 'Entra ID',
    'google': 'Google',
    'keycloak': 'Keycloak',
    'auth0': 'Auth0',
    'okta': 'Okta'
  };
  return nameMap[name.toLowerCase()] || name.charAt(0).toUpperCase() + name.slice(1);
}

/**
 * Account info response from the backend (server-authoritative data)
 */
export interface AccountInfoResponse {
  userID: string;
  email: string;
  displayName: string;
  groups: string[];
  casbinRoles: string[];
  projects: string[];
  roles: Record<string, string>;
  issuer: string;
  tokenExpiresAt: number;
  tokenIssuedAt: number;
}

/**
 * Get the authenticated user's account info.
 * Returns server-side authoritative data including filtered groups
 * (only groups with Casbin policy mappings).
 */
export async function getAccountInfo(): Promise<AccountInfoResponse> {
  const response = await apiClient.get<AccountInfoResponse>('/v1/account/info');
  return response.data;
}

/**
 * Response from the can-i permission check endpoint
 */
export interface CanIResponse {
  value: 'yes' | 'no';
}

/**
 * Check if the current user has permission to perform an action on a resource.
 * This calls the backend's Casbin enforcer for real-time permission evaluation.
 *
 * ArgoCD-aligned pattern: GET /api/v1/account/can-i/{resource}/{action}/{subresource}
 *
 * @param resource - Resource type (e.g., 'projects', 'instances', 'repositories')
 * @param action - Action to check (e.g., 'create', 'update', 'delete', 'get')
 * @param subresource - Optional subresource/scope (e.g., project name). Use '-' for no subresource.
 * @returns Promise<boolean> - true if user has permission, false otherwise
 */
export async function canI(
  resource: string,
  action: string,
  subresource: string = '-'
): Promise<boolean> {
  try {
    const response = await apiClient.get<CanIResponse>(
      `/v1/account/can-i/${encodeURIComponent(resource)}/${encodeURIComponent(action)}/${encodeURIComponent(subresource)}`
    );
    return response.data.value === 'yes';
  } catch (error) {
    // Distinguish explicit deny (403) from transient errors (network, 500, etc.)
    // 403 = server explicitly denied permission → return false
    // Other errors (network, 500, server restart) → throw to let React Query retry
    const axiosError = error as import('axios').AxiosError;
    if (axiosError?.response?.status === 403) {
      return false;
    }
    // Re-throw so React Query can retry transient failures (e.g., Tilt server restart)
    logger.error('[Auth] Permission check failed (will retry):', error);
    throw error;
  }
}
