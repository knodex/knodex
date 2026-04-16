// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TopBar } from './TopBar';

// Mock useAuth hook
vi.mock('@/hooks/useAuth', () => ({
  useAuth: vi.fn(() => ({
    logout: vi.fn(),
    user: { email: 'test@example.com' },
  })),
  useCurrentProject: vi.fn(() => null),
}));

// Mock useTheme hook
vi.mock('@/hooks/useTheme', () => ({
  useTheme: vi.fn(() => ({
    isDark: false,
    toggleTheme: vi.fn(),
  })),
}));

// Mock useSettings hook
const mockUseSettings = vi.fn();
vi.mock('@/hooks/useSettings', () => ({
  useSettings: (...args: unknown[]) => mockUseSettings(...args),
}));

// Mock useProjects (used by ProjectSelector)
vi.mock('@/hooks/useProjects', () => ({
  useProjects: vi.fn(() => ({
    data: { items: [] },
    isLoading: false,
  })),
}));

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderTopBar() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <MemoryRouter>
        <TopBar />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('TopBar - Organization Name', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders org name when settings returns non-default organization', () => {
    mockUseSettings.mockReturnValue({
      data: { organization: 'acme' },
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    const orgElement = screen.getByTestId('org-name');
    expect(orgElement).toBeInTheDocument();
    expect(orgElement).toHaveTextContent('acme');
  });

  it('hides org name when organization is "default"', () => {
    mockUseSettings.mockReturnValue({
      data: { organization: 'default' },
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('hides org name when settings fetch fails (graceful degradation)', () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    });

    renderTopBar();

    // Header renders without org name - no error shown
    expect(screen.getByLabelText('Open navigation menu')).toBeInTheDocument();
    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('hides org name when settings is still loading', () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    });

    renderTopBar();

    expect(screen.getByLabelText('Open navigation menu')).toBeInTheDocument();
    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('hides org name when organization is empty string', () => {
    mockUseSettings.mockReturnValue({
      data: { organization: '' },
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('hides org name when organization field is null at runtime', () => {
    mockUseSettings.mockReturnValue({
      data: { organization: null },
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('hides org name when organization field is missing from response', () => {
    mockUseSettings.mockReturnValue({
      data: {},
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    expect(screen.queryByTestId('org-name')).not.toBeInTheDocument();
  });

  it('renders long org name with truncation classes', () => {
    const longName = 'my-very-long-organization-name-for-acme-corp';
    mockUseSettings.mockReturnValue({
      data: { organization: longName },
      isLoading: false,
      isError: false,
    });

    renderTopBar();

    const orgElement = screen.getByTestId('org-name');
    expect(orgElement).toHaveTextContent(longName);
    expect(orgElement).toHaveClass('truncate');
    expect(orgElement).toHaveClass('max-w-[160px]');
  });
});
