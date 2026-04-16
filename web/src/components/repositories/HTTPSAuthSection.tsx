// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { UseFormRegisterReturn } from "react-hook-form";
import { Info, Eye, EyeOff } from "@/lib/icons";
import { Button } from "@/components/ui/button";

const inputClasses = "w-full px-3 py-2 border border-border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary";
const errorClasses = "mt-1 text-sm text-destructive";

interface HTTPSAuthSectionProps {
  registerUsername: UseFormRegisterReturn;
  registerPassword: UseFormRegisterReturn;
  registerBearerToken: UseFormRegisterReturn;
  registerTlsCert: UseFormRegisterReturn;
  registerTlsKey: UseFormRegisterReturn;
  httpsAuthError?: string;
  showPassword: boolean;
  togglePassword: () => void;
  showBearerToken: boolean;
  toggleBearerToken: () => void;
}

export function HTTPSAuthSection({
  registerUsername,
  registerPassword,
  registerBearerToken,
  registerTlsCert,
  registerTlsKey,
  httpsAuthError,
  showPassword,
  togglePassword,
  showBearerToken,
  toggleBearerToken,
}: HTTPSAuthSectionProps) {
  return (
    <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Info className="h-4 w-4" />
        <span>HTTPS authentication - provide at least one method</span>
      </div>

      {/* Username & Password */}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label htmlFor="httpsUsername" className="text-sm font-medium">
            Username
          </label>
          <input
            id="httpsUsername"
            {...registerUsername}
            className={inputClasses}
            placeholder="git"
          />
        </div>
        <div>
          <div className="flex justify-between items-center">
            <label htmlFor="httpsPassword" className="text-sm font-medium">
              Password / Token
            </label>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={togglePassword}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </div>
          <input
            id="httpsPassword"
            type={showPassword ? "text" : "password"}
            {...registerPassword}
            className={inputClasses}
            placeholder="••••••••"
          />
        </div>
      </div>

      {/* Bearer Token */}
      <div>
        <div className="flex justify-between items-center">
          <label htmlFor="httpsBearerToken" className="text-sm font-medium">
            Bearer Token
          </label>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={toggleBearerToken}
          >
            {showBearerToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        </div>
        <input
          id="httpsBearerToken"
          type={showBearerToken ? "text" : "password"}
          {...registerBearerToken}
          className={inputClasses}
          placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
        />
        <p className="mt-1 text-xs text-muted-foreground">
          GitHub Personal Access Token or OAuth token
        </p>
      </div>

      {/* TLS Client Certificate (collapsible) */}
      <details className="group">
        <summary className="cursor-pointer text-sm font-medium text-muted-foreground hover:text-foreground">
          TLS Client Certificate (Advanced)
        </summary>
        <div className="mt-3 space-y-3">
          <div>
            <label htmlFor="httpsTlsCert" className="text-sm font-medium">
              TLS Client Certificate
            </label>
            <textarea
              id="httpsTlsCert"
              {...registerTlsCert}
              className={`${inputClasses} font-mono text-xs min-h-[80px]`}
              placeholder="-----BEGIN CERTIFICATE-----"
            />
          </div>
          <div>
            <label htmlFor="httpsTlsKey" className="text-sm font-medium">
              TLS Client Key
            </label>
            <textarea
              id="httpsTlsKey"
              {...registerTlsKey}
              className={`${inputClasses} font-mono text-xs min-h-[80px]`}
              placeholder="-----BEGIN PRIVATE KEY-----"
            />
          </div>
        </div>
      </details>

      {httpsAuthError && (
        <p className={errorClasses}>{httpsAuthError}</p>
      )}
    </div>
  );
}
