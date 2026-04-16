// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useInstancePermissions } from './useInstancePermissions';
import type { Instance } from '@/types/rgd';

vi.mock('@/hooks/useCanI', () => ({
  useCanI: vi.fn(),
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

describe('useInstancePermissions', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const { useCanI } = await import('@/hooks/useCanI');

    vi.mocked(useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    } as any);
  });

  it('returns permission flags from useCanI', () => {
    const { result } = renderHook(() => useInstancePermissions(mockInstance));

    expect(result.current.canDelete).toBe(true);
    expect(result.current.canUpdate).toBe(true);
    expect(result.current.canReadRGD).toBe(true);
  });

  it('uses project label as permission object for namespaced instances', async () => {
    const { useCanI } = await import('@/hooks/useCanI');
    const instanceWithProject: Instance = {
      ...mockInstance,
      labels: { 'knodex.io/project': 'alpha' },
    };

    renderHook(() => useInstancePermissions(instanceWithProject));

    expect(useCanI).toHaveBeenCalledWith('instances', 'delete', 'alpha');
    expect(useCanI).toHaveBeenCalledWith('instances', 'update', 'alpha');
  });

  it('falls back to dash when instance has no project label', async () => {
    const { useCanI } = await import('@/hooks/useCanI');

    renderHook(() => useInstancePermissions(mockInstance));

    expect(useCanI).toHaveBeenCalledWith('instances', 'delete', '-');
    expect(useCanI).toHaveBeenCalledWith('instances', 'update', '-');
  });

  it('uses project label for cluster-scoped instances', async () => {
    const { useCanI } = await import('@/hooks/useCanI');
    const clusterInstance: Instance = {
      ...mockInstance,
      namespace: '',
      labels: { 'knodex.io/project': 'alpha' },
    };

    renderHook(() => useInstancePermissions(clusterInstance));

    expect(useCanI).toHaveBeenCalledWith('instances', 'delete', 'alpha');
    expect(useCanI).toHaveBeenCalledWith('instances', 'update', 'alpha');
  });

  it('falls back to dash when no project label on cluster-scoped instance', async () => {
    const { useCanI } = await import('@/hooks/useCanI');
    const noLabelInstance: Instance = {
      ...mockInstance,
      namespace: '',
      labels: {},
    };

    renderHook(() => useInstancePermissions(noLabelInstance));

    expect(useCanI).toHaveBeenCalledWith('instances', 'delete', '-');
  });

  it('uses parent RGD project label for rgds:get permission when parentRGD is passed', async () => {
    const { useCanI } = await import('@/hooks/useCanI');
    const parentRGD = { labels: { 'knodex.io/project': 'platform' } };

    renderHook(() => useInstancePermissions(mockInstance, parentRGD));

    expect(useCanI).toHaveBeenCalledWith('rgds', 'get', 'platform/test-rgd');
  });

  it('falls back to instance project label for rgds:get when parentRGD has no project label', async () => {
    const { useCanI } = await import('@/hooks/useCanI');
    const instanceWithLabel: Instance = {
      ...mockInstance,
      labels: { 'knodex.io/project': 'alpha' },
    };

    renderHook(() => useInstancePermissions(instanceWithLabel, { labels: {} }));

    expect(useCanI).toHaveBeenCalledWith('rgds', 'get', 'alpha/test-rgd');
  });

  it('uses dash for rgds:get when parentRGD is undefined and no project label', async () => {
    const { useCanI } = await import('@/hooks/useCanI');

    renderHook(() => useInstancePermissions(mockInstance, undefined));

    expect(useCanI).toHaveBeenCalledWith('rgds', 'get', '-');
  });
});
