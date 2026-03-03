import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ExternalRefSelector } from './ExternalRefSelector';
import { TooltipProvider } from '@/components/ui/tooltip';
import { FormProvider, useForm, type UseFormReturn } from 'react-hook-form';
import type { ReactNode } from 'react';

// Mock the useK8sResources hook
const mockUseK8sResources = vi.fn();
vi.mock('@/hooks/useRGDs', () => ({
  useK8sResources: (...args: unknown[]) => mockUseK8sResources(...args),
}));

const defaultProps = {
  name: 'externalRef.permissionResults',
  apiVersion: 'v1',
  kind: 'ConfigMap',
  autoFillFields: { name: 'name', namespace: 'namespace' },
  label: 'Permission Results',
};

// Capture form methods for assertions
let formMethods: UseFormReturn | null = null;

// Wrapper that provides FormProvider context
function FormWrapper({ children, defaultValues }: { children: ReactNode; defaultValues?: Record<string, unknown> }) {
  const Wrapper = () => {
    const methods = useForm({ defaultValues: defaultValues || {} });
    formMethods = methods;
    return (
      <FormProvider {...methods}>
        <TooltipProvider>
          {children}
        </TooltipProvider>
      </FormProvider>
    );
  };
  return <Wrapper />;
}

function renderSelector(props = {}, defaultValues?: Record<string, unknown>) {
  formMethods = null;
  return render(
    <FormWrapper defaultValues={defaultValues}>
      <ExternalRefSelector {...defaultProps} {...props} />
    </FormWrapper>
  );
}

describe('ExternalRefSelector', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseK8sResources.mockReturnValue({
      data: [
        { name: 'config-a', namespace: 'default' },
        { name: 'config-b', namespace: 'production' },
      ],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
    });
  });

  it('uses deploymentNamespace for resource filtering', () => {
    renderSelector({
      deploymentNamespace: 'production',
    });

    expect(mockUseK8sResources).toHaveBeenCalledWith('v1', 'ConfigMap', 'production', true);
  });

  it('disables query when no deploymentNamespace is available', () => {
    renderSelector({
      deploymentNamespace: undefined,
    });

    expect(mockUseK8sResources).toHaveBeenCalledWith('v1', 'ConfigMap', undefined, false);
  });

  it('shows "Select a deployment namespace" when no namespace', () => {
    mockUseK8sResources.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
    });

    renderSelector({
      deploymentNamespace: undefined,
    });

    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).toBeDisabled();
    expect(select).toHaveTextContent('Select a deployment namespace to view available ConfigMaps');
  });

  it('shows resource count with namespace', () => {
    renderSelector({
      deploymentNamespace: 'default',
    });

    expect(screen.getByText('2 ConfigMaps available in default')).toBeInTheDocument();
  });

  it('shows resources when namespace is available', () => {
    renderSelector({
      deploymentNamespace: 'default',
    });

    expect(screen.getByText('Select a ConfigMap...')).toBeInTheDocument();
    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).not.toBeDisabled();
  });

  it('auto-fills both name and namespace on selection', () => {
    renderSelector({
      deploymentNamespace: 'default',
    });

    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).not.toBeDisabled();

    // Select a resource from the dropdown
    fireEvent.change(select, { target: { value: 'config-a' } });

    // Verify the select reflects the chosen value
    expect(select).toHaveValue('config-a');

    // Verify both name AND namespace were set in the form context
    expect(formMethods).not.toBeNull();
    const values = formMethods!.getValues();
    expect(values.externalRef.permissionResults.name).toBe('config-a');
    expect(values.externalRef.permissionResults.namespace).toBe('default');
  });

  it('clears both name and namespace when selection is cleared', () => {
    renderSelector({
      deploymentNamespace: 'default',
    });

    const select = screen.getByTestId('input-externalRef.permissionResults');

    // Select a resource first
    fireEvent.change(select, { target: { value: 'config-a' } });
    expect(formMethods!.getValues().externalRef.permissionResults.name).toBe('config-a');

    // Clear selection
    fireEvent.change(select, { target: { value: '' } });

    // Verify both fields were cleared
    const values = formMethods!.getValues();
    expect(values.externalRef.permissionResults.name).toBe('');
    expect(values.externalRef.permissionResults.namespace).toBe('');
  });

  it('shows loading state while fetching resources', () => {
    mockUseK8sResources.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: true,
    });

    renderSelector({ deploymentNamespace: 'default' });

    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).toBeDisabled();
    expect(select).toHaveTextContent('Loading ConfigMaps...');
  });

  it('shows generic error with retry option on fetch failure', () => {
    const mockRefetch = vi.fn();
    mockUseK8sResources.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('network error'),
      refetch: mockRefetch,
      isFetching: false,
    });

    renderSelector({ deploymentNamespace: 'default' });

    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).toBeDisabled();
    expect(select).toHaveTextContent('Failed to load ConfigMaps');

    // Error message should be visible
    expect(screen.getByTestId('error-externalRef.permissionResults')).toBeInTheDocument();
    expect(screen.getByTestId('error-fetch-externalRef.permissionResults')).toBeInTheDocument();
  });

  it('shows forbidden error for 403 responses', () => {
    mockUseK8sResources.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: { response: { status: 403 } },
      refetch: vi.fn(),
      isFetching: false,
    });

    renderSelector({ deploymentNamespace: 'default' });

    expect(screen.getByTestId('error-forbidden-externalRef.permissionResults')).toBeInTheDocument();
  });

  it('shows empty state when no resources found', () => {
    mockUseK8sResources.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
    });

    renderSelector({ deploymentNamespace: 'default' });

    const select = screen.getByTestId('input-externalRef.permissionResults');
    expect(select).toHaveTextContent('No ConfigMaps found in default');
  });
});
