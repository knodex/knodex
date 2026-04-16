// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AddRoleForm } from './AddRoleForm';

vi.mock('./PolicyRulesTable', () => ({
  PolicyRulesTable: () => <div data-testid="policy-rules-table" />,
}));

vi.mock('./OIDCGroupsManager', () => ({
  OIDCGroupsManager: () => <div data-testid="oidc-groups-manager" />,
}));

vi.mock('./DestinationScopeSelector', () => ({
  DestinationScopeSelector: () => <div data-testid="destination-scope-selector" />,
}));

vi.mock('@/lib/role-presets', () => ({
  ROLE_PRESETS: [
    { name: 'admin', label: 'Admin', description: 'Full access', policies: ['p, proj:{project}:admin, *, *, {project}/*, allow'] },
    { name: 'viewer', label: 'Viewer', description: 'Read-only', policies: ['p, proj:{project}:viewer, *, get, {project}/*, allow'] },
  ],
  resolvePresetPolicies: vi.fn().mockReturnValue(['resolved-policy']),
}));

vi.mock('@/components/ui/tooltip', async () => {
  const actual = await vi.importActual<typeof import('@/components/ui/tooltip')>('@/components/ui/tooltip');
  return {
    ...actual,
    Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    TooltipTrigger: ({ children }: { children: React.ReactNode; asChild?: boolean }) => <>{children}</>,
    TooltipContent: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  };
});

const defaultAddition = {
  showAddForm: true,
  setShowAddForm: vi.fn(),
  newRoleName: '',
  setNewRoleName: vi.fn(),
  newRoleDescription: '',
  setNewRoleDescription: vi.fn(),
  newRolePolicies: [] as string[],
  setNewRolePolicies: vi.fn(),
  newRoleGroups: [] as string[],
  setNewRoleGroups: vi.fn(),
  newRoleDestinations: [] as string[],
  setNewRoleDestinations: vi.fn(),
  isAdding: false,
  addRoleError: null as string | null,
  setAddRoleError: vi.fn(),
  resetForm: vi.fn(),
  handleAddRole: vi.fn(),
};

const defaultProps = {
  projectName: 'alpha',
  projectDestinations: [{ namespace: 'ns-app' }, { namespace: 'ns-staging' }],
  roles: [] as any[],
  addition: defaultAddition,
  onCancel: vi.fn(),
};

describe('AddRoleForm', () => {
  it('renders role name and description inputs', () => {
    render(<AddRoleForm {...defaultProps} />);

    expect(screen.getByLabelText('Role Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Description')).toBeInTheDocument();
  });

  it('renders preset buttons', () => {
    render(<AddRoleForm {...defaultProps} />);

    expect(screen.getByText('Admin')).toBeInTheDocument();
    expect(screen.getByText('Viewer')).toBeInTheDocument();
    expect(screen.getByText('Custom Role')).toBeInTheDocument();
  });

  it('disables preset button when role already exists', () => {
    const props = {
      ...defaultProps,
      roles: [{ name: 'admin', policies: [], groups: [] }],
    };
    render(<AddRoleForm {...props} />);

    expect(screen.getByText('Admin').closest('button')).toBeDisabled();
  });

  it('disables Add Role button when name is empty', () => {
    render(<AddRoleForm {...defaultProps} />);

    expect(screen.getByText('Add Role').closest('button')).toBeDisabled();
  });

  it('enables Add Role button when name is provided', () => {
    const props = {
      ...defaultProps,
      addition: { ...defaultAddition, newRoleName: 'deployer' },
    };
    render(<AddRoleForm {...props} />);

    expect(screen.getByText('Add Role').closest('button')).not.toBeDisabled();
  });

  it('displays error message', () => {
    const props = {
      ...defaultProps,
      addition: { ...defaultAddition, addRoleError: 'Role already exists' },
    };
    render(<AddRoleForm {...props} />);

    expect(screen.getByText('Role already exists')).toBeInTheDocument();
  });

  it('shows "Adding..." text while isAdding', () => {
    const props = {
      ...defaultProps,
      addition: { ...defaultAddition, isAdding: true, newRoleName: 'test' },
    };
    render(<AddRoleForm {...props} />);

    expect(screen.getByText('Adding...')).toBeInTheDocument();
  });

  it('calls onCancel when Cancel button is clicked', async () => {
    const onCancel = vi.fn();
    render(<AddRoleForm {...defaultProps} onCancel={onCancel} />);

    await userEvent.click(screen.getByText('Cancel'));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it('calls handleAddRole when Add Role button is clicked', async () => {
    const handleAddRole = vi.fn();
    const props = {
      ...defaultProps,
      addition: { ...defaultAddition, newRoleName: 'deployer', handleAddRole },
    };
    render(<AddRoleForm {...props} />);

    await userEvent.click(screen.getByText('Add Role'));
    expect(handleAddRole).toHaveBeenCalledOnce();
  });
});
