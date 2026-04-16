// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { UseFormRegisterReturn } from "react-hook-form";
import { Info, Eye, EyeOff } from "@/lib/icons";
import { Button } from "@/components/ui/button";

const inputClasses = "w-full px-3 py-2 border border-border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary";
const errorClasses = "mt-1 text-sm text-destructive";

interface SSHAuthSectionProps {
  registerPrivateKey: UseFormRegisterReturn;
  privateKeyError?: string;
  showPrivateKey: boolean;
  togglePrivateKey: () => void;
}

export function SSHAuthSection({
  registerPrivateKey,
  privateKeyError,
  showPrivateKey,
  togglePrivateKey,
}: SSHAuthSectionProps) {
  return (
    <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Info className="h-4 w-4" />
        <span>SSH authentication using a private key</span>
      </div>

      <div>
        <div className="flex justify-between items-center mb-1.5">
          <label htmlFor="sshPrivateKey" className="text-sm font-medium">
            SSH Private Key *
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
          id="sshPrivateKey"
          {...registerPrivateKey}
          className={`${inputClasses} font-mono text-xs min-h-[120px]`}
          placeholder={showPrivateKey ? "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----" : "••••••••••••"}
          style={showPrivateKey ? {} : { WebkitTextSecurity: "disc" } as React.CSSProperties}
        />
        {privateKeyError && (
          <p className={errorClasses}>{privateKeyError}</p>
        )}
        <p className="mt-1 text-xs text-muted-foreground">
          Paste your SSH private key in PEM format
        </p>
      </div>
    </div>
  );
}
