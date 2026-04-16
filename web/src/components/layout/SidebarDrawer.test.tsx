// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SidebarDrawer } from './SidebarDrawer';

// Mock hooks used by SidebarNav
vi.mock('@/hooks/useRGDs', () => ({
  useRGDCount: () => ({ data: { count: 3 } }),
  useRGDList: () => ({ data: undefined }),
}));

vi.mock('@/hooks/useInstances', () => ({
  useInstanceCount: () => ({ data: { count: 5 } }),
  useInstanceList: () => ({ data: undefined }),
}));

vi.mock('@/hooks/useCompliance', () => ({
  useViolationCount: () => ({ data: 0 }),
  isEnterprise: () => false,
}));

vi.mock('@/hooks/useCategories', () => ({
  useCategoriesEnabled: () => ({ enabled: false, isLoading: false, categories: [] }),
}));

vi.mock('@/hooks/useCanI', () => ({
  useCanI: () => ({ allowed: false }),
}));

vi.mock('@/hooks/useAuth', () => ({
  useCurrentProject: () => null,
}));

vi.mock('@/hooks/useProjects', () => ({
  useProjects: () => ({ data: undefined }),
}));

vi.mock('@/lib/route-preloads', () => ({
  routePreloads: {},
}));

vi.mock('@/lib/icons', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/icons')>();
  return {
    ...actual,
    getLucideIcon: () => () => null,
  };
});

function renderDrawer(open: boolean, onOpenChange = vi.fn()) {
  return {
    onOpenChange,
    ...render(
      <MemoryRouter>
        <SidebarDrawer open={open} onOpenChange={onOpenChange} />
      </MemoryRouter>
    ),
  };
}

describe('SidebarDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders navigation content when open', () => {
    renderDrawer(true);

    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('Projects')).toBeInTheDocument();
  });

  it('does not render navigation content when closed', () => {
    renderDrawer(false);

    expect(screen.queryByText('Catalog')).not.toBeInTheDocument();
  });

  it('has accessible title for screen readers', () => {
    renderDrawer(true);

    expect(screen.getByText('Navigation menu')).toBeInTheDocument();
  });

  it('closes on nav item click', () => {
    const onOpenChange = vi.fn();
    renderDrawer(true, onOpenChange);

    fireEvent.click(screen.getByText('Catalog'));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('closes on Escape key', () => {
    const onOpenChange = vi.fn();
    renderDrawer(true, onOpenChange);

    fireEvent.keyDown(document, { key: 'Escape' });

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
