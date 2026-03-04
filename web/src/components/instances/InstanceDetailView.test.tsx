import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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

    it('renders deployment timeline', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

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

  describe('Spec/Status Collapsible Sections', () => {
    it('renders spec section when spec exists', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Use exact name "Spec" to distinguish from "Edit Spec" button
      expect(screen.getByRole('button', { name: /^Spec$/i })).toBeInTheDocument();
    });

    it('expands spec section when clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const specButton = screen.getByRole('button', { name: /^Spec$/i });
      await user.click(specButton);

      expect(screen.getByText(/"replicas": 3/)).toBeInTheDocument();
      expect(screen.getByText(/"image": "nginx:latest"/)).toBeInTheDocument();
    });

    it('collapses spec section when clicked again', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      const specButton = screen.getByRole('button', { name: /^Spec$/i });
      await user.click(specButton);
      expect(screen.getByText(/"replicas": 3/)).toBeInTheDocument();

      await user.click(specButton);
      expect(screen.queryByText(/"replicas": 3/)).not.toBeInTheDocument();
    });

    it('renders unified status card when status exists', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // Status is now rendered via InstanceStatusCard, not a collapsible button
      // The status card shows fields as structured key-values
      expect(screen.getByText('Running')).toBeInTheDocument();
    });

    it('does not render raw JSON status section (replaced by unified card)', () => {
      renderWithProviders(
        <InstanceDetailView instance={mockInstance} onBack={mockOnBack} />
      );

      // The old collapsible status button should no longer exist
      // Only the Spec button should be a collapsible section
      const buttons = screen.getAllByRole('button');
      const statusButtons = buttons.filter(btn => btn.textContent?.trim() === 'Status');
      expect(statusButtons).toHaveLength(0);
    });

    it('does not render spec section when spec is empty', () => {
      const instanceWithoutSpec: Instance = {
        ...mockInstance,
        spec: {},
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutSpec} onBack={mockOnBack} />
      );

      // Collapsible "Spec" section should not render, but "Edit Spec" button may still exist
      expect(screen.queryByRole('button', { name: /^Spec$/i })).not.toBeInTheDocument();
    });

    it('does not render status section when status is empty', () => {
      const instanceWithoutStatus: Instance = {
        ...mockInstance,
        status: {},
      };

      renderWithProviders(
        <InstanceDetailView instance={instanceWithoutStatus} onBack={mockOnBack} />
      );

      expect(screen.queryByRole('button', { name: /status/i })).not.toBeInTheDocument();
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
