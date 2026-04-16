// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useInstanceMetadata } from './useInstanceMetadata';
import type { Instance } from '@/types/rgd';

vi.mock('@/hooks/useRGDs', () => ({
  useRGD: vi.fn(),
}));

vi.mock('@/hooks/useHistory', () => ({
  useInstanceEvents: vi.fn(),
}));

vi.mock('../instance-utils', () => ({
  getInstanceUrl: vi.fn().mockReturnValue(null),
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

describe('useInstanceMetadata', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const { useRGD } = await import('@/hooks/useRGDs');
    const { useInstanceEvents } = await import('@/hooks/useHistory');

    vi.mocked(useRGD).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useInstanceEvents).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as any);
  });

  it('returns isGitOps as false for direct deployment', () => {
    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.isGitOps).toBe(false);
  });

  it('returns isGitOps as true for gitops deployment', () => {
    const gitopsInstance: Instance = { ...mockInstance, deploymentMode: 'gitops' };
    const { result } = renderHook(() => useInstanceMetadata(gitopsInstance));
    expect(result.current.isGitOps).toBe(true);
  });

  it('returns isGitOps as true for hybrid deployment', () => {
    const hybridInstance: Instance = { ...mockInstance, deploymentMode: 'hybrid' };
    const { result } = renderHook(() => useInstanceMetadata(hybridInstance));
    expect(result.current.isGitOps).toBe(true);
  });

  it('derives kroState from instance status', () => {
    const deletingInstance: Instance = { ...mockInstance, status: { state: 'DELETING' } };
    const { result } = renderHook(() => useInstanceMetadata(deletingInstance));
    expect(result.current.kroState).toBe('DELETING');
    expect(result.current.isDeleting).toBe(true);
    expect(result.current.isTerminal).toBe(true);
  });

  it('returns empty kroState when no status', () => {
    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.kroState).toBe('');
    expect(result.current.isDeleting).toBe(false);
    expect(result.current.isTerminal).toBe(false);
  });

  it('returns hasSpec true when spec has keys', () => {
    const specInstance: Instance = { ...mockInstance, spec: { replicas: 3 } };
    const { result } = renderHook(() => useInstanceMetadata(specInstance));
    expect(result.current.hasSpec).toBe(true);
  });

  it('returns hasSpec false when spec is empty or absent', () => {
    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.hasSpec).toBe(false);

    const emptySpec: Instance = { ...mockInstance, spec: {} };
    const { result: r2 } = renderHook(() => useInstanceMetadata(emptySpec));
    expect(r2.current.hasSpec).toBe(false);
  });

  it('returns eventsCount from useInstanceEvents data', async () => {
    const { useInstanceEvents } = await import('@/hooks/useHistory');
    vi.mocked(useInstanceEvents).mockReturnValue({
      data: { events: [{ id: '1' }, { id: '2' }] },
      isLoading: false,
      error: null,
    } as any);

    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.eventsCount).toBe(2);
  });

  it('returns eventsCount as 0 when no events data', () => {
    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.eventsCount).toBe(0);
  });

  it('returns externalRefCount for valid externalRef entries', () => {
    const extRefInstance: Instance = {
      ...mockInstance,
      spec: {
        externalRef: {
          myService: { name: 'svc-1', namespace: 'default' },
          other: { name: 'svc-2', namespace: 'prod' },
          invalid: 'not-an-object',
          missingName: { namespace: 'dev' },
        },
      },
    };
    const { result } = renderHook(() => useInstanceMetadata(extRefInstance));
    expect(result.current.externalRefCount).toBe(2);
  });

  it('returns externalRefCount as 0 when no externalRef in spec', () => {
    const { result } = renderHook(() => useInstanceMetadata(mockInstance));
    expect(result.current.externalRefCount).toBe(0);
  });
});
