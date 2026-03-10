// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DeployPage } from './DeployPage';
import type { CatalogRGD } from '@/types/rgd';

// Mock hooks
vi.mock('@/hooks/useRGDs', () => ({
  useRGDSchema: vi.fn(),
  useCreateInstance: vi.fn(),
}));

vi.mock('@/hooks/useRepositories', () => ({
  useRepositories: vi.fn(),
}));

vi.mock('@/hooks/useProjects', () => ({
  useProjects: vi.fn(),
}));

vi.mock('@/hooks/useNamespaces', () => ({
  useProjectNamespaces: vi.fn(),
}));

vi.mock('@/hooks/useCanI', () => ({
  useCanI: vi.fn(),
}));

// Mock child components
vi.mock('./DeploymentModeSelector', () => ({
  DeploymentModeSelector: () => <div data-testid="deployment-mode-selector">Deployment Mode Selector</div>,
}));

vi.mock('./FormField', () => ({
  FormField: ({ name }: { name: string }) => <div data-testid={`form-field-${name}`}>Form Field: {name}</div>,
}));

vi.mock('./YAMLPreview', () => ({
  YAMLPreview: () => <div data-testid="yaml-preview">YAML Preview</div>,
}));

const mockRGD: CatalogRGD = {
  name: 'test-rgd',
  namespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGroup',
  description: 'A test RGD for deployment',
  version: 'v1',
  category: 'Database',
  icon: '🗄️',
  tags: [],
  labels: {},
  instances: 0,
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
};

const mockSchema = {
  group: 'example.com',
  version: 'v1',
  kind: 'TestResource',
  description: 'Test resource schema',
  properties: {
    replicas: {
      type: 'integer',
      description: 'Number of replicas',
      default: 1,
    },
    image: {
      type: 'string',
      description: 'Container image',
    },
  },
  required: ['image'],
  conditionalSections: [],
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

describe('DeployPage', () => {
  const defaultProps = {
    rgd: mockRGD,
    onBack: vi.fn(),
    onDeploySuccess: vi.fn(),
  };

  beforeEach(async () => {
    vi.clearAllMocks();

    const { useRGDSchema, useCreateInstance } = await import('@/hooks/useRGDs');
    const { useRepositories } = await import('@/hooks/useRepositories');
    const { useProjects } = await import('@/hooks/useProjects');
    const { useProjectNamespaces } = await import('@/hooks/useNamespaces');
    const { useCanI } = await import('@/hooks/useCanI');

    vi.mocked(useRGDSchema).mockReturnValue({
      data: { crdFound: true, schema: mockSchema },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useCreateInstance).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      error: null,
    } as any);

    vi.mocked(useRepositories).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useProjects).mockReturnValue({
      data: { items: [{ name: 'default', description: 'Default project' }] },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useProjectNamespaces).mockReturnValue({
      data: { namespaces: ['default', 'production'] },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
    } as any);
  });

  describe('Loading State', () => {
    it('renders loading state when schema is loading', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Loading schema...')).toBeInTheDocument();
    });

    it('displays loading spinner when schema is loading', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      const loader = screen.getByText('Loading schema...')
        .closest('div')
        ?.querySelector('svg');
      expect(loader).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('renders error state when schema fetch fails', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Failed to fetch schema'),
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Cannot load deployment form')).toBeInTheDocument();
      expect(screen.getByText('Failed to fetch schema')).toBeInTheDocument();
    });

    it('renders error state when CRD is not found', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: { crdFound: false, schema: null, error: 'CRD not found' },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Cannot load deployment form')).toBeInTheDocument();
      expect(screen.getByText('CRD not found')).toBeInTheDocument();
    });

    it('shows return to catalog button on error', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Schema error'),
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Return to catalog')).toBeInTheDocument();
    });
  });

  describe('Basic Rendering', () => {
    it('renders the page header with RGD name', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Deploy test-rgd')).toBeInTheDocument();
      expect(screen.getByText('Create a new instance of this resource')).toBeInTheDocument();
    });

    it('renders the page header with title when title annotation is set', () => {
      const rgdWithTitle: CatalogRGD = {
        ...mockRGD,
        title: 'My Custom Display Title',
      };
      renderWithProviders(<DeployPage {...defaultProps} rgd={rgdWithTitle} />);

      expect(screen.getByText('Deploy My Custom Display Title')).toBeInTheDocument();
      expect(screen.getByText('test-rgd')).toBeInTheDocument(); // K8s name shown as subtitle
    });

    it('displays API version and kind badges', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('example.com/v1')).toBeInTheDocument();
      expect(screen.getByText('TestResource')).toBeInTheDocument();
    });

    it('renders back button', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Back to details')).toBeInTheDocument();
    });

    it('renders instance details section', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Instance Details')).toBeInTheDocument();
      expect(screen.getByLabelText(/Instance Name/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Project/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Namespace/)).toBeInTheDocument();
    });

    it('renders deployment options section', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Deployment Options')).toBeInTheDocument();
      expect(screen.getByTestId('deployment-mode-selector')).toBeInTheDocument();
    });

    it('renders configuration section', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Configuration')).toBeInTheDocument();
    });

    it('renders YAML preview', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByTestId('yaml-preview')).toBeInTheDocument();
    });

    it('renders deploy button', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByTestId('deploy-submit-button')).toBeInTheDocument();
    });
  });

  describe('Form Fields', () => {
    it('renders instance name input field', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      const instanceNameInput = screen.getByTestId('input-instanceName');
      expect(instanceNameInput).toBeInTheDocument();
      expect(instanceNameInput).toHaveAttribute('placeholder', 'my-instance');
    });

    it('renders project selector', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByTestId('input-project')).toBeInTheDocument();
    });

    it('renders namespace selector', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByTestId('input-namespace')).toBeInTheDocument();
    });

    it('displays schema description when available', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('Test resource schema')).toBeInTheDocument();
    });

    it('shows required field indicators', () => {
      renderWithProviders(<DeployPage {...defaultProps} />);

      const requiredMarkers = screen.getAllByText('*');
      expect(requiredMarkers.length).toBeGreaterThan(0);
    });
  });

  describe('Permission Handling', () => {
    it('shows permission warning when user cannot deploy', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('deploy-permission-warning')).toBeInTheDocument();
        expect(screen.getByText('Cannot deploy to this project')).toBeInTheDocument();
      });
    });

    it('does not show permission warning when user can deploy', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.queryByTestId('deploy-permission-warning')).not.toBeInTheDocument();
    });

    it('disables deploy button when user lacks permission', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      await waitFor(() => {
        const deployButton = screen.getByTestId('deploy-submit-button');
        expect(deployButton).toBeDisabled();
        expect(deployButton).toHaveTextContent('No Permission to Deploy');
      });
    });

    it('shows "Checking permissions..." while permissions are loading', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: undefined,
        isLoading: true,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      await waitFor(() => {
        const deployButton = screen.getByTestId('deploy-submit-button');
        expect(deployButton).toBeDisabled();
        expect(deployButton).toHaveTextContent('Checking permissions...');
      });
    });

    it('shows deploy button (not "No Permission") when permission check errors (optimistic)', async () => {
      const { useCanI } = await import('@/hooks/useCanI');
      vi.mocked(useCanI).mockReturnValue({
        allowed: undefined,
        isLoading: false,
        isError: true,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      await waitFor(() => {
        const deployButton = screen.getByTestId('deploy-submit-button');
        // Button should show deploy text, not "No Permission to Deploy"
        expect(deployButton).toHaveTextContent('Deploy Instance');
        // Button should NOT show permission warning — isError is not explicit deny
        expect(screen.queryByTestId('deploy-permission-warning')).not.toBeInTheDocument();
      });
    });
  });

  describe('Empty Schema State', () => {
    it('shows message when no configuration options are required', async () => {
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: {
          crdFound: true,
          schema: {
            ...mockSchema,
            properties: {},
          },
        },
        isLoading: false,
        error: null,
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} />);

      expect(screen.getByText('No configuration options required for this resource.')).toBeInTheDocument();
    });
  });

  describe('Back Navigation', () => {
    it('calls onBack when back button is clicked', () => {
      const onBack = vi.fn();
      renderWithProviders(<DeployPage {...defaultProps} onBack={onBack} />);

      const backButton = screen.getByText('Back to details');
      backButton.click();

      expect(onBack).toHaveBeenCalledTimes(1);
    });

    it('calls onBack when return to catalog is clicked on error', async () => {
      const onBack = vi.fn();
      const { useRGDSchema } = await import('@/hooks/useRGDs');
      vi.mocked(useRGDSchema).mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Schema error'),
      } as any);

      renderWithProviders(<DeployPage {...defaultProps} onBack={onBack} />);

      const returnButton = screen.getByText('Return to catalog');
      returnButton.click();

      expect(onBack).toHaveBeenCalledTimes(1);
    });
  });
});
