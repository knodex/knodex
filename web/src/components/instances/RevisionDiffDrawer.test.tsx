// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RevisionDiffDrawer } from './RevisionDiffDrawer';

vi.mock('@/hooks/useRGDs', () => ({
  useRGDRevisionDiff: vi.fn(),
  useRGDRevision: vi.fn(),
}));

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

const renderDrawer = (props: Partial<React.ComponentProps<typeof RevisionDiffDrawer>> = {}) => {
  const defaultProps = {
    rgdName: 'test-rgd',
    currentRevision: 3,
    open: true,
    onOpenChange: vi.fn(),
    ...props,
  };
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <MemoryRouter>
        <RevisionDiffDrawer {...defaultProps} />
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('RevisionDiffDrawer', () => {
  beforeEach(async () => {
    vi.clearAllMocks();

    const { useRGDRevisionDiff, useRGDRevision } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevisionDiff).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useRGDRevision).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);
  });

  it('renders diff content when currentRevision > 1 and diff data is available', async () => {
    const { useRGDRevisionDiff, useRGDRevision } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevisionDiff).mockReturnValue({
      data: {
        rgdName: 'test-rgd',
        rev1: 2,
        rev2: 3,
        added: [{ path: 'spec.newField', newValue: 'hello' }],
        removed: [],
        modified: [{ path: 'spec.replicas', oldValue: 2, newValue: 3 }],
        identical: false,
      },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useRGDRevision).mockImplementation((name: string, rev: number | null) => {
      if (rev === 2) {
        return {
          data: { revisionNumber: 2, rgdName: name, snapshot: { spec: { replicas: 2 } } },
          isLoading: false,
          error: null,
        } as any;
      }
      if (rev === 3) {
        return {
          data: { revisionNumber: 3, rgdName: name, snapshot: { spec: { replicas: 3, newField: 'hello' } } },
          isLoading: false,
          error: null,
        } as any;
      }
      return { data: undefined, isLoading: false, error: null } as any;
    });

    renderDrawer({ currentRevision: 3 });

    expect(screen.getByText('Revision Changes')).toBeInTheDocument();
    expect(screen.getByText('Rev 2 → Rev 3')).toBeInTheDocument();
    expect(screen.getByText('1 added')).toBeInTheDocument();
    expect(screen.getByText('1 modified')).toBeInTheDocument();
  });

  it('renders initial revision notice when currentRevision is 1', async () => {
    const { useRGDRevision } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevision).mockImplementation((_name: string, rev: number | null) => {
      if (rev === 1) {
        return {
          data: {
            revisionNumber: 1,
            rgdName: 'test-rgd',
            snapshot: { spec: { replicas: 1 } },
          },
          isLoading: false,
          error: null,
        } as any;
      }
      return { data: undefined, isLoading: false, error: null } as any;
    });

    renderDrawer({ currentRevision: 1 });

    expect(screen.getByTestId('initial-revision-notice')).toBeInTheDocument();
    expect(screen.getByText(/initial revision/i)).toBeInTheDocument();
    expect(screen.getByText('Rev 1 (initial)')).toBeInTheDocument();
    expect(screen.getByTestId('snapshot-yaml')).toBeInTheDocument();
  });

  it('renders "Open in Revision Explorer" link with correct href', () => {
    renderDrawer({ rgdName: 'my-rgd' });

    const link = screen.getByTestId('revision-explorer-link');
    expect(link).toHaveAttribute('href', '/catalog/my-rgd?tab=revisions');
    expect(link).toHaveTextContent('Open in Revision Explorer');
  });

  it('renders loading state', async () => {
    const { useRGDRevisionDiff, useRGDRevision } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevisionDiff).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    vi.mocked(useRGDRevision).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    renderDrawer();

    expect(screen.getByTestId('diff-loading')).toBeInTheDocument();
  });

  it('renders error state', async () => {
    const { useRGDRevisionDiff } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevisionDiff).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network failure'),
    } as any);

    renderDrawer();

    expect(screen.getByTestId('diff-error')).toBeInTheDocument();
    expect(screen.getByText(/Network failure/)).toBeInTheDocument();
  });

  it('calls onOpenChange(false) when "Open in Revision Explorer" is clicked', async () => {
    const { default: userEvent } = await import('@testing-library/user-event');
    const onOpenChange = vi.fn();

    renderDrawer({ onOpenChange });

    const link = screen.getByTestId('revision-explorer-link');
    await userEvent.click(link);

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('renders "No differences" when diff is identical', async () => {
    const { useRGDRevisionDiff, useRGDRevision } = await import('@/hooks/useRGDs');

    vi.mocked(useRGDRevisionDiff).mockReturnValue({
      data: {
        rgdName: 'test-rgd',
        rev1: 2,
        rev2: 3,
        added: [],
        removed: [],
        modified: [],
        identical: true,
      },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useRGDRevision).mockReturnValue({
      data: { revisionNumber: 2, rgdName: 'test-rgd', snapshot: { spec: {} } },
      isLoading: false,
      error: null,
    } as any);

    renderDrawer();

    expect(screen.getByText('No differences')).toBeInTheDocument();
  });
});
