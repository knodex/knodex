// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { AuthType, GitHubAppType } from "@/types/repository";

/**
 * Shared form data shape for repository hooks.
 * All hooks that read or build from form state should reference this type.
 */
export interface RepositoryFormData {
  name: string;
  projectId: string;
  repoURL: string;
  authType: AuthType;
  defaultBranch: string;
  sshAuth?: { privateKey: string };
  httpsAuth?: {
    username: string;
    password: string;
    bearerToken: string;
    tlsClientCert: string;
    tlsClientKey: string;
    insecureSkipTLSVerify: boolean;
  };
  githubAppAuth?: {
    appType: GitHubAppType;
    appId: string;
    installationId: string;
    privateKey: string;
    enterpriseUrl: string;
  };
}
