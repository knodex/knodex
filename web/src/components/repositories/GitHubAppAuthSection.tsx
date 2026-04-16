// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { UseFormRegisterReturn } from "react-hook-form";
import { Info, Eye, EyeOff } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import type { GitHubAppType } from "@/types/repository";

const inputClasses = "w-full px-3 py-2 border border-border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary";
const errorClasses = "mt-1 text-sm text-destructive";

interface GitHubAppAuthSectionProps {
  registerAppId: UseFormRegisterReturn;
  registerInstallationId: UseFormRegisterReturn;
  registerPrivateKey: UseFormRegisterReturn;
  registerEnterpriseUrl: UseFormRegisterReturn;
  appIdError?: string;
  installationIdError?: string;
  privateKeyError?: string;
  enterpriseUrlError?: string;
  githubAppType: GitHubAppType;
  githubAppTypeSetters: { github: () => void; "github-enterprise": () => void };
  showPrivateKey: boolean;
  togglePrivateKey: () => void;
}

export function GitHubAppAuthSection({
  registerAppId,
  registerInstallationId,
  registerPrivateKey,
  registerEnterpriseUrl,
  appIdError,
  installationIdError,
  privateKeyError,
  enterpriseUrlError,
  githubAppType,
  githubAppTypeSetters,
  showPrivateKey,
  togglePrivateKey,
}: GitHubAppAuthSectionProps) {
  return (
    <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Info className="h-4 w-4" />
        <span>GitHub App authentication - recommended for organizations</span>
      </div>

      {/* App Type */}
      <div>
        {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
        <label className="text-sm font-medium" id="github-app-type-label">GitHub App Type *</label>
        <div className="flex gap-2 mt-2">
          {(["github", "github-enterprise"] as GitHubAppType[]).map((type) => (
            <button
              key={type}
              type="button"
              onClick={githubAppTypeSetters[type]}
              className={`flex-1 px-3 py-1.5 text-sm rounded-md border transition-colors ${
                githubAppType === type
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background border-border hover:bg-accent hover:text-accent-foreground"
              }`}
            >
              {type === "github" ? "GitHub.com" : "GitHub Enterprise"}
            </button>
          ))}
        </div>
      </div>

      {/* Enterprise URL (conditional) */}
      {githubAppType === "github-enterprise" && (
        <div>
          <label htmlFor="githubEnterpriseUrl" className="text-sm font-medium">
            Enterprise URL *
          </label>
          <input
            id="githubEnterpriseUrl"
            {...registerEnterpriseUrl}
            className={inputClasses}
            placeholder="https://github.mycompany.com"
          />
          {enterpriseUrlError && (
            <p className={errorClasses}>{enterpriseUrlError}</p>
          )}
        </div>
      )}

      {/* App ID & Installation ID */}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label htmlFor="githubAppId" className="text-sm font-medium">
            App ID *
          </label>
          <input
            id="githubAppId"
            {...registerAppId}
            className={inputClasses}
            placeholder="123456"
          />
          {appIdError && (
            <p className={errorClasses}>{appIdError}</p>
          )}
        </div>
        <div>
          <label htmlFor="githubInstallId" className="text-sm font-medium">
            Installation ID *
          </label>
          <input
            id="githubInstallId"
            {...registerInstallationId}
            className={inputClasses}
            placeholder="12345678"
          />
          {installationIdError && (
            <p className={errorClasses}>{installationIdError}</p>
          )}
        </div>
      </div>

      {/* Private Key */}
      <div>
        <div className="flex justify-between items-center mb-1.5">
          <label htmlFor="githubAppPrivateKey" className="text-sm font-medium">
            Private Key *
          </label>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={togglePrivateKey}
          >
            {showPrivateKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        </div>
        <textarea
          id="githubAppPrivateKey"
          {...registerPrivateKey}
          className={`${inputClasses} font-mono text-xs min-h-[120px]`}
          placeholder={showPrivateKey ? "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----" : "••••••••••••"}
          style={showPrivateKey ? {} : { WebkitTextSecurity: "disc" } as React.CSSProperties}
        />
        {privateKeyError && (
          <p className={errorClasses}>{privateKeyError}</p>
        )}
        <p className="mt-1 text-xs text-muted-foreground">
          Download the private key when creating your GitHub App
        </p>
      </div>
    </div>
  );
}
