// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SidebarDrawer } from './SidebarDrawer';

// Mock hooks used by SidebarNav
vi.mock('@/hooks/useRGDs', () => ({
  useRGDList: () => ({ data: undefined }),
}));

vi.mock('@/hooks/useCompliance', () => ({
  useViolationCount: () => ({ data: 0 }),
  useComplianceSummary: () => ({ data: undefined }),
  isEnterprise: () => false,
}));

vi.mock('@/hooks/useCategories', () => ({
  useCategoriesEnabled: () => ({ enabled: false, isLoading: false, categories: [] }),
}));

vi.mock('@/hooks/useCanI', () => ({
  useCanI: () => ({ allowed: false }),
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

  // Guards against accidentally re-adding count badges on the main Catalog
  // and Instances nav entries. Badges render as a sibling <span> with an
  // aria-label like "5 items"; absence of any such span on these links proves
  // no badge is rendered.
  it('does not render count badges on Catalog or Instances nav items', () => {
    renderDrawer(true);

    const catalogLink = screen.getByRole('link', { name: 'Catalog' });
    const instancesLink = screen.getByRole('link', { name: 'Instances' });

    expect(catalogLink.querySelector('span[aria-label$="items"]')).toBeNull();
    expect(instancesLink.querySelector('span[aria-label$="items"]')).toBeNull();
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
