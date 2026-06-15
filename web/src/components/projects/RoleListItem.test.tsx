// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RoleListItem } from './RoleListItem';

vi.mock('./PolicyRulesTable', () => ({
  PolicyRulesTable: () => <div data-testid="policy-rules-table" />,
}));

vi.mock('@/components/teams/TeamPicker', () => ({
  TeamPicker: () => <div data-testid="team-picker" />,
}));

vi.mock('./DestinationScopeSelector', () => ({
  DestinationScopeSelector: () => <div data-testid="destination-scope-selector" />,
  DestinationScopeBadge: () => <span data-testid="destination-scope-badge">All destinations</span>,
}));

const baseRole = {
  name: 'deployer',
  description: 'Can deploy and manage instances',
  policies: ['p1', 'p2'],
  teams: ['team-alpha'],
};

const defaultProps = {
  originalRole: baseRole,
  role: baseRole,
  projectName: 'alpha',
  projectDestinations: [{ namespace: 'ns-app' }, { namespace: 'ns-staging' }],
  isExpanded: false,
  isEditing: false,
  hasChanges: false,
  canManage: true,
  isUpdating: false,
  onToggleExpand: vi.fn(),
  onEdit: vi.fn(),
  onSave: vi.fn(),
  onCancelEdit: vi.fn(),
  onDelete: vi.fn(),
  onPoliciesChange: vi.fn(),
  onTeamsChange: vi.fn(),
  onDestinationsChange: vi.fn(),
};

describe('RoleListItem', () => {
  it('renders role name and policy count', () => {
    render(<RoleListItem {...defaultProps} />);

    expect(screen.getByText('deployer')).toBeInTheDocument();
    expect(screen.getByText('2 policies')).toBeInTheDocument();
  });

  it('renders teams badge, not groups badge', () => {
    render(<RoleListItem {...defaultProps} />);

    expect(screen.getByText('1 teams')).toBeInTheDocument();
    expect(screen.queryByText(/groups/)).not.toBeInTheDocument();
  });

  it('renders description', () => {
    render(<RoleListItem {...defaultProps} />);

    expect(screen.getByText('Can deploy and manage instances')).toBeInTheDocument();
  });

  it('shows Built-in badge for built-in roles', () => {
    const adminRole = { ...baseRole, name: 'admin' };
    render(<RoleListItem {...defaultProps} originalRole={adminRole} role={adminRole} />);

    expect(screen.getByText('Built-in')).toBeInTheDocument();
  });

  it('does not show Built-in badge for custom roles', () => {
    render(<RoleListItem {...defaultProps} />);

    expect(screen.queryByText('Built-in')).not.toBeInTheDocument();
  });

  it('shows Unsaved badge when hasChanges is true', () => {
    render(<RoleListItem {...defaultProps} hasChanges={true} />);

    expect(screen.getByText('Unsaved')).toBeInTheDocument();
  });

  it('calls onToggleExpand when header is clicked', async () => {
    const onToggleExpand = vi.fn();
    render(<RoleListItem {...defaultProps} onToggleExpand={onToggleExpand} />);

    await userEvent.click(screen.getByText('deployer'));
    expect(onToggleExpand).toHaveBeenCalledWith('deployer');
  });

  it('renders PolicyRulesTable and TeamPicker when expanded, no OIDCGroupsManager', () => {
    render(<RoleListItem {...defaultProps} isExpanded={true} />);

    expect(screen.getByTestId('policy-rules-table')).toBeInTheDocument();
    expect(screen.getByTestId('team-picker')).toBeInTheDocument();
    expect(screen.queryByTestId('oidc-groups-manager')).not.toBeInTheDocument();
  });

  it('does not render expanded content when collapsed', () => {
    render(<RoleListItem {...defaultProps} isExpanded={false} />);

    expect(screen.queryByTestId('policy-rules-table')).not.toBeInTheDocument();
  });

  it('hides delete button for built-in roles', () => {
    const adminRole = { ...baseRole, name: 'admin' };
    render(<RoleListItem {...defaultProps} originalRole={adminRole} role={adminRole} />);

    // No trash button rendered for built-in roles
    const buttons = screen.getAllByRole('button');
    const trashButton = buttons.find(b => b.classList.contains('text-destructive'));
    expect(trashButton).toBeUndefined();
  });

  it('shows Save/Cancel buttons when hasChanges', () => {
    render(<RoleListItem {...defaultProps} hasChanges={true} />);

    expect(screen.getByText('Save')).toBeInTheDocument();
  });

  it('calls onSave with role name when Save is clicked', async () => {
    const onSave = vi.fn();
    render(<RoleListItem {...defaultProps} hasChanges={true} onSave={onSave} />);

    await userEvent.click(screen.getByText('Save'));
    expect(onSave).toHaveBeenCalledWith('deployer');
  });
});
