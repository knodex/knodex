// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstanceChildResources } from './InstanceChildResources';
import type { ChildResourceResponse } from '@/types/rgd';

vi.mock('@/hooks/useInstances', () => ({
  useInstanceChildren: vi.fn(),
}));

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
};

const defaultProps = {
  namespace: 'default',
  kind: 'TestPodPair',
  name: 'demo-app',
};

const mockResponse: ChildResourceResponse = {
  instanceName: 'demo-app',
  instanceNamespace: 'default',
  instanceKind: 'TestPodPair',
  totalCount: 3,
  groups: [
    {
      nodeId: 'frontend',
      kind: 'Pod',
      apiVersion: 'v1',
      count: 2,
      readyCount: 2,
      health: 'Healthy',
      resources: [
        {
          name: 'frontend-pod-1',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'frontend',
          health: 'Healthy',
          phase: 'Running',
          createdAt: new Date().toISOString(),
        },
        {
          name: 'frontend-pod-2',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'frontend',
          health: 'Healthy',
          phase: 'Running',
          createdAt: new Date().toISOString(),
        },
      ],
    },
    {
      nodeId: 'backend',
      kind: 'Pod',
      apiVersion: 'v1',
      count: 1,
      readyCount: 0,
      health: 'Unhealthy',
      resources: [
        {
          name: 'backend-pod-1',
          namespace: 'default',
          kind: 'Pod',
          apiVersion: 'v1',
          nodeId: 'backend',
          health: 'Unhealthy',
          phase: 'Failed',
          createdAt: new Date().toISOString(),
        },
      ],
    },
  ],
};

describe('InstanceChildResources', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
  });

  it('renders loading state', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);
    expect(screen.getByText('Discovering child resources...')).toBeInTheDocument();
  });

  it('renders error state', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('API failure'),
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);
    expect(screen.getByText(/Failed to load child resources/)).toBeInTheDocument();
  });

  it('renders empty state when no children', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: { ...mockResponse, totalCount: 0, groups: [] },
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);
    expect(screen.getByText('No child resources found for this instance.')).toBeInTheDocument();
  });

  it('renders resource groups with node-id names', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    expect(screen.getByText('frontend')).toBeInTheDocument();
    expect(screen.getByText('backend')).toBeInTheDocument();
  });

  it('shows ready count per group', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);
    expect(screen.getByText('2/2 ready')).toBeInTheDocument();
    expect(screen.getByText('0/1 ready')).toBeInTheDocument();
  });

  it('expands group to show individual resources', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    // Resources not visible initially
    expect(screen.queryByText('frontend-pod-1')).not.toBeInTheDocument();

    // Click to expand
    const frontendGroup = screen.getByText('frontend').closest('button')!;
    await userEvent.click(frontendGroup);

    // Resources now visible
    expect(screen.getByText('frontend-pod-1')).toBeInTheDocument();
    expect(screen.getByText('frontend-pod-2')).toBeInTheDocument();
  });

  it('passes correct params to useInstanceChildren', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    expect(useInstanceChildren).toHaveBeenCalledWith('default', 'TestPodPair', 'demo-app');
  });

  // --- STORY-421 Task 10: Cluster badge and unreachable banner tests ---

  it('renders cluster badge when resource has cluster field', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    const responseWithCluster: ChildResourceResponse = {
      ...mockResponse,
      groups: [
        {
          nodeId: 'frontend',
          kind: 'Pod',
          apiVersion: 'v1',
          count: 1,
          readyCount: 1,
          health: 'Healthy',
          resources: [
            {
              name: 'frontend-pod-1',
              namespace: 'default',
              kind: 'Pod',
              apiVersion: 'v1',
              nodeId: 'frontend',
              health: 'Healthy',
              phase: 'Running',
              createdAt: new Date().toISOString(),
              cluster: 'prod-eu-west',
            },
          ],
        },
      ],
    };
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: responseWithCluster,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    // Expand group to see resources
    const groupButton = screen.getByText('frontend').closest('button')!;
    await userEvent.click(groupButton);

    expect(screen.getByText('prod-eu-west')).toBeInTheDocument();
  });

  it('does not render cluster badge when cluster is empty', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    const responseNoCluster: ChildResourceResponse = {
      ...mockResponse,
      groups: [
        {
          nodeId: 'frontend',
          kind: 'Pod',
          apiVersion: 'v1',
          count: 1,
          readyCount: 1,
          health: 'Healthy',
          resources: [
            {
              name: 'frontend-pod-1',
              namespace: 'default',
              kind: 'Pod',
              apiVersion: 'v1',
              nodeId: 'frontend',
              health: 'Healthy',
              phase: 'Running',
              createdAt: new Date().toISOString(),
              // No cluster field
            },
          ],
        },
      ],
    };
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: responseNoCluster,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    const groupButton = screen.getByText('frontend').closest('button')!;
    await userEvent.click(groupButton);

    // Should show pod name but no cluster badge
    expect(screen.getByText('frontend-pod-1')).toBeInTheDocument();
    expect(screen.queryByText('prod-eu-west')).not.toBeInTheDocument();
  });

  it('renders cluster badge with unreachable styling', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    const responseUnreachable: ChildResourceResponse = {
      ...mockResponse,
      clusterUnreachable: true,
      unreachableClusters: ['prod-us-east'],
      groups: [
        {
          nodeId: 'frontend',
          kind: 'Pod',
          apiVersion: 'v1',
          count: 1,
          readyCount: 0,
          health: 'Unknown',
          resources: [
            {
              name: 'frontend-pod-1',
              namespace: 'default',
              kind: 'Pod',
              apiVersion: 'v1',
              nodeId: 'frontend',
              health: 'Unknown',
              createdAt: new Date().toISOString(),
              cluster: 'prod-us-east',
              clusterStatus: 'unreachable',
            },
          ],
        },
      ],
    };
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: responseUnreachable,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    const groupButton = screen.getByText('frontend').closest('button')!;
    await userEvent.click(groupButton);

    expect(screen.getByText('prod-us-east (unreachable)')).toBeInTheDocument();
  });

  it('renders unreachable cluster banner when clusterUnreachable is true', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: {
        ...mockResponse,
        clusterUnreachable: true,
        unreachableClusters: ['prod-us-east'],
      },
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    expect(screen.getByText(/prod-us-east.*temporarily unreachable.*showing last known data/)).toBeInTheDocument();
  });

  it('does not render unreachable banner when clusterUnreachable is false', async () => {
    const { useInstanceChildren } = await import('@/hooks/useInstances');
    vi.mocked(useInstanceChildren).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<InstanceChildResources {...defaultProps} />);

    expect(screen.queryByText(/temporarily unreachable/)).not.toBeInTheDocument();
  });
});
