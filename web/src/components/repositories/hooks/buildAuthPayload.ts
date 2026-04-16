// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { SSHAuthConfig, HTTPSAuthConfig, GitHubAppAuthConfig } from "@/types/repository";
import type { RepositoryFormData } from "./types";

interface AuthPayload {
  sshAuth?: SSHAuthConfig;
  httpsAuth?: HTTPSAuthConfig;
  githubAppAuth?: GitHubAppAuthConfig;
}

/**
 * Builds the auth-specific credential payload from form data.
 * Shared between form submission and connection testing.
 */
export function buildAuthPayload(data: Pick<RepositoryFormData, "authType" | "sshAuth" | "httpsAuth" | "githubAppAuth">): AuthPayload {
  if (data.authType === "ssh" && data.sshAuth) {
    return { sshAuth: { privateKey: data.sshAuth.privateKey } };
  }

  if (data.authType === "https" && data.httpsAuth) {
    return {
      httpsAuth: {
        username: data.httpsAuth.username || undefined,
        password: data.httpsAuth.password || undefined,
        bearerToken: data.httpsAuth.bearerToken || undefined,
        tlsClientCert: data.httpsAuth.tlsClientCert || undefined,
        tlsClientKey: data.httpsAuth.tlsClientKey || undefined,
        insecureSkipTLSVerify: data.httpsAuth.insecureSkipTLSVerify,
      },
    };
  }

  if (data.authType === "github-app" && data.githubAppAuth) {
    return {
      githubAppAuth: {
        appType: data.githubAppAuth.appType,
        appId: data.githubAppAuth.appId,
        installationId: data.githubAppAuth.installationId,
        privateKey: data.githubAppAuth.privateKey,
        enterpriseUrl: data.githubAppAuth.enterpriseUrl || undefined,
      },
    };
  }

  return {};
}
