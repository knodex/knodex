import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import * as authApi from '@/api/auth';

// Mock the auth API
vi.mock('@/api/auth', () => ({
  getOIDCProviders: vi.fn(),
  localAdminLogin: vi.fn(),
}));

// Mock useAuth hook
vi.mock('@/hooks/useAuth', () => ({
  useIsAuthenticated: vi.fn(),
}));

// Mock child components
vi.mock('./OIDCButton', () => ({
  OIDCButton: ({ provider, displayName }: { provider: string; displayName: string }) => (
    <button data-testid={`oidc-${provider}`}>Continue with {displayName}</button>
  ),
}));

vi.mock('./LocalAdminForm', () => ({
  LocalAdminForm: ({ onSuccess }: { onSuccess?: () => void }) => (
    <form data-testid="local-admin-form">
      <button onClick={onSuccess}>Sign in</button>
    </form>
  ),
}));

const { useIsAuthenticated } = await import('@/hooks/useAuth');

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useIsAuthenticated).mockReturnValue(false);
  });

  it('renders the login page header', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    expect(screen.getByText('Knodex')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes Native Self Service Platform')).toBeInTheDocument();
  });

  it('displays OIDC providers when available', async () => {
    const mockProviders = [
      { name: 'google', display_name: 'Google', enabled: true },
      { name: 'keycloak', display_name: 'Keycloak', enabled: true },
    ];
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue(mockProviders);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('oidc-google')).toBeInTheDocument();
      expect(screen.getByTestId('oidc-keycloak')).toBeInTheDocument();
    });
  });

  it('filters out disabled OIDC providers', async () => {
    const mockProviders = [
      { name: 'google', display_name: 'Google', enabled: true },
      { name: 'keycloak', display_name: 'Keycloak', enabled: false },
    ];
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue(mockProviders);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('oidc-google')).toBeInTheDocument();
      expect(screen.queryByTestId('oidc-keycloak')).not.toBeInTheDocument();
    });
  });

  it('displays local admin form', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('local-admin-form')).toBeInTheDocument();
    });
  });

  it('shows SSO section heading when OIDC providers exist', async () => {
    const mockProviders = [{ name: 'google', display_name: 'Google', enabled: true }];
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue(mockProviders);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Single Sign-On')).toBeInTheDocument();
    });
  });

  it('shows Administrator Login heading when no OIDC providers', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Administrator Login')).toBeInTheDocument();
    });
  });

  it('shows divider when both OIDC and local admin are available', async () => {
    const mockProviders = [{ name: 'google', display_name: 'Google', enabled: true }];
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue(mockProviders);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      // The component renders "Or" divider and "Single Sign-On" header when OIDC providers are available
      expect(screen.getByText('Or')).toBeInTheDocument();
      expect(screen.getByText('Single Sign-On')).toBeInTheDocument();
    });
  });

  it('handles provider fetch errors gracefully', async () => {
    vi.mocked(authApi.getOIDCProviders).mockRejectedValue(
      new Error('Failed to fetch providers')
    );

    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('local-admin-form')).toBeInTheDocument();
      expect(consoleSpy).toHaveBeenCalledWith(
        '[LoginPage] Failed to fetch OIDC providers:',
        expect.any(Error)
      );
    });

    consoleSpy.mockRestore();
  });

  it('displays Terms of Service and Privacy Policy text', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue([]);

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    expect(
      screen.getByText(/by signing in, you agree to our terms of service and privacy policy/i)
    ).toBeInTheDocument();
  });
});
