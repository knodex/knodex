// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Button } from '@/components/ui/button';
import { initiateOIDCLogin } from '@/api/auth';
import { EntraIDIcon } from './icons/EntraIDIcon';
import { GoogleIcon } from './icons/GoogleIcon';
import { KeyRound, Shield, Lock } from 'lucide-react';
import type { ReactNode } from 'react';

interface OIDCButtonProps {
  provider: string;
  displayName: string;
  disabled?: boolean;
}

// Map provider names to their official icons
const getProviderIcon = (provider: string): ReactNode => {
  const lowerProvider = provider.toLowerCase();

  switch (lowerProvider) {
    case 'entraid':
    case 'azuread':
      return <EntraIDIcon className="h-5 w-5" />;
    case 'google':
      return <GoogleIcon className="h-5 w-5" />;
    case 'keycloak':
      return <KeyRound className="h-5 w-5 text-primary" />;
    case 'okta':
      return <Shield className="h-5 w-5 text-primary" />;
    case 'auth0':
      return <Lock className="h-5 w-5 text-primary" />;
    default:
      return <Lock className="h-5 w-5 text-primary" />;
  }
};

export function OIDCButton({ provider, displayName, disabled }: OIDCButtonProps) {
  const handleClick = () => {
    initiateOIDCLogin(provider);
  };

  const icon = getProviderIcon(provider);

  return (
    <Button
      onClick={handleClick}
      disabled={disabled}
      variant="accent"
      className="w-full justify-start gap-3"
    >
      {icon}
      <span>Continue with {displayName}</span>
    </Button>
  );
}
