// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
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
  useRGD: vi.fn(),
}));

vi.mock('@/hooks/useHistory', () => ({
  useInstanceEvents: vi.fn().mockReturnValue({
    data: undefined,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

// Mock child components
vi.mock('./HealthBadge', () => ({
  HealthBadge: ({ health }: { health: string }) => <div data-testid="health-badge">{health}</div>,
}));

vi.mock('./InstanceStatusBanner', () => ({
  InstanceStatusBanner: ({ health, state }: { health: string; state?: string }) => (
    <div data-testid="instance-status-banner">{state || health}</div>
  ),
}));

vi.mock('./GitStatusDisplay', () => ({
  GitStatusDisplay: () => <div data-testid="git-status-display">Git Status Display</div>,
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

vi.mock('./InstanceExternalRefs', () => ({
  InstanceExternalRefs: () => <div data-testid="instance-external-refs">Instance External Refs</div>,
}));

vi.mock('./RevisionDiffDrawer', () => ({
  RevisionDiffDrawer: ({ open, rgdName, currentRevision }: { open: boolean; rgdName: string; currentRevision: number }) => (
    open ? <div data-testid="revision-diff-drawer">Revision Changes for {rgdName} rev {currentRevision}</div> : null
  ),
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
        <TooltipProvider>
          {ui}
        </TooltipProvider>
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('InstanceDetailView', () => {

  const mockOnDeleted = vi.fn();
  const mockMutateAsync = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();

    const { useDeleteInstance } = await import('@/hooks/useInstances');
    const { useCanI } = await import('@/hooks/useCanI');
    const { useRGDList, useRGD } = await import('@/hooks/useRGDs');

    vi.mocked(useDeleteInstance).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      error: null,
      reset: vi.fn(),
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

    vi.mocked(useRGD).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);
  });

  describe('Basic Rendering', () => {
    it('renders instance name and namespace', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByText('test-instance')).toBeInTheDocument();
      expect(screen.getByText('test-namespace')).toBeInTheDocument();
    });

    it('renders instance metadata', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Kind links to the RGD catalog page
      const kindLink = screen.getByRole('link', { name: 'TestResource' });
      expect(kindLink).toHaveAttribute('href', '/catalog/test-rgd');
    });

    it('renders health status in header without health-badge component', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Health is displayed inline in InstanceHeaderCard's Status row
      expect(screen.getAllByText('Healthy').length).toBeGreaterThanOrEqual(1);
      // HealthBadge component is not used — health is rendered inline
      expect(screen.queryByTestId('health-badge')).not.toBeInTheDocument();
    });

    it('renders git status display for gitops instances', () => {
      const gitopsInstance = { ...mockInstance, deploymentMode: 'gitops' as const, gitInfo: { branch: 'main', commitSha: 'abc123', pushStatus: 'completed' as const } };
      renderWithProviders(
        <InstanceDetailView instance={gitopsInstance} />
      );

      expect(screen.getByTestId('git-status-display')).toBeInTheDocument();
    });

    it('renders git status display for hybrid instances', () => {
      const hybridInstance = { ...mockInstance, deploymentMode: 'hybrid' as const, gitInfo: { branch: 'main', commitSha: 'abc123', pushStatus: 'completed' as const } };
      renderWithProviders(
        <InstanceDetailView instance={hybridInstance} />
      );

      expect(screen.getByTestId('git-status-display')).toBeInTheDocument();
    });

    it('does not render git status display for direct instances', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByTestId('git-status-display')).not.toBeInTheDocument();
      expect(screen.getByText('Direct deployment')).toBeInTheDocument();
    });

    it('renders deployment timeline in Deployment History tab', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Deployment timeline is behind the Deployment History tab
      await user.click(screen.getByRole('tab', { name: /deployment history/i }));
      expect(screen.getByTestId('deployment-timeline')).toBeInTheDocument();
    });
  });

  describe('Back Navigation', () => {
    it('does not render a back button (breadcrumbs handle navigation)', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByRole('button', { name: /back/i })).not.toBeInTheDocument();
    });
  });

  describe('Delete Button Permissions', () => {
    it('shows delete button when user has permission', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
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
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('checks correct permission for delete button', async () => {
      const { useCanI } = await import('@/hooks/useCanI');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // No project label on mockInstance → falls back to '-' per useInstancePermissions
      expect(useCanI).toHaveBeenCalledWith('instances', 'delete', '-');
    });

    it('checks correct permission for edit spec button', async () => {
      const { useCanI } = await import('@/hooks/useCanI');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // No project label on mockInstance → falls back to '-' per useInstancePermissions
      expect(useCanI).toHaveBeenCalledWith('instances', 'update', '-');
    });

    it('shows action buttons when useCanI returns an error (fail-open)', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
        isError: true,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /edit spec/i })).toBeInTheDocument();
    });
  });

  describe('Delete Confirmation Flow', () => {
    it('opens type-to-confirm dialog when delete is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByText(`Delete ${mockInstance.name}?`)).toBeInTheDocument();
      expect(screen.getByText(/All resources managed by this instance will be deleted/i)).toBeInTheDocument();
    });

    it('shows type-to-confirm input and disabled Delete Instance button', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByTestId('confirm-name-input')).toBeInTheDocument();
      expect(screen.getByTestId('confirm-delete-button')).toBeDisabled();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('cancels deletion when cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(screen.queryByText(`Delete ${mockInstance.name}?`)).not.toBeInTheDocument();
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('calls delete mutation when name typed and confirmed', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockResolvedValue({});

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onDeleted={mockOnDeleted} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const input = screen.getByTestId('confirm-name-input');
      await user.type(input, mockInstance.name);

      const confirmButton = screen.getByTestId('confirm-delete-button');
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
        <InstanceDetailView instance={mockInstance} onDeleted={mockOnDeleted} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const input = screen.getByTestId('confirm-name-input');
      await user.type(input, mockInstance.name);

      const confirmButton = screen.getByTestId('confirm-delete-button');
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
        reset: vi.fn(),
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      // Input and cancel should be disabled during deletion
      expect(screen.getByTestId('confirm-name-input')).toBeDisabled();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
    });
  });

  describe('Delete Error Handling', () => {
    it('displays error message in dialog when deletion fails', async () => {
      const user = userEvent.setup();
      const { useDeleteInstance } = await import('@/hooks/useInstances');
      const mockError = new Error('Network connection failed');

      vi.mocked(useDeleteInstance).mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: true,
        error: mockError,
        reset: vi.fn(),
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Open the dialog to see the error
      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByText('Network connection failed')).toBeInTheDocument();
    });

    it('displays fallback error message for null error message', async () => {
      const user = userEvent.setup();
      const { useDeleteInstance } = await import('@/hooks/useInstances');

      vi.mocked(useDeleteInstance).mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: true,
        error: new Error(),
        reset: vi.fn(),
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      expect(screen.getByText('Failed to delete instance')).toBeInTheDocument();
    });
  });

  describe('Status Card Rendering', () => {
    it('renders unified status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
    });

    it('passes conditions to status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Mock renders condition count
      expect(screen.getByText('Conditions: 1')).toBeInTheDocument();
    });

    it('passes status to status card', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
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
        <InstanceDetailView instance={instanceWithoutConditions} />
      );

      // Status card should still render (it has status data)
      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
    });
  });

  describe('Tab Navigation', () => {
    it('shows Status tab as active by default with status content', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // Status tab exists and is active (has border-primary class)
      const statusTab = screen.getByRole('tab', { name: /^Status$/i });
      expect(statusTab).toBeInTheDocument();
      expect(statusTab.className).toContain('border-primary');

      // Status content is visible (instance status card for direct mode)
      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
    });

    it('AC1: Status tab does not render status-timeline (StatusTimeline removed)', () => {
      const gitopsInstance: Instance = {
        ...mockInstance,
        deploymentMode: 'gitops',
      };

      renderWithProviders(
        <InstanceDetailView instance={gitopsInstance} />
      );

      // StatusTimeline component was removed — the Status tab must not render it
      expect(screen.queryByTestId('status-timeline')).not.toBeInTheDocument();
    });

    it('switches to Deployment History tab and shows timeline', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
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
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByRole('tab', { name: /add-ons \(2\)/i })).toBeInTheDocument();
    });

    it('hides Add-ons tab when add-ons count is 0', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
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
        <InstanceDetailView instance={mockInstance} />
      );

      await user.click(screen.getByRole('tab', { name: /add-ons \(2\)/i }));

      expect(screen.getByTestId('instance-add-ons')).toBeInTheDocument();
      expect(screen.queryByTestId('git-status-display')).not.toBeInTheDocument();
    });

    it('shows Spec tab when instance.spec is non-empty', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByRole('tab', { name: /^Spec$/i })).toBeInTheDocument();
    });

    it('hides Spec tab when spec is empty', () => {
      const instanceWithoutSpec: Instance = {
        ...mockInstance,
        spec: {},
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutSpec} />
      );

      expect(screen.queryByRole('tab', { name: /^Spec$/i })).not.toBeInTheDocument();
    });

    it('hides Spec tab when spec is undefined', () => {
      const instanceWithUndefinedSpec: Instance = {
        ...mockInstance,
        spec: undefined,
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithUndefinedSpec} />
      );

      expect(screen.queryByRole('tab', { name: /^Spec$/i })).not.toBeInTheDocument();
    });

    it('shows spec content when Spec tab is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      await user.click(screen.getByRole('tab', { name: /^Spec$/i }));

      const specContent = screen.getByTestId('spec-content');
      expect(within(specContent).getByText(/"replicas": 3/)).toBeInTheDocument();
      expect(within(specContent).getByText(/"image": "nginx:latest"/)).toBeInTheDocument();
    });

    it('renders tabs in correct order: Status, Events, Add-ons, Deployment History, Resources, Spec', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDList).mockReturnValue({
        data: { items: [{ name: 'addon-1' }], totalCount: 1 },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const tabButtons = screen.getAllByRole('tab');
      expect(tabButtons).toHaveLength(6);
      expect(tabButtons[0]).toHaveTextContent(/^Status$/);
      expect(tabButtons[1]).toHaveTextContent(/^Events/);
      expect(tabButtons[2]).toHaveTextContent(/^Add-ons/);
      expect(tabButtons[3]).toHaveTextContent(/^Deployment History$/);
      expect(tabButtons[4]).toHaveTextContent(/^Resources$/);
      expect(tabButtons[5]).toHaveTextContent(/^Spec$/);
    });

    it('calls useRGDList with extendsKind and pageSize 100 for add-ons count', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(useRGDList).toHaveBeenCalledWith({ extendsKind: 'TestResource', pageSize: 100 });
    });

    it('calls useRGDList with undefined when instance.kind is falsy', async () => {
      const { useRGDList } = await import('@/hooks/useRGDs');
      const instanceWithoutKind: Instance = { ...mockInstance, kind: '' };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutKind} />
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
            <TooltipProvider>
              <InstanceDetailView instance={mockInstance} />
            </TooltipProvider>
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
            <TooltipProvider>
              <InstanceDetailView instance={mockInstance} />
            </TooltipProvider>
          </MemoryRouter>
        </QueryClientProvider>
      );

      // useEffect detects "addons" is no longer in tabs and resets to Status
      await waitFor(() => {
        expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
      });
    });
  });

  describe('Cluster-Scoped Instances', () => {
    it('shows Cluster-Scoped instead of namespace for cluster-scoped instances', () => {
      const clusterScopedInstance: Instance = {
        ...mockInstance,
        isClusterScoped: true,
        namespace: '',
      };

      renderWithProviders(
        <InstanceDetailView instance={clusterScopedInstance} />
      );

      expect(screen.getByText('Cluster-Scoped')).toBeInTheDocument();
      expect(screen.queryByText('test-namespace')).not.toBeInTheDocument();
    });

    it('shows namespace for namespace-scoped instances', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByText('test-namespace')).toBeInTheDocument();
      expect(screen.queryByText('Cluster-Scoped')).not.toBeInTheDocument();
    });

    it('calls delete mutation with empty namespace for cluster-scoped instances', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockResolvedValue({});

      const clusterScopedInstance: Instance = {
        ...mockInstance,
        isClusterScoped: true,
        namespace: '',
      };

      renderWithProviders(
        <InstanceDetailView instance={clusterScopedInstance} onDeleted={mockOnDeleted} />
      );

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      const input = screen.getByTestId('confirm-name-input');
      await user.type(input, clusterScopedInstance.name);

      const confirmButton = screen.getByTestId('confirm-delete-button');
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          namespace: '',
          kind: 'TestResource',
          name: 'test-instance',
        });
      });
    });
  });

  describe('Different Health States', () => {
    it('renders Degraded health status', () => {
      const degradedInstance: Instance = {
        ...mockInstance,
        health: 'Degraded',
      };

      renderWithProviders(
        <InstanceDetailView instance={degradedInstance} />
      );

      expect(screen.getByText('Degraded')).toBeInTheDocument();
    });

    it('renders Unhealthy health status', () => {
      const unhealthyInstance: Instance = {
        ...mockInstance,
        health: 'Unhealthy',
      };

      renderWithProviders(
        <InstanceDetailView instance={unhealthyInstance} />
      );

      expect(screen.getByText('Unhealthy')).toBeInTheDocument();
    });

    it('renders Progressing health status', () => {
      const progressingInstance: Instance = {
        ...mockInstance,
        health: 'Progressing',
      };

      renderWithProviders(
        <InstanceDetailView instance={progressingInstance} />
      );

      expect(screen.getByText('Progressing')).toBeInTheDocument();
    });

    it('renders Unknown health status', () => {
      const unknownInstance: Instance = {
        ...mockInstance,
        health: 'Unknown',
      };

      renderWithProviders(
        <InstanceDetailView instance={unknownInstance} />
      );

      expect(screen.getByText('Unknown')).toBeInTheDocument();
    });
  });

  describe('Revision Badge (STORY-400)', () => {
    it('renders revision badge when parentRGD has lastIssuedRevision > 0 and user has rgds:get permission', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 5 } as any,
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.getByText('Rev 5')).toBeInTheDocument();
    });

    it('does not render revision badge when lastIssuedRevision is 0', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 0 } as any,
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByText(/Rev \d+/)).not.toBeInTheDocument();
    });

    it('does not render revision badge when lastIssuedRevision is undefined', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default' } as any,
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByText(/Rev \d+/)).not.toBeInTheDocument();
    });

    it('does not render revision badge when user lacks rgds:get permission', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 3 } as any,
        isLoading: false,
        error: null,
      } as any);
      vi.mocked(useCanI).mockImplementation((resource: string) => {
        if (resource === 'rgds') {
          return { allowed: false, isLoading: false } as any;
        }
        return { allowed: true, isLoading: false } as any;
      });

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByText(/Rev \d+/)).not.toBeInTheDocument();
    });

    it('falls back to instance project label for rgds:get when parentRGD has no project label', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      const projectInstance: Instance = {
        ...mockInstance,
        labels: { ...mockInstance.labels, 'knodex.io/project': 'alpha' },
      };

      renderWithProviders(
        <InstanceDetailView instance={projectInstance} />
      );

      expect(useCanI).toHaveBeenCalledWith('rgds', 'get', 'alpha/test-rgd');
    });

    it('uses parent RGD project label for rgds:get when parentRGD has a project label', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: {
          name: 'test-rgd',
          namespace: 'default',
          lastIssuedRevision: 5,
          labels: { 'knodex.io/project': 'platform' },
        } as any,
        isLoading: false,
        error: null,
      } as any);
      const projectInstance: Instance = {
        ...mockInstance,
        labels: { ...mockInstance.labels, 'knodex.io/project': 'dev' },
      };

      renderWithProviders(
        <InstanceDetailView instance={projectInstance} />
      );

      // Parent RGD's project label ('platform') takes precedence over instance label ('dev')
      expect(useCanI).toHaveBeenCalledWith('rgds', 'get', 'platform/test-rgd');
    });

    it('uses dash fallback for rgds:get when instance has no project label', async () => {
      const { useCanI } = await import('@/hooks/useCanI');

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      // No project label on instance or parentRGD → falls back to '-' per useInstancePermissions
      expect(useCanI).toHaveBeenCalledWith('rgds', 'get', '-');
    });

    it('badge is a button that opens revision diff drawer on click (STORY-401)', async () => {
      const { default: userEvent } = await import('@testing-library/user-event');
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 7 } as any,
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      const badge = screen.getByText('Rev 7').closest('button');
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveAttribute('type', 'button');
      expect(badge).toHaveAttribute('aria-label', 'View changes for revision 7');

      // Drawer should not be open initially
      expect(screen.queryByTestId('revision-diff-drawer')).not.toBeInTheDocument();

      // Click badge — drawer should open
      await userEvent.click(badge!);
      expect(screen.getByTestId('revision-diff-drawer')).toBeInTheDocument();
    });

    it('does not render revision diff drawer when canReadRGD is false', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 5 } as any,
        isLoading: false,
        error: null,
      } as any);
      vi.mocked(useCanI).mockImplementation((resource: string) => {
        if (resource === 'rgds') {
          return { allowed: false, isLoading: false } as any;
        }
        return { allowed: true, isLoading: false } as any;
      });

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByText('Revision Changes')).not.toBeInTheDocument();
    });

    it('does not render revision diff drawer when lastIssuedRevision is falsy', async () => {
      const { useRGD } = await import('@/hooks/useRGDs');
      vi.mocked(useRGD).mockReturnValue({
        data: { name: 'test-rgd', namespace: 'default', lastIssuedRevision: 0 } as any,
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(
        <InstanceDetailView instance={mockInstance} />
      );

      expect(screen.queryByText('Revision Changes')).not.toBeInTheDocument();
    });
  });

  describe('Terminal State Button Behavior (AC2)', () => {
    it('hides Delete button when instance state is DELETING', () => {
      const deletingInstance: Instance = {
        ...mockInstance,
        status: { state: 'DELETING' },
      };

      renderWithProviders(
        <InstanceDetailView instance={deletingInstance} />
      );

      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('disables Edit Spec button when instance state is DELETING', () => {
      const deletingInstance: Instance = {
        ...mockInstance,
        status: { state: 'DELETING' },
      };

      renderWithProviders(
        <InstanceDetailView instance={deletingInstance} />
      );

      const editButton = screen.getByRole('button', { name: /edit spec/i });
      expect(editButton).toBeDisabled();
    });

    it('disables Edit Spec button when instance state is ERROR', () => {
      const errorInstance: Instance = {
        ...mockInstance,
        status: { state: 'ERROR' },
      };

      renderWithProviders(
        <InstanceDetailView instance={errorInstance} />
      );

      const editButton = screen.getByRole('button', { name: /edit spec/i });
      expect(editButton).toBeDisabled();
    });

    it('shows Delete and enabled Edit Spec buttons for non-terminal states', () => {
      const activeInstance: Instance = {
        ...mockInstance,
        status: { state: 'ACTIVE' },
      };

      renderWithProviders(
        <InstanceDetailView instance={activeInstance} />
      );

      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      const editButton = screen.getByRole('button', { name: /edit spec/i });
      expect(editButton).not.toBeDisabled();
    });

    it('shows DELETING state in instance header', () => {
      const deletingInstance: Instance = {
        ...mockInstance,
        status: { state: 'DELETING' },
      };

      renderWithProviders(
        <InstanceDetailView instance={deletingInstance} />
      );

      // InstanceHeaderCard renders kroState when != 'ACTIVE'
      expect(screen.getByText('DELETING')).toBeInTheDocument();
    });
  });
});
