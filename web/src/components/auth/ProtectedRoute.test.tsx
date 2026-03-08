// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';

// Mock the useAuth hook
vi.mock('@/hooks/useAuth', () => ({
  useIsAuthenticated: vi.fn(),
}));

// Mock the user store
const mockLogout = vi.fn();
const mockIsTokenExpired = vi.fn();

vi.mock('@/stores/userStore', () => ({
  useUserStore: vi.fn((selector) =>
    selector({
      logout: mockLogout,
      isTokenExpired: mockIsTokenExpired,
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
      login: vi.fn(),
      setCurrentProject: vi.fn(),
    })
  ),
}));

const { useIsAuthenticated } = await import('@/hooks/useAuth');

describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTokenExpired.mockReturnValue(false);
  });

  it('renders children when authenticated and token is valid', () => {
    vi.mocked(useIsAuthenticated).mockReturnValue(true);

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Protected Content')).toBeInTheDocument();
    expect(screen.queryByText('Login Page')).not.toBeInTheDocument();
  });

  it('redirects to login when not authenticated', () => {
    vi.mocked(useIsAuthenticated).mockReturnValue(false);

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });

  it('redirects to login when token is expired', () => {
    vi.mocked(useIsAuthenticated).mockReturnValue(true);
    mockIsTokenExpired.mockReturnValue(true);

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByText('Login Page')).toBeInTheDocument();
    expect(mockLogout).toHaveBeenCalled();
  });

  it('saves the intended location for redirect after login', () => {
    vi.mocked(useIsAuthenticated).mockReturnValue(false);

    render(
      <MemoryRouter initialEntries={['/dashboard/settings']}>
        <Routes>
          <Route
            path="/dashboard/settings"
            element={
              <ProtectedRoute>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });

  it('handles nested routes correctly', () => {
    vi.mocked(useIsAuthenticated).mockReturnValue(true);

    render(
      <MemoryRouter initialEntries={['/app/nested/route']}>
        <Routes>
          <Route
            path="/app/*"
            element={
              <ProtectedRoute>
                <Routes>
                  <Route path="nested/route" element={<div>Nested Content</div>} />
                </Routes>
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Nested Content')).toBeInTheDocument();
    expect(screen.queryByText('Login Page')).not.toBeInTheDocument();
  });
});
