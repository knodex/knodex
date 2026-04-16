// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstanceAddOns } from './InstanceAddOns';

vi.mock('@/hooks/useRGDs', () => ({
  useRGDList: vi.fn(),
}));

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
};

const defaultProps = {
  kind: 'WebApp',
  instanceName: 'my-instance',
  instanceNamespace: 'production',
};

describe('InstanceAddOns', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const { useRGDList } = await import('@/hooks/useRGDs');
    vi.mocked(useRGDList).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);
  });

  describe('Query parameters', () => {
    it('calls useRGDList with extendsKind and pageSize 100', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');

      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(useRGDList).toHaveBeenCalledWith({ extendsKind: 'WebApp', pageSize: 100 });
    });
  });

  describe('Loading state', () => {
    it('shows loading spinner and text', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
      } as any);

      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('Loading add-ons...')).toBeInTheDocument();
    });
  });

  describe('Error state', () => {
    it('shows error message with the kind name', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Network error'),
      } as any);

      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      // Error message is rendered as a <span> inside the card
      const errorText = screen.getByText((content) =>
        content.includes('Failed to load add-ons for WebApp')
      );
      expect(errorText).toBeInTheDocument();
    });
  });

  describe('Empty state', () => {
    it('shows empty message when data has no items', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: [], totalCount: 0 },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.queryByText('Deploy on this instance')).not.toBeInTheDocument();
      expect(screen.getByText('No add-ons available')).toBeInTheDocument();
    });
  });

  describe('Renders add-ons', () => {
    const mockAddons = [
      {
        name: 'redis-addon',
        title: 'Redis Cache',
        description: 'Add Redis caching to your instance',
        tags: ['database', 'cache'],
        namespace: 'default',
        category: 'storage',
        labels: {},
        instances: 0,
        createdAt: '2024-01-01T00:00:00Z',
        updatedAt: '2024-01-01T00:00:00Z',
      },
    ];

    beforeEach(async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: mockAddons, totalCount: 1 },
        isLoading: false,
        error: null,
      } as any);
    });

    it('renders section heading', () => {
      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('Deploy on this instance')).toBeInTheDocument();
    });

    it('renders add-on title', () => {
      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('Redis Cache')).toBeInTheDocument();
    });

    it('renders add-on description', () => {
      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('Add Redis caching to your instance')).toBeInTheDocument();
    });

    it('renders add-on tags', () => {
      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('database')).toBeInTheDocument();
      expect(screen.getByText('cache')).toBeInTheDocument();
    });

    it('deploy link links to catalog page for addon', () => {
      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      const deployLink = screen.getByRole('link', { name: /deploy/i });
      expect(deployLink).toHaveAttribute(
        'href',
        `/catalog/${encodeURIComponent('redis-addon')}`
      );
    });

    it('falls back to addon.name when title is absent', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: {
          items: [{ ...mockAddons[0], title: undefined }],
          totalCount: 1,
        },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(<InstanceAddOns {...defaultProps} />);

      expect(screen.getByText('redis-addon')).toBeInTheDocument();
    });
  });
});
