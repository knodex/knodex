// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { InstanceHeaderCard } from './InstanceHeaderCard';
import type { InstanceRollup } from './InstanceHeaderCard';
import type { Instance } from '@/types/rgd';

vi.mock('@/components/ui/rgd-icon', () => ({
  RGDIcon: () => <span data-testid="rgd-icon" />,
}));

const baseInstance: Instance = {
  name: 'my-instance',
  namespace: 'production',
  rgdName: 'web-app',
  rgdNamespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'WebApp',
  health: 'Healthy',
  conditions: [],
  createdAt: '2024-01-15T10:30:00Z',
  updatedAt: '2024-01-15T11:00:00Z',
  deploymentMode: 'direct',
};

const baseRollup: InstanceRollup = {
  conditionsPassing: 4,
  conditionsTotal: 4,
  resourcesReady: 6,
  resourcesTotal: 7,
  resourcesFailing: 1,
  eventsCount: 16,
  eventsWarnings: 0,
  lastReconciled: '2024-01-15T11:00:00Z',
};

const defaultProps = {
  instance: baseInstance,
  parentRGD: { description: 'A web application', lastIssuedRevision: 3, labels: {} },
  canReadRGD: true,
  kroState: 'ACTIVE',
  isGitOps: false,
  rollup: baseRollup,
  actions: <button type="button">Edit Spec</button>,
  onRevisionClick: vi.fn(),
  onSelectTab: vi.fn(),
};

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <MemoryRouter>{children}</MemoryRouter>
);

describe('InstanceHeaderCard', () => {
  it('renders instance name and RGD description', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('my-instance')).toBeInTheDocument();
    expect(screen.getByText('A web application')).toBeInTheDocument();
  });

  it('renders the health badge in the title row', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('Healthy')).toBeInTheDocument();
  });

  it('renders kind as a link to catalog', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    const kindLink = screen.getByText('WebApp');
    expect(kindLink.closest('a')).toHaveAttribute('href', '/catalog/web-app');
  });

  it('renders namespace chip for namespaced instances', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('Namespace')).toBeInTheDocument();
    expect(screen.getByText('production')).toBeInTheDocument();
  });

  it('renders Scope chip with Cluster for cluster-scoped instances', () => {
    const clusterInstance = { ...baseInstance, isClusterScoped: true as const, namespace: '' as const };
    render(<InstanceHeaderCard {...defaultProps} instance={clusterInstance} />, { wrapper });

    expect(screen.getByText('Scope')).toBeInTheDocument();
    expect(screen.getByText('Cluster')).toBeInTheDocument();
    expect(screen.queryByText('Namespace')).not.toBeInTheDocument();
  });

  it('renders Source chip with direct deployment label', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('Source')).toBeInTheDocument();
    expect(screen.getByText('Direct deployment')).toBeInTheDocument();
  });

  it('renders the actions slot', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByRole('button', { name: 'Edit Spec' })).toBeInTheDocument();
  });

  it('renders revision button when canReadRGD and revision exists', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('Rev 3')).toBeInTheDocument();
  });

  it('calls onRevisionClick when revision button is clicked', async () => {
    const onRevisionClick = vi.fn();
    render(<InstanceHeaderCard {...defaultProps} onRevisionClick={onRevisionClick} />, { wrapper });

    await userEvent.click(screen.getByText('Rev 3'));
    expect(onRevisionClick).toHaveBeenCalledOnce();
  });

  it('does not render revision button when canReadRGD is false', () => {
    render(<InstanceHeaderCard {...defaultProps} canReadRGD={false} />, { wrapper });

    expect(screen.queryByText('Rev 3')).not.toBeInTheDocument();
  });

  it('shows kroState when not ACTIVE', () => {
    render(<InstanceHeaderCard {...defaultProps} kroState="DELETING" />, { wrapper });

    expect(screen.getByText('DELETING')).toBeInTheDocument();
  });

  it('hides kroState when ACTIVE', () => {
    render(<InstanceHeaderCard {...defaultProps} kroState="ACTIVE" />, { wrapper });

    expect(screen.queryByText('ACTIVE')).not.toBeInTheDocument();
  });

  describe('health rollup strip', () => {
    it('renders conditions, resources, events, and last reconciled cells', () => {
      render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

      const rollup = screen.getByTestId('health-rollup');
      expect(within(rollup).getByText('Conditions')).toBeInTheDocument();
      expect(within(rollup).getByText('4/4')).toBeInTheDocument();
      expect(within(rollup).getByText('passing')).toBeInTheDocument();

      expect(within(rollup).getByText('Resources')).toBeInTheDocument();
      expect(within(rollup).getByText('6/7')).toBeInTheDocument();
      expect(within(rollup).getByText('ready · 1 failing')).toBeInTheDocument();

      expect(within(rollup).getByText('Events')).toBeInTheDocument();
      expect(within(rollup).getByText('16')).toBeInTheDocument();
      expect(within(rollup).getByText('0 warnings')).toBeInTheDocument();

      expect(within(rollup).getByText('Last reconciled')).toBeInTheDocument();
    });

    it('shows failing label when not all conditions pass', () => {
      render(
        <InstanceHeaderCard
          {...defaultProps}
          rollup={{ ...baseRollup, conditionsPassing: 3, conditionsTotal: 4 }}
        />,
        { wrapper }
      );

      const rollup = screen.getByTestId('health-rollup');
      expect(within(rollup).getByText('3/4')).toBeInTheDocument();
      expect(within(rollup).getByText('failing')).toBeInTheDocument();
    });

    it('shows dashes when counts are unknown', () => {
      render(
        <InstanceHeaderCard
          {...defaultProps}
          rollup={{
            ...baseRollup,
            conditionsPassing: 0,
            conditionsTotal: 0,
            resourcesReady: 0,
            resourcesTotal: 0,
            resourcesFailing: 0,
          }}
        />,
        { wrapper }
      );

      const rollup = screen.getByTestId('health-rollup');
      expect(within(rollup).getAllByText('—').length).toBeGreaterThanOrEqual(2);
    });

    it('navigates to the matching tab when a cell is clicked', async () => {
      const onSelectTab = vi.fn();
      render(<InstanceHeaderCard {...defaultProps} onSelectTab={onSelectTab} />, { wrapper });

      const rollup = screen.getByTestId('health-rollup');
      await userEvent.click(within(rollup).getByText('Resources'));
      expect(onSelectTab).toHaveBeenCalledWith('children');

      await userEvent.click(within(rollup).getByText('Events'));
      expect(onSelectTab).toHaveBeenCalledWith('events');

      await userEvent.click(within(rollup).getByText('Last reconciled'));
      expect(onSelectTab).toHaveBeenCalledWith('deployment-history');
    });
  });
});
