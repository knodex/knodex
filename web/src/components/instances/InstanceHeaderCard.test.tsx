// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { InstanceHeaderCard } from './InstanceHeaderCard';
import type { Instance } from '@/types/rgd';

vi.mock('@/components/ui/rgd-icon', () => ({
  RGDIcon: () => <span data-testid="rgd-icon" />,
}));

vi.mock('@/components/shared/ScopeIndicator', () => ({
  ScopeIndicator: () => <span data-testid="scope-indicator" />,
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
  deploymentMode: 'direct',
};

const defaultProps = {
  instance: baseInstance,
  parentRGD: { description: 'A web application', lastIssuedRevision: 3, labels: {} },
  canReadRGD: true,
  kroState: 'ACTIVE',
  onRevisionClick: vi.fn(),
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

  it('renders kind as a link to catalog', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    const kindLink = screen.getByText('WebApp');
    expect(kindLink.closest('a')).toHaveAttribute('href', '/catalog/web-app');
  });

  it('renders namespace for namespaced instances', () => {
    render(<InstanceHeaderCard {...defaultProps} />, { wrapper });

    expect(screen.getByText('production')).toBeInTheDocument();
  });

  it('renders scope indicator for cluster-scoped instances', () => {
    const clusterInstance = { ...baseInstance, isClusterScoped: true, namespace: '' };
    render(<InstanceHeaderCard {...defaultProps} instance={clusterInstance} />, { wrapper });

    expect(screen.getByTestId('scope-indicator')).toBeInTheDocument();
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
});
