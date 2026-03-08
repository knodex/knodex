import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Boxes } from 'lucide-react';
import { useUserStore } from '@/stores/userStore';
import { exchangeAuthCode } from '@/api/auth';
import { logger } from '@/lib/logger';

export function AuthCallback() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const login = useUserStore((state) => state.login);
  const [exchangeError, setExchangeError] = useState<string | null>(null);

  // Derive error state from search params (provider-side errors)
  const paramError = useMemo(() => {
    const errorParam = searchParams.get('error');
    if (errorParam) {
      return decodeURIComponent(errorParam);
    }

    const code = searchParams.get('code');
    if (!code) {
      return 'No authorization code received from authentication provider';
    }

    return null;
  }, [searchParams]);

  const error = exchangeError || paramError;

  useEffect(() => {
    const code = searchParams.get('code');

    if (paramError) {
      // Redirect to login after 3 seconds on error
      const timeoutId = setTimeout(() => navigate('/login'), 3000);
      return () => clearTimeout(timeoutId);
    }

    if (code) {
      // Exchange opaque code for session cookie via backend
      let cancelled = false;
      exchangeAuthCode(code)
        .then((resp) => {
          if (cancelled) return;
          // Store user info in Zustand store (JWT is in HttpOnly cookie)
          login(resp.user, resp.expiresAt);
          navigate('/', { replace: true });
        })
        .catch((err) => {
          if (cancelled) return;
          logger.error('[AuthCallback] Failed to exchange auth code:', err);
          setExchangeError('Failed to complete authentication. The code may have expired.');
          setTimeout(() => navigate('/login'), 3000);
        });
      return () => { cancelled = true; };
    }
  }, [searchParams, login, navigate, paramError]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-secondary/30 px-4">
      <div className="w-full max-w-md space-y-8 text-center">
        <div className="flex justify-center mb-4">
          <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Boxes className="h-6 w-6" />
          </div>
        </div>

        {error ? (
          <div className="space-y-4">
            <h1 className="text-2xl font-bold text-destructive">Authentication Failed</h1>
            <p className="text-sm text-muted-foreground">{error}</p>
            <p className="text-xs text-muted-foreground">
              Redirecting to login page...
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <h1 className="text-2xl font-bold text-foreground">Signing in...</h1>
            <div className="flex justify-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
            <p className="text-sm text-muted-foreground">
              Please wait while we complete your authentication
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
