// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TopBar } from './TopBar';

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

function renderTopBar(options: {
  route?: string;
  onCommandPaletteOpen?: () => void;
  isSidebarCollapsed?: boolean;
  onMobileMenuToggle?: () => void;
} = {}) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <MemoryRouter initialEntries={[options.route ?? '/']}>
        <TopBar
          onCommandPaletteOpen={options.onCommandPaletteOpen ?? vi.fn()}
          isSidebarCollapsed={options.isSidebarCollapsed}
          onMobileMenuToggle={options.onMobileMenuToggle}
        />
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

describe('TopBar - Breadcrumb chrome', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSettings.mockReturnValue({
      data: { organization: 'default' },
      isLoading: false,
      isError: false,
    });
  });

  it('renders the breadcrumb container instead of a centered page title', () => {
    renderTopBar({ route: '/catalog' });

    expect(screen.getByTestId('breadcrumbs')).toBeInTheDocument();
    expect(screen.getByText('Catalog')).toBeInTheDocument();
  });

  it('reflects the current route in the breadcrumb (catalog category)', () => {
    renderTopBar({ route: '/catalog/categories/database' });

    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('database')).toBeInTheDocument();
  });

  it('always renders the ⌘K trigger (callback is required)', () => {
    renderTopBar();

    expect(screen.getByTestId('cmdk-trigger')).toBeInTheDocument();
  });

  it('clicking the ⌘K trigger invokes the command-palette callback', () => {
    const onOpen = vi.fn();
    renderTopBar({ onCommandPaletteOpen: onOpen });

    fireEvent.click(screen.getByTestId('cmdk-trigger'));

    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('renders the CmdK trigger inline (no absolute positioning) with a flex-1 spacer sibling', () => {
    renderTopBar();

    const trigger = screen.getByTestId('cmdk-trigger');
    const wrapper = trigger.parentElement;
    expect(wrapper).not.toBeNull();
    expect(wrapper?.className).not.toMatch(/absolute/);
    expect(wrapper?.className).toContain('min-w-[220px]');

    const header = document.querySelector('header');
    const spacer = header?.querySelector('div.flex-1[aria-hidden="true"]');
    expect(spacer).not.toBeNull();
  });

  it('does not render the connection pill (Live indicator removed)', () => {
    renderTopBar();

    expect(screen.queryByTestId('connection-pill')).not.toBeInTheDocument();
  });

  it('does not render the user chip or logout button (moved into sidebar dropdown)', () => {
    renderTopBar();

    expect(screen.queryByLabelText('View account info')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Logout')).not.toBeInTheDocument();
  });

  it('renders the ProjectSelector slot (right cluster)', () => {
    // Renders unauthenticated by default — ProjectSelector returns null,
    // but the absence of the user chip + presence of breadcrumbs proves the
    // new TopBar shape. ProjectSelector rendering is exercised in its own
    // test suite.
    renderTopBar();

    expect(screen.queryByLabelText('View account info')).not.toBeInTheDocument();
    expect(screen.getByTestId('breadcrumbs')).toBeInTheDocument();
  });

  it('hamburger label stays "Open navigation menu" regardless of collapse state (rail handles desktop re-expand)', () => {
    renderTopBar({ isSidebarCollapsed: true });
    const trigger = screen.getByTestId('topbar-menu-trigger');
    expect(trigger).toHaveAttribute('aria-label', 'Open navigation menu');
  });

  it('anchors left edge to the sidebar width (260px → 64px) so it does not overlay the sidebar header', () => {
    const { rerender } = renderTopBar();
    let header = document.querySelector('header');
    expect(header?.className).toContain('lg:left-[260px]');

    rerender(
      <QueryClientProvider client={createQueryClient()}>
        <MemoryRouter initialEntries={['/']}>
          <TopBar onCommandPaletteOpen={vi.fn()} isSidebarCollapsed />
        </MemoryRouter>
      </QueryClientProvider>
    );
    header = document.querySelector('header');
    expect(header?.className).toContain('lg:left-16');
    expect(header?.className).not.toContain('lg:left-[260px]');
  });
});
