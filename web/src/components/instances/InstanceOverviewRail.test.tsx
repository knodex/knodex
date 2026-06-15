// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InstanceResourcesSummaryCard, InstanceRecentActivityCard } from './InstanceOverviewRail';
import type { ChildResourceGroup } from '@/types/rgd';
import type { KubernetesEvent } from '@/types/history';

const makeGroup = (overrides: Partial<ChildResourceGroup>): ChildResourceGroup => ({
  nodeId: 'spokeVnet',
  kind: 'VirtualNetwork',
  apiVersion: 'network.azure.com/v1api20240101',
  count: 1,
  readyCount: 1,
  health: 'Healthy',
  resources: [],
  ...overrides,
});

describe('InstanceResourcesSummaryCard', () => {
  it('renders nothing when there are no groups', () => {
    const { container } = render(
      <InstanceResourcesSummaryCard groups={[]} totalCount={0} onViewAll={vi.fn()} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders group rows with ready counts and sorts failing groups first', () => {
    const groups = [
      makeGroup({ nodeId: 'spokeVnet', health: 'Healthy' }),
      makeGroup({ nodeId: 'peeringFromHub', kind: 'VNetPeering', count: 2, readyCount: 0, health: 'Unhealthy' }),
    ];
    render(<InstanceResourcesSummaryCard groups={groups} totalCount={3} onViewAll={vi.fn()} />);

    const card = screen.getByTestId('resources-summary-card');
    expect(within(card).getByText('1 failing')).toBeInTheDocument();

    const rows = within(card).getAllByText(/spokeVnet|peeringFromHub/);
    // failing group surfaces first
    expect(rows[0]).toHaveTextContent('peeringFromHub');
    expect(within(card).getByText('0/2')).toBeInTheDocument();
    expect(within(card).getByText('1/1')).toBeInTheDocument();
  });

  it('navigates to the resources tab from the view-all footer', async () => {
    const onViewAll = vi.fn();
    render(
      <InstanceResourcesSummaryCard
        groups={[makeGroup({})]}
        totalCount={7}
        onViewAll={onViewAll}
      />
    );

    await userEvent.click(screen.getByText('All 7 resources →'));
    expect(onViewAll).toHaveBeenCalledOnce();
  });
});

const makeEvent = (overrides: Partial<KubernetesEvent>): KubernetesEvent => ({
  lastSeen: '2024-01-15T11:00:00Z',
  type: 'Normal',
  reason: 'BeginCreateOrUpdate',
  object: 'VirtualNetwork/vnet-demo',
  message: 'Successfully sent resource to Azure',
  ...overrides,
});

describe('InstanceRecentActivityCard', () => {
  it('renders nothing when there are no events', () => {
    const { container } = render(<InstanceRecentActivityCard events={[]} onViewAll={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the most recent events first, capped at three', () => {
    const events = [
      makeEvent({ reason: 'Oldest', lastSeen: '2024-01-15T08:00:00Z' }),
      makeEvent({ reason: 'Newest', lastSeen: '2024-01-15T11:30:00Z' }),
      makeEvent({ reason: 'Middle', lastSeen: '2024-01-15T10:00:00Z' }),
      makeEvent({ reason: 'Older', lastSeen: '2024-01-15T09:00:00Z' }),
    ];
    render(<InstanceRecentActivityCard events={events} onViewAll={vi.fn()} />);

    const card = screen.getByTestId('recent-activity-card');
    expect(within(card).getByText('Newest')).toBeInTheDocument();
    expect(within(card).getByText('Middle')).toBeInTheDocument();
    expect(within(card).getByText('Older')).toBeInTheDocument();
    expect(within(card).queryByText('Oldest')).not.toBeInTheDocument();
    expect(within(card).getByText('All 4 events →')).toBeInTheDocument();
  });

  it('renders event message and object', () => {
    render(<InstanceRecentActivityCard events={[makeEvent({})]} onViewAll={vi.fn()} />);

    expect(screen.getByText('Successfully sent resource to Azure')).toBeInTheDocument();
    expect(screen.getByText('VirtualNetwork/vnet-demo')).toBeInTheDocument();
  });

  it('navigates to the events tab from the view-all footer', async () => {
    const onViewAll = vi.fn();
    render(<InstanceRecentActivityCard events={[makeEvent({})]} onViewAll={onViewAll} />);

    await userEvent.click(screen.getByText('All 1 event →'));
    expect(onViewAll).toHaveBeenCalledOnce();
  });
});
