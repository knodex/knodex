// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo } from "react";
import type { UseFormSetValue } from "react-hook-form";
import type { AuthType, GitHubAppType } from "@/types/repository";
import type { RepositoryFormData } from "./types";

/**
 * Hook encapsulating all auth type setter callbacks.
 */
export function useAuthTypeHandlers(setValue: UseFormSetValue<RepositoryFormData>) {
  const setAuthTypeSsh = useCallback(() => setValue("authType", "ssh" as AuthType), [setValue]);
  const setAuthTypeHttps = useCallback(() => setValue("authType", "https" as AuthType), [setValue]);
  const setAuthTypeGithubApp = useCallback(() => setValue("authType", "github-app" as AuthType), [setValue]);

  const authTypeSetters = useMemo(() => ({
    ssh: setAuthTypeSsh,
    https: setAuthTypeHttps,
    "github-app": setAuthTypeGithubApp,
  }), [setAuthTypeSsh, setAuthTypeHttps, setAuthTypeGithubApp]);

  const setGithubAppTypeGithub = useCallback(() => setValue("githubAppAuth.appType", "github" as GitHubAppType), [setValue]);
  const setGithubAppTypeEnterprise = useCallback(() => setValue("githubAppAuth.appType", "github-enterprise" as GitHubAppType), [setValue]);

  const githubAppTypeSetters = useMemo(() => ({
    github: setGithubAppTypeGithub,
    "github-enterprise": setGithubAppTypeEnterprise,
  }), [setGithubAppTypeGithub, setGithubAppTypeEnterprise]);

  return { authTypeSetters, githubAppTypeSetters };
}
