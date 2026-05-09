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
  const [loadError, setLoadError] = useState(false);
  // Default to false. The previous default of true caused the local form to
  // flash for SSO-only deployments during the initial load, encouraging users
  // to enter credentials that would then 404. We render `loading` first, then
  // reveal whichever option(s) the server reports.
  const [localLoginEnabled, setLocalLoginEnabled] = useState(false);

  useEffect(() => {
    // Redirect if already authenticated
    if (isAuthenticated) {
      navigate('/');
      return;
    }

    // Fetch OIDC providers
    const fetchProviders = async () => {
      try {
        const result = await getOIDCProviders();
        setProviders(result.providers.filter((p) => p.enabled));
        setLocalLoginEnabled(result.localLoginEnabled);
        setLoadError(false);
      } catch (error) {
        logger.error('[LoginPage] Failed to fetch OIDC providers:', error);
        setLoadError(true);
      } finally {
        setLoading(false);
      }
    };

    fetchProviders();
  }, [isAuthenticated, navigate]);

  // Sentinel: loaded but no login methods are available. Surface the
  // misconfiguration explicitly instead of rendering an empty card.
  const noLoginMethods = !loading && !loadError && !localLoginEnabled && providers.length === 0;

  const handleLoginSuccess = () => {
    // Navigate to dashboard after successful login
    navigate('/');
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-secondary/30 px-4">
      <div className="w-full max-w-md">
        {/* Login Card */}
        <Card className="p-6 space-y-6">
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

          {/* Load error */}
          {loadError && (
            <div className="space-y-2 text-center" role="alert">
              <h2 className="text-sm font-medium text-destructive">
                Unable to load login options
              </h2>
              <p className="text-xs text-muted-foreground">
                The server is unreachable. Refresh the page to retry, or contact
                your administrator if this persists.
              </p>
            </div>
          )}

          {/* No login methods available (misconfiguration) */}
          {noLoginMethods && (
            <div className="space-y-2 text-center" role="alert">
              <h2 className="text-sm font-medium text-destructive">
                No login methods available
              </h2>
              <p className="text-xs text-muted-foreground">
                Local login is disabled and no SSO providers are configured.
                Contact your administrator.
              </p>
            </div>
          )}

          {/* Local Admin Login */}
          {!loading && !loadError && localLoginEnabled && (
            <div className="space-y-3">
              <h2 className="text-sm font-medium text-muted-foreground text-center">
                Administrator Login
              </h2>
              <LocalAdminForm onSuccess={handleLoginSuccess} />
            </div>
          )}

          {/* Divider */}
          {!loading && !loadError && localLoginEnabled && providers.length > 0 && (
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
          {!loading && !loadError && providers.length > 0 && (
            <div className="space-y-3">
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
      </div>
    </div>
  );
}
