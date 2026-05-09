// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: [], localLoginEnabled: true });

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
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: mockProviders, localLoginEnabled: true });

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
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: mockProviders, localLoginEnabled: true });

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
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: [], localLoginEnabled: true });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('local-admin-form')).toBeInTheDocument();
    });
  });

  it('renders OIDC provider button when OIDC providers exist', async () => {
    const mockProviders = [{ name: 'google', display_name: 'Google', enabled: true }];
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: mockProviders, localLoginEnabled: true });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Google/i })).toBeInTheDocument();
    });
  });

  it('shows Administrator Login heading when no OIDC providers', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: [], localLoginEnabled: true });

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
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({ providers: mockProviders, localLoginEnabled: true });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      // The component renders "Or" divider and the OIDC button when both methods are available
      expect(screen.getByText('Or')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Google/i })).toBeInTheDocument();
    });
  });

  it('shows an error state and hides login forms when provider fetch fails', async () => {
    vi.mocked(authApi.getOIDCProviders).mockRejectedValue(
      new Error('Failed to fetch providers')
    );

    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    // F10 regression guard: previously this rendered the local form on API
    // error (defaulting to localLoginEnabled=true), which trained users on
    // SSO-only deployments to ignore the resulting 403/404. Now we render an
    // explicit error state and suppress the form entirely.
    await waitFor(() => {
      expect(screen.getByText(/unable to load login options/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId('local-admin-form')).not.toBeInTheDocument();
    expect(consoleSpy).toHaveBeenCalledWith(
      '[LoginPage] Failed to fetch OIDC providers:',
      expect.any(Error)
    );

    consoleSpy.mockRestore();
  });

  it('shows the no-login-methods sentinel when both local login and OIDC are disabled', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({
      providers: [],
      localLoginEnabled: false,
    });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/no login methods available/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId('local-admin-form')).not.toBeInTheDocument();
  });

  it('hides local login form when localLoginEnabled is false', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({
      providers: [{ name: 'knodex-cloud', display_name: 'Knodex Cloud', enabled: true }],
      localLoginEnabled: false,
    });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.queryByTestId('local-admin-form')).not.toBeInTheDocument();
    });
  });

  it('shows local login form when localLoginEnabled is true', async () => {
    vi.mocked(authApi.getOIDCProviders).mockResolvedValue({
      providers: [],
      localLoginEnabled: true,
    });

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('local-admin-form')).toBeInTheDocument();
    });
  });
});
