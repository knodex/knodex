// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OIDCButton } from './OIDCButton';
import * as authApi from '@/api/auth';

// Mock the auth API
vi.mock('@/api/auth', () => ({
  initiateOIDCLogin: vi.fn(),
}));

describe('OIDCButton', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with provider display name', () => {
    render(<OIDCButton provider="google" displayName="Google" />);

    expect(
      screen.getByRole('button', { name: /continue with google/i })
    ).toBeInTheDocument();
  });

  it('renders Google provider with icon', () => {
    const { container } = render(<OIDCButton provider="google" displayName="Google" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Google');
    // Google icon is an SVG element
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders Keycloak provider with icon', () => {
    const { container } = render(<OIDCButton provider="keycloak" displayName="Keycloak" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Keycloak');
    // Keycloak uses KeyRound icon from lucide-react (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders Auth0 provider with icon', () => {
    const { container } = render(<OIDCButton provider="auth0" displayName="Auth0" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Auth0');
    // Auth0 uses Lock icon from lucide-react (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders Entra ID provider with icon', () => {
    const { container } = render(<OIDCButton provider="entraid" displayName="Entra ID" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Entra ID');
    // Entra ID uses custom EntraIDIcon (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders Azure AD provider with Entra ID icon (backward compatibility)', () => {
    const { container } = render(<OIDCButton provider="azuread" displayName="Entra ID" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Entra ID');
    // Azure AD now uses EntraIDIcon (SVG) for backward compatibility
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders Okta provider with icon', () => {
    const { container } = render(<OIDCButton provider="okta" displayName="Okta" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Okta');
    // Okta uses Shield icon from lucide-react (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders unknown provider with default lock icon', () => {
    const { container } = render(<OIDCButton provider="unknown-provider" displayName="Custom SSO" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Custom SSO');
    // Unknown providers use Lock icon from lucide-react (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('initiates OIDC login when clicked', async () => {
    const user = userEvent.setup();
    render(<OIDCButton provider="google" displayName="Google" />);

    const button = screen.getByRole('button', { name: /continue with google/i });
    await user.click(button);

    expect(authApi.initiateOIDCLogin).toHaveBeenCalledWith('google');
  });

  it('can be disabled', async () => {
    const user = userEvent.setup();
    render(<OIDCButton provider="google" displayName="Google" disabled />);

    const button = screen.getByRole('button', { name: /continue with google/i });
    expect(button).toBeDisabled();

    await user.click(button);
    expect(authApi.initiateOIDCLogin).not.toHaveBeenCalled();
  });

  it('renders Knodex Cloud provider with Knodex logo image', () => {
    const { container } = render(<OIDCButton provider="knodex-cloud" displayName="Knodex Cloud" />);
    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Sign in with Knodex Cloud');
    expect(container.querySelector('img[alt="Knodex"]')).toBeInTheDocument();
  });

  it('renders knodex provider (no dash) with Knodex logo image', () => {
    const { container } = render(<OIDCButton provider="knodex" displayName="Knodex Cloud" />);
    expect(container.querySelector('img[alt="Knodex"]')).toBeInTheDocument();
  });

  it('is case-insensitive for provider names', () => {
    const { container } = render(<OIDCButton provider="GOOGLE" displayName="Google" />);

    const button = screen.getByRole('button');
    expect(button.textContent).toContain('Continue with Google');
    // Should still render the Google icon (SVG)
    expect(container.querySelector('svg')).toBeInTheDocument();
  });
});
