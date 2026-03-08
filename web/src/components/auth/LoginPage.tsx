// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '@/components/ui/card';
import { OIDCButton } from './OIDCButton';
import { LocalAdminForm } from './LocalAdminForm';
import { useIsAuthenticated } from '@/hooks/useAuth';
import { getOIDCProviders, type OIDCProvider } from '@/api/auth';
import { logger } from '@/lib/logger';

export function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useIsAuthenticated();
  const [providers, setProviders] = useState<OIDCProvider[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Redirect if already authenticated
    if (isAuthenticated) {
      navigate('/');
      return;
    }

    // Fetch OIDC providers
    const fetchProviders = async () => {
      try {
        const providers = await getOIDCProviders();
        setProviders(providers.filter((p) => p.enabled));
      } catch (error) {
        logger.error('[LoginPage] Failed to fetch OIDC providers:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchProviders();
  }, [isAuthenticated, navigate]);

  const handleLoginSuccess = () => {
    // Navigate to dashboard after successful login
    navigate('/');
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-secondary/30 px-4">
      <div className="w-full max-w-md space-y-8">
        {/* Header */}
        <div className="text-center">
          <div className="flex justify-center mb-4">
            <img src="/logo.svg" alt="Knodex" className="h-20 w-20" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Knodex</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Kubernetes Native Self Service Platform
          </p>
        </div>

        {/* Login Card */}
        <Card className="p-6 space-y-6">
          {/* Local Admin Login */}
          <div className="space-y-3">
            <h2 className="text-sm font-medium text-muted-foreground text-center">
              Administrator Login
            </h2>
            <LocalAdminForm onSuccess={handleLoginSuccess} />
          </div>

          {/* Divider */}
          {!loading && providers.length > 0 && (
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-border"></div>
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="bg-card px-2 text-muted-foreground">
                  Or
                </span>
              </div>
            </div>
          )}

          {/* OIDC Providers */}
          {!loading && providers.length > 0 && (
            <div className="space-y-3">
              <h2 className="text-sm font-medium text-muted-foreground text-center">
                Single Sign-On
              </h2>
              {providers.map((provider) => (
                <OIDCButton
                  key={provider.name}
                  provider={provider.name}
                  displayName={provider.display_name}
                />
              ))}
            </div>
          )}
        </Card>

        {/* Footer */}
        <p className="text-center text-xs text-muted-foreground">
          By signing in, you agree to our Terms of Service and Privacy Policy
        </p>
      </div>
    </div>
  );
}
