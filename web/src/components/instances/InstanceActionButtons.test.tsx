// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InstanceActionButtons } from './InstanceActionButtons';

// Wrap with TooltipProvider since component uses Tooltip
vi.mock('@/components/ui/tooltip', async () => {
  const actual = await vi.importActual<typeof import('@/components/ui/tooltip')>('@/components/ui/tooltip');
  return {
    ...actual,
    Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    TooltipTrigger: ({ children, asChild: _asChild }: { children: React.ReactNode; asChild?: boolean }) => <>{children}</>,
    TooltipContent: ({ children }: { children: React.ReactNode }) => <span data-testid="tooltip">{children}</span>,
  };
});

const defaultProps = {
  instanceUrl: null,
  canUpdate: true,
  isLoadingCanUpdate: false,
  isErrorCanUpdate: false,
  canDelete: true,
  isLoadingCanDelete: false,
  isErrorCanDelete: false,
  isTerminal: false,
  isDeleting: false,
  kroState: 'ACTIVE',
  onEdit: vi.fn(),
  onDelete: vi.fn(),
};

describe('InstanceActionButtons', () => {
  it('renders Edit and Delete buttons when permitted', () => {
    render(<InstanceActionButtons {...defaultProps} />);

    expect(screen.getByText('Edit Spec')).toBeInTheDocument();
    expect(screen.getByText('Delete')).toBeInTheDocument();
  });

  it('renders Visit button when instanceUrl is provided', () => {
    render(<InstanceActionButtons {...defaultProps} instanceUrl="https://example.com" />);

    const link = screen.getByText('Visit').closest('a');
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('does not render Visit button when instanceUrl is null', () => {
    render(<InstanceActionButtons {...defaultProps} instanceUrl={null} />);

    expect(screen.queryByText('Visit')).not.toBeInTheDocument();
  });

  it('hides Edit button when canUpdate is false and not loading/error', () => {
    render(<InstanceActionButtons {...defaultProps} canUpdate={false} />);

    expect(screen.queryByText('Edit Spec')).not.toBeInTheDocument();
  });

  it('disables Edit button when isTerminal is true', () => {
    render(<InstanceActionButtons {...defaultProps} isTerminal={true} kroState="DELETING" />);

    expect(screen.getByText('Edit Spec').closest('button')).toBeDisabled();
  });

  it('hides Delete button when isDeleting is true', () => {
    render(<InstanceActionButtons {...defaultProps} isDeleting={true} />);

    expect(screen.queryByText('Delete')).not.toBeInTheDocument();
  });

  it('calls onEdit when Edit button is clicked', async () => {
    const onEdit = vi.fn();
    render(<InstanceActionButtons {...defaultProps} onEdit={onEdit} />);

    await userEvent.click(screen.getByText('Edit Spec'));
    expect(onEdit).toHaveBeenCalledOnce();
  });

  it('calls onDelete when Delete button is clicked', async () => {
    const onDelete = vi.fn();
    render(<InstanceActionButtons {...defaultProps} onDelete={onDelete} />);

    await userEvent.click(screen.getByText('Delete'));
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it('shows Edit button while loading permissions', () => {
    render(<InstanceActionButtons {...defaultProps} canUpdate={false} isLoadingCanUpdate={true} />);

    expect(screen.getByText('Edit Spec')).toBeInTheDocument();
  });
});
