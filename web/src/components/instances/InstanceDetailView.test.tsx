// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstanceDetailView } from './InstanceDetailView';
import type { Instance } from '@/types/rgd';

// Mock hooks
vi.mock('@/hooks/useInstances', () => ({
  useDeleteInstance: vi.fn(),
}));

vi.mock('@/hooks/useCanI', () => ({
  useCanI: vi.fn(),
}));

vi.mock('@/hooks/useRGDs', () => ({
  useRGDList: vi.fn(),
}));

// Mock child components
vi.mock('./HealthBadge', () => ({
  HealthBadge: ({ health }: { health: string }) => <div data-testid="health-badge">{health}</div>,
}));

vi.mock('./GitStatusDisplay', () => ({
  GitStatusDisplay: () => <div data-testid="git-status-display">Git Status Display</div>,
}));

vi.mock('./StatusTimeline', () => ({
  StatusTimeline: () => <div data-testid="status-timeline">Status Timeline</div>,
}));

vi.mock('./DeploymentTimeline', () => ({
  DeploymentTimeline: () => <div data-testid="deployment-timeline">Deployment Timeline</div>,
}));

vi.mock('./EditInstanceSpecDialog', () => ({
  EditInstanceSpecDialog: () => <div data-testid="edit-instance-spec-dialog" />,
}));

vi.mock('./GitOpsDriftBanner', () => ({
  GitOpsDriftBanner: () => null,
}));

vi.mock('./InstanceAddOns', () => ({
  InstanceAddOns: () => <div data-testid="instance-add-ons">Instance Add-Ons</div>,
}));

vi.mock('./InstanceDependsOn', () => ({
  InstanceDependsOn: () => <div data-testid="instance-depends-on">Instance Depends On</div>,
}));

vi.mock('./InstanceStatusCard', () => ({
  InstanceStatusCard: ({ status, conditions }: { status?: Record<string, unknown>; conditions?: unknown[] }) => (
    <div data-testid="instance-status-card">
      {status && Object.entries(status)
        .filter(([k]) => k !== 'state' && k !== 'conditions')
        .map(([k, v]) => (
          <span key={k}>{String(v)}</span>
        ))}
      {conditions && conditions.length > 0 && <span>Conditions: {conditions.length}</span>}
    </div>
  ),
}));

const mockInstance: Instance = {
  name: 'test-instance',
  namespace: 'test-namespace',
  rgdName: 'test-rgd',
  rgdNamespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'TestResource',
  health: 'Healthy',
  conditions: [
    {
      type: 'Ready',
      status: 'True',
      reason: 'AllResourcesReady',
      message: 'All resources are ready',
    },
  ],
  spec: {
    replicas: 3,
    image: 'nginx:latest',
  },
  status: {
    phase: 'Running',
  },
  labels: {
    app: 'test',
  },
  annotations: {
    'knodex.io/instance-id': 'test-id-123',
  },
  createdAt: '2024-01-15T10:30:00Z',
  updatedAt: '2024-01-15T11:00:00Z',
  deploymentMode: 'direct',
};

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
      <MemoryRouter>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('InstanceDetailView', () => {
  const mockOnBack = vi.fn();
  const mockOnDeleted = vi.fn();
  const mockMutateAsync = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();

    const { useDeleteInstance } = await import('@/hooks/useInstances');
    const { useCanI } = await import('@/hooks/useCanI');
    const { useRGDList } = await import('@/hooks/useRGDs');

    vi.mocked(useDeleteInstance).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      error: null,
    } as any);

    vi.mocked(useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
    } as any);

    vi.mocked(useRGDList).mockReturnValue({
      data: { items: [], totalCount: 0 },
      isLoading: false,
      error: null,
    } as any);
  });

  describe('Basic Rendering', () => {
    it('renders instance name and namespace', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByText('test-instance')).toBeInTheDocument();
      expect(screen.getByText('test-namespace')).toBeInTheDocument();
    });

    it('renders instance metadata', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByText('test-rgd')).toBeInTheDocument();
      expect(screen.getByText('TestResource')).toBeInTheDocument();
      expect(screen.getByText('kro.run/v1alpha1')).toBeInTheDocument();
    });

    it('renders health badge', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('health-badge')).toBeInTheDocument();
      expect(screen.getByText('Healthy')).toBeInTheDocument();
    });

    it('renders created date', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const expectedDate = new Date('2024-01-15T10:30:00Z').toLocaleString();
      expect(screen.getByText(expectedDate)).toBeInTheDocument();
    });

    it('renders git status display', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('git-status-display')).toBeInTheDocument();
    });

    it('renders deployment timeline in Deployment History tab', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Deployment timeline is behind the Deployment History tab
      await user.click(screen.getByRole('tab', { name: /deployment history/i }));
      expect(screen.getByTestId('deployment-timeline')).toBeInTheDocument();
    });
  });

  describe('Back Button', () => {
    it('renders back button', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByRole('button', { name: /back/i })).toBeInTheDocument();
    });

    it('calls onBack when back button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const backButton = screen.getByRole('button', { name: /back/i });
      await user.click(backButton);

      expect(mockOnBack).toHaveBeenCalledTimes(1);
    });
  });

  describe('Delete Button Permissions', () => {
    it('shows delete button when user has permission', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
    });

    it('hides delete button when user lacks permission', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('checks correct permission for delete button', async () => {
      const { useCanI } = await import('@/hooks/useCanI');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(useCanI).toHaveBeenCalledWith('instances', 'delete', 'test-namespace');
    });

    it('checks correct permission for edit spec button', async () => {
      const { useCanI } = await import('@/hooks/useCanI');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(useCanI).toHaveBeenCalledWith('instances', 'update', 'test-namespace');
    });

    it('shows action buttons when useCanI returns an error (fail-open)', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
        isError: true,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /edit spec/i })).toBeInTheDocument();
    });
  });

  describe('Delete Confirmation Flow', () => {
    it('shows confirmation dialog when delete is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByText(/Delete "test-instance"\?/i)).toBeInTheDocument();
      expect(screen.getByText(/This action cannot be undone/i)).toBeInTheDocument();
    });

    it('shows confirm and cancel buttons in confirmation dialog', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByRole('button', { name: /yes, delete/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('cancels deletion when cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(screen.queryByText(/Delete "test-instance"\?/i)).not.toBeInTheDocument();
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('calls delete mutation when confirmed', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockResolvedValue({});

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} onDeleted={mockOnDeleted} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const confirmButton = screen.getByRole('button', { name: /yes, delete/i });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          namespace: 'test-namespace',
          kind: 'TestResource',
          name: 'test-instance',
        });
      });
    });

    it('calls onDeleted callback after successful deletion', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockResolvedValue({});

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} onDeleted={mockOnDeleted} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const confirmButton = screen.getByRole('button', { name: /yes, delete/i });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockOnDeleted).toHaveBeenCalled();
      });
    });

    it('shows loading state during deletion', async () => {
      const user = userEvent.setup();
      const { useDeleteInstance } = await import('@/hooks/useInstances');

      vi.mocked(useDeleteInstance).mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: true,
        isError: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByRole('button', { name: /deleting\.\.\./i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /deleting\.\.\./i })).toBeDisabled();
    });
  });

  describe('Delete Error Handling', () => {
    it('displays error message when deletion fails', async () => {
      const { useDeleteInstance } = await import('@/hooks/useInstances');
      const mockError = new Error('Network connection failed');

      vi.mocked(useDeleteInstance).mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: true,
        error: mockError,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByText('Failed to delete instance')).toBeInTheDocument();
      expect(screen.getByText('Network connection failed')).toBeInTheDocument();
    });

    it('displays generic error message for unknown errors', async () => {
      const { useDeleteInstance } = await import('@/hooks/useInstances');

      vi.mocked(useDeleteInstance).mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: true,
        error: 'Some string error',
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByText('Failed to delete instance')).toBeInTheDocument();
      expect(screen.getByText('An unexpected error occurred')).toBeInTheDocument();
    });
  });

  describe('Status Card Rendering', () => {
    it('renders unified status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
    });

    it('passes conditions to status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Mock renders condition count
      expect(screen.getByText('Conditions: 1')).toBeInTheDocument();
    });

    it('passes status to status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Mock renders status values (excluding state/conditions)
      expect(screen.getByText('Running')).toBeInTheDocument();
    });

    it('renders status card even with empty conditions', () => {
      const instanceWithoutConditions: Instance = {
        ...mockInstance,
        conditions: [],
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutConditions} onBack={mockOnBack} />
      );

      // Status card should still render (it has status data)
      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
    });
  });

  describe('Tab Navigation', () => {
    it('shows Status tab as active by default with status content', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Status tab exists and is active (has border-primary class)
      const statusTab = screen.getByRole('tab', { name: /^Status$/i });
      expect(statusTab).toBeInTheDocument();
      expect(statusTab.className).toContain('border-primary');

      // Status content is visible
      expect(screen.getByTestId('git-status-display')).toBeInTheDocument();
    });

    it('switches to Deployment History tab and shows timeline', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      await user.click(screen.getByRole('tab', { name: /deployment history/i }));

      expect(screen.getByTestId('deployment-timeline')).toBeInTheDocument();
      // Status content should be hidden
      expect(screen.queryByTestId('git-status-display')).not.toBeInTheDocument();
    });

    it('shows Add-ons tab when add-ons exist', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: [{ name: 'addon-1' }], totalCount: 2 },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByRole('tab', { name: /add-ons \(2\)/i })).toBeInTheDocument();
    });

    it('hides Add-ons tab when add-ons count is 0', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.queryByRole('tab', { name: /add-ons/i })).not.toBeInTheDocument();
    });

    it('shows InstanceAddOns content when Add-ons tab is clicked', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: [], totalCount: 2 },
        isLoading: false,
        error: null,
      } as any);

      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      await user.click(screen.getByRole('tab', { name: /add-ons \(2\)/i }));

      expect(screen.getByTestId('instance-add-ons')).toBeInTheDocument();
      expect(screen.queryByTestId('git-status-display')).not.toBeInTheDocument();
    });

    it('shows Spec tab when instance.spec is non-empty', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(screen.getByRole('tab', { name: /^Spec$/i })).toBeInTheDocument();
    });

    it('hides Spec tab when spec is empty', () => {
      const instanceWithoutSpec: Instance = {
        ...mockInstance,
        spec: {},
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutSpec} onBack={mockOnBack} />
      );

      expect(screen.queryByRole('tab', { name: /^Spec$/i })).not.toBeInTheDocument();
    });

    it('hides Spec tab when spec is undefined', () => {
      const instanceWithUndefinedSpec: Instance = {
        ...mockInstance,
        spec: undefined,
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithUndefinedSpec} onBack={mockOnBack} />
      );

      expect(screen.queryByRole('tab', { name: /^Spec$/i })).not.toBeInTheDocument();
    });

    it('shows spec content when Spec tab is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      await user.click(screen.getByRole('tab', { name: /^Spec$/i }));

      const specContent = screen.getByTestId('spec-content');
      expect(within(specContent).getByText(/"replicas": 3/)).toBeInTheDocument();
      expect(within(specContent).getByText(/"image": "nginx:latest"/)).toBeInTheDocument();
    });

    it('renders tabs in correct order: Status, Add-ons, Deployment History, Spec', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: [{ name: 'addon-1' }], totalCount: 1 },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const tabButtons = screen.getAllByRole('tab');
      expect(tabButtons).toHaveLength(4);
      expect(tabButtons[0]).toHaveTextContent(/^Status$/);
      expect(tabButtons[1]).toHaveTextContent(/^Add-ons/);
      expect(tabButtons[2]).toHaveTextContent(/^Deployment History$/);
      expect(tabButtons[3]).toHaveTextContent(/^Spec$/);
    });

    it('calls useRGDList with extendsKind and pageSize 100 for add-ons count', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      expect(useRGDList).toHaveBeenCalledWith({ extendsKind: 'TestResource', pageSize: 100 });
    });

    it('calls useRGDList with undefined when instance.kind is falsy', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      const instanceWithoutKind: Instance = { ...mockInstance, kind: '' };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutKind} onBack={mockOnBack} />
      );

      expect(useRGDList).toHaveBeenCalledWith(undefined);
    });

    it('resets to Status tab when the active tab is removed from the tab list', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      let currentMockReturn: ReturnType<typeof vi.fn> = {
        data: { items: [], totalCount: 2 },
        isLoading: false,
        error: null,
      };
      vi.mocked(useRGDList).mockImplementation(() => currentMockReturn as any);

      const user = userEvent.setup();
      const queryClient = createQueryClient();
      const { rerender } = render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
          </MemoryRouter>
        </QueryClientProvider>
      );

      // Navigate to Add-ons tab
      await user.click(screen.getByRole('tab', { name: /add-ons \(2\)/i }));
      expect(screen.queryByTestId('git-status-display')).not.toBeInTheDocument();

      // Simulate add-ons count dropping to 0 (background re-fetch returns empty)
      currentMockReturn = {
        data: { items: [], totalCount: 0 },
        isLoading: false,
        error: null,
      };
      rerender(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
          </MemoryRouter>
        </QueryClientProvider>
      );

      // useEffect detects "addons" is no longer in tabs and resets to Status
      await waitFor(() => {
        expect(screen.getByTestId('git-status-display')).toBeInTheDocument();
      });
    });
  });

  describe('GitOps Deployment Mode', () => {
    it('renders status timeline for gitops deployment', () => {
      const gitopsInstance: Instance = {
        ...mockInstance,
        deploymentMode: 'gitops',
      };

      renderWithProviders(
        <InstanceDetailView instance={gitopsInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('status-timeline')).toBeInTheDocument();
    });

    it('renders status timeline for hybrid deployment', () => {
      const hybridInstance: Instance = {
        ...mockInstance,
        deploymentMode: 'hybrid',
      };

      renderWithProviders(
        <InstanceDetailView instance={hybridInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('status-timeline')).toBeInTheDocument();
    });

    it('does not render status timeline for direct deployment', () => {
      const directInstance: Instance = {
        ...mockInstance,
        deploymentMode: 'direct',
      };

      renderWithProviders(
        <InstanceDetailView instance={directInstance} onBack={mockOnBack} />
      );

      expect(screen.queryByTestId('status-timeline')).not.toBeInTheDocument();
    });

    it('does not render status timeline when deployment mode is undefined', () => {
      const instanceWithoutMode: Instance = {
        ...mockInstance,
        deploymentMode: undefined,
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutMode} onBack={mockOnBack} />
      );

      expect(screen.queryByTestId('status-timeline')).not.toBeInTheDocument();
    });
  });

  describe('Different Health States', () => {
    it('renders Degraded health status', () => {
      const degradedInstance: Instance = {
        ...mockInstance,
        health: 'Degraded',
      };

      renderWithProviders(
        <InstanceDetailView instance={degradedInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('health-badge')).toHaveTextContent('Degraded');
    });

    it('renders Unhealthy health status', () => {
      const unhealthyInstance: Instance = {
        ...mockInstance,
        health: 'Unhealthy',
      };

      renderWithProviders(
        <InstanceDetailView instance={unhealthyInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('health-badge')).toHaveTextContent('Unhealthy');
    });

    it('renders Progressing health status', () => {
      const progressingInstance: Instance = {
        ...mockInstance,
        health: 'Progressing',
      };

      renderWithProviders(
        <InstanceDetailView instance={progressingInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('health-badge')).toHaveTextContent('Progressing');
    });

    it('renders Unknown health status', () => {
      const unknownInstance: Instance = {
        ...mockInstance,
        health: 'Unknown',
      };

      renderWithProviders(
        <InstanceDetailView instance={unknownInstance} onBack={mockOnBack} />
      );

      expect(screen.getByTestId('health-badge')).toHaveTextContent('Unknown');
    });
  });
});
