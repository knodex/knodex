/**
 * TypeScript types for Repository Configuration
 */

/**
 * Authentication types for repository credentials
 */
export type AuthType = 'ssh' | 'https' | 'github-app';

/**
 * GitHub App types
 */
export type GitHubAppType = 'github' | 'github-enterprise';

/**
 * SSH authentication configuration
 */
export interface SSHAuthConfig {
  privateKey: string;
}

/**
 * HTTPS authentication configuration
 */
export interface HTTPSAuthConfig {
  username?: string;
  password?: string;
  bearerToken?: string;
  tlsClientCert?: string;
  tlsClientKey?: string;
  insecureSkipTLSVerify?: boolean;
}

/**
 * GitHub App authentication configuration
 */
export interface GitHubAppAuthConfig {
  appType: GitHubAppType;
  appId: string;
  installationId: string;
  privateKey: string;
  enterpriseUrl?: string;
}

/**
 * Secret reference for credential storage
 */
export interface SecretReference {
  name: string;
  namespace: string;
}

/**
 * Repository configuration for GitOps deployments
 */
export interface RepositoryConfig {
  id: string;
  name: string;
  projectId?: string;
  repoURL?: string;
  authType?: AuthType;
  secretRef?: SecretReference;

  // Legacy fields (populated from RepoURL parsing for backwards compatibility)
  owner?: string;
  repo?: string;

  defaultBranch: string;
  createdBy?: string;
  createdAt?: string;
  updatedBy?: string;
  updatedAt?: string;
  validationStatus?: "valid" | "invalid" | "unknown";
  validationMessage?: string;
}

/**
 * API response for listing repository configs
 */
export interface RepositoryListResponse {
  items: RepositoryConfig[];
  totalCount: number;
}

/**
 * Request to create a repository with inline credentials (ArgoCD-style)
 */
export interface CreateRepositoryRequest {
  name: string;
  projectId: string;
  repoURL: string;
  authType: AuthType;
  defaultBranch: string;

  // Auth-specific credentials (only one should be provided based on authType)
  sshAuth?: SSHAuthConfig;
  httpsAuth?: HTTPSAuthConfig;
  githubAppAuth?: GitHubAppAuthConfig;
}

/**
 * Request to update an existing repository configuration
 */
export interface UpdateRepositoryRequest {
  name?: string;
  defaultBranch?: string;

  // Credential update fields
  repoURL?: string;
  authType?: AuthType;
  sshAuth?: SSHAuthConfig;
  httpsAuth?: HTTPSAuthConfig;
  githubAppAuth?: GitHubAppAuthConfig;
}

/**
 * Request to test repository connection (ArgoCD-style)
 */
export interface TestConnectionRequest {
  repoURL: string;
  authType: AuthType;

  // Auth-specific credentials (only one should be provided based on authType)
  sshAuth?: SSHAuthConfig;
  httpsAuth?: HTTPSAuthConfig;
  githubAppAuth?: GitHubAppAuthConfig;
}

/**
 * Response from testing repository connection
 */
export interface TestConnectionResponse {
  valid: boolean;
  message: string;
}

/**
 * Valid auth types constant for validation
 */
export const AUTH_TYPES: AuthType[] = ['ssh', 'https', 'github-app'];

/**
 * Valid GitHub app types constant for validation
 */
export const GITHUB_APP_TYPES: GitHubAppType[] = ['github', 'github-enterprise'];

/**
 * Helper to get display name for auth type
 */
export function getAuthTypeDisplayName(authType: AuthType): string {
  switch (authType) {
    case 'ssh':
      return 'SSH';
    case 'https':
      return 'HTTPS';
    case 'github-app':
      return 'GitHub App';
    default:
      return authType;
  }
}

/**
 * Helper to get repository display URL
 */
export function getRepositoryDisplayURL(config: RepositoryConfig): string {
  if (config.repoURL) {
    return config.repoURL;
  }
  if (config.owner && config.repo) {
    return `https://github.com/${config.owner}/${config.repo}`;
  }
  return '';
}
