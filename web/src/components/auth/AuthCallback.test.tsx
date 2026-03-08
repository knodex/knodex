// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthCallback } from './AuthCallback';
import type { UserState } from '@/stores/userStore';

// Mock useNavigate to verify navigation calls
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock exchangeAuthCode
const mockExchangeAuthCode = vi.fn();
vi.mock('@/api/auth', () => ({
  exchangeAuthCode: (...args: unknown[]) => mockExchangeAuthCode(...args),
}));

// Mock the user store
const mockLogin = vi.fn();
const mockLogout = vi.fn();
const mockIsTokenExpired = vi.fn(() => false);

const mockState = {
  login: mockLogin,
  logout: mockLogout,
  user: null,
  projects: [],
  roles: {},
  currentProject: null,
  isAuthenticated: false,
  groups: [],
  issuer: null,
  casbinRoles: [],
  permissions: {},
  tokenExp: null,
  tokenIat: null,
  setCurrentProject: vi.fn(),
  isTokenExpired: mockIsTokenExpired,
};

vi.mock('@/stores/userStore', () => ({
  useUserStore: <T,>(selector: (state: UserState) => T) => selector(mockState),
}));

describe('AuthCallback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('displays error when error parameter is present', () => {
    const errorMessage = 'Authentication failed: invalid provider';

    render(
      <MemoryRouter initialEntries={[`/?error=${encodeURIComponent(errorMessage)}`]}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Authentication Failed')).toBeInTheDocument();
    expect(screen.getByText(errorMessage)).toBeInTheDocument();
    expect(screen.getByText('Redirecting to login page...')).toBeInTheDocument();
  });

  it('displays error when no code is provided', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Authentication Failed')).toBeInTheDocument();
    expect(
      screen.getByText('No authorization code received from authentication provider')
    ).toBeInTheDocument();
    expect(screen.getByText('Redirecting to login page...')).toBeInTheDocument();
  });

  it('decodes URL-encoded error messages', () => {
    const errorMessage = 'User denied access';
    const encodedError = encodeURIComponent(errorMessage);

    render(
      <MemoryRouter initialEntries={[`/?error=${encodedError}`]}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText(errorMessage)).toBeInTheDocument();
  });

  it('does not call exchangeAuthCode when error parameter is present', () => {
    render(
      <MemoryRouter initialEntries={['/?error=Authentication%20failed']}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(mockExchangeAuthCode).not.toHaveBeenCalled();
  });

  it('displays icon in header', () => {
    render(
      <MemoryRouter initialEntries={['/?error=test']}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    // Icon should be present (Boxes lucide icon)
    const icon = document.querySelector('.lucide-boxes');
    expect(icon).toBeInTheDocument();
  });

  it('exchanges code for token and navigates to dashboard with replace', async () => {
    const testCode = 'opaque-auth-code-abc123';
    const mockResp = {
      user: { id: 'user-123', email: 'test@example.com', displayName: 'Test' },
      expiresAt: '2099-01-01T00:00:00Z',
    };

    // Mock exchangeAuthCode to return user info (cookie is set by server)
    mockExchangeAuthCode.mockResolvedValue(mockResp);

    vi.useRealTimers(); // Need real timers for async operations

    render(
      <MemoryRouter initialEntries={[`/?code=${testCode}`]}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
        </Routes>
      </MemoryRouter>
    );

    // exchangeAuthCode should be called with the opaque code
    await waitFor(() => {
      expect(mockExchangeAuthCode).toHaveBeenCalledWith(testCode);
    });

    // login should be called with user info and expiresAt
    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith(mockResp.user, mockResp.expiresAt);
    });

    // Should navigate to dashboard with replace:true (prevents back-button to callback)
    expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true });
  });

  it('shows error and redirects to login when code exchange fails', async () => {
    const testCode = 'expired-code';

    // Mock exchangeAuthCode to reject (expired/invalid code)
    mockExchangeAuthCode.mockRejectedValue(new Error('code expired'));

    vi.useRealTimers();

    render(
      <MemoryRouter initialEntries={[`/?code=${testCode}`]}>
        <Routes>
          <Route path="/" element={<AuthCallback />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    // Should show exchange error
    await waitFor(() => {
      expect(
        screen.getByText('Failed to complete authentication. The code may have expired.')
      ).toBeInTheDocument();
    });
  });
});
