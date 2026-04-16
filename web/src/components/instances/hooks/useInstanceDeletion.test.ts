// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useInstanceDeletion } from './useInstanceDeletion';
import type { Instance } from '@/types/rgd';

vi.mock('@/hooks/useInstances', () => ({
  useDeleteInstance: vi.fn(),
}));

vi.mock('@/lib/toast-helpers', () => ({
  showSuccessToast: vi.fn(),
}));

const mockInstance: Instance = {
  name: 'test-instance',
  namespace: 'test-namespace',
  rgdName: 'test-rgd',
  rgdNamespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'TestResource',
  health: 'Healthy',
  conditions: [],
  createdAt: '2024-01-15T10:30:00Z',
  deploymentMode: 'direct',
};

describe('useInstanceDeletion', () => {
  const mockMutateAsync = vi.fn();

  beforeEach(async () => {
    vi.clearAllMocks();
    const { useDeleteInstance } = await import('@/hooks/useInstances');
    vi.mocked(useDeleteInstance).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      error: null,
      reset: vi.fn(),
    } as any);
  });

  it('returns handleDelete and deleteInstance', () => {
    const { result } = renderHook(() => useInstanceDeletion(mockInstance));

    expect(result.current.handleDelete).toBeInstanceOf(Function);
    expect(result.current.deleteInstance).toBeDefined();
  });

  it('calls mutateAsync with correct params on delete', async () => {
    mockMutateAsync.mockResolvedValue({});
    const onDeleted = vi.fn();
    const { result } = renderHook(() => useInstanceDeletion(mockInstance, onDeleted));

    await act(async () => {
      await result.current.handleDelete();
    });

    expect(mockMutateAsync).toHaveBeenCalledWith({
      namespace: 'test-namespace',
      kind: 'TestResource',
      name: 'test-instance',
    });
    const { showSuccessToast } = await import('@/lib/toast-helpers');
    expect(showSuccessToast).toHaveBeenCalledWith('"test-instance" deleted');
    expect(onDeleted).toHaveBeenCalled();
  });

  it('does not call onDeleted when mutation fails', async () => {
    mockMutateAsync.mockRejectedValue(new Error('fail'));
    const onDeleted = vi.fn();
    const { result } = renderHook(() => useInstanceDeletion(mockInstance, onDeleted));

    await act(async () => {
      await result.current.handleDelete();
    });

    expect(onDeleted).not.toHaveBeenCalled();
  });
});
