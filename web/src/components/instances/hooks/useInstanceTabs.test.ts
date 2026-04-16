// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useInstanceTabs } from './useInstanceTabs';
import type { ConditionalTab } from '@/hooks/useDynamicTabs';
import type { InstanceTabId } from './useInstanceTabs';
import type { Instance } from '@/types/rgd';

vi.mock('@/hooks/useRGDs', () => ({
  useRGDList: vi.fn(),
}));

vi.mock('@/hooks/useDynamicTabs', () => ({
  useDynamicTabs: vi.fn(),
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
  spec: { replicas: 3 },
  createdAt: '2024-01-15T10:30:00Z',
  deploymentMode: 'direct',
};

describe('useInstanceTabs', () => {
  const mockSetActiveTab = vi.fn();
  let capturedConditionalTabs: ConditionalTab<InstanceTabId>[] = [];

  beforeEach(async () => {
    vi.clearAllMocks();
    capturedConditionalTabs = [];

    const { useRGDList } = await import('@/hooks/useRGDs');
    const { useDynamicTabs } = await import('@/hooks/useDynamicTabs');

    vi.mocked(useRGDList).mockReturnValue({
      data: { items: [], totalCount: 0 },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useDynamicTabs).mockImplementation((_baseTabs, conditionalTabs) => {
      capturedConditionalTabs = conditionalTabs as ConditionalTab<InstanceTabId>[];
      return {
        tabs: [{ id: 'status', label: 'Status', icon: null }],
        activeTab: 'status',
        setActiveTab: mockSetActiveTab,
      };
    });
  });

  it('returns tabs, activeTab, and setActiveTab', () => {
    const { result } = renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    expect(result.current.tabs).toBeDefined();
    expect(result.current.activeTab).toBe('status');
    expect(result.current.setActiveTab).toBe(mockSetActiveTab);
  });

  it('calls useRGDList with extendsKind and pageSize 100', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    expect(useRGDList).toHaveBeenCalledWith({ extendsKind: 'TestResource', pageSize: 100 });
  });

  it('calls useRGDList with undefined when kind is falsy', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    const noKindInstance: Instance = { ...mockInstance, kind: '' };
    renderHook(() => useInstanceTabs(noKindInstance, 0, 0, false));

    expect(useRGDList).toHaveBeenCalledWith(undefined);
  });

  it('includes addons tab with correct label when addOnsCount > 0', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    vi.mocked(useRGDList).mockReturnValue({
      data: { items: [], totalCount: 3 },
      isLoading: false,
      error: null,
    } as any);

    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const addonsTab = capturedConditionalTabs.find(t => t.tab.id === 'addons');
    expect(addonsTab?.condition).toBe(true);
    expect(addonsTab?.tab.label).toBe('Add-ons (3)');
  });

  it('addons tab condition is false when addOnsCount is 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const addonsTab = capturedConditionalTabs.find(t => t.tab.id === 'addons');
    expect(addonsTab?.condition).toBe(false);
  });

  it('includes external-refs tab when externalRefCount > 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 2, false));

    const refsTab = capturedConditionalTabs.find(t => t.tab.id === 'external-refs');
    expect(refsTab?.condition).toBe(true);
    expect(refsTab?.tab.label).toBe('External References (2)');
  });

  it('external-refs tab condition is false when externalRefCount is 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const refsTab = capturedConditionalTabs.find(t => t.tab.id === 'external-refs');
    expect(refsTab?.condition).toBe(false);
  });

  it('includes spec tab when hasSpec is true', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, true));

    const specTab = capturedConditionalTabs.find(t => t.tab.id === 'spec');
    expect(specTab?.condition).toBe(true);
  });

  it('spec tab condition is false when hasSpec is false', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const specTab = capturedConditionalTabs.find(t => t.tab.id === 'spec');
    expect(specTab?.condition).toBe(false);
  });

  it('shows events count in tab label when eventsCount > 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, 5, 0, false));

    const eventsTab = capturedConditionalTabs.find(t => t.tab.id === 'events');
    expect(eventsTab?.tab.label).toBe('Events (5)');
    expect(eventsTab?.condition).toBe(true);
  });

  it('shows plain "Events" label when eventsCount is 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const eventsTab = capturedConditionalTabs.find(t => t.tab.id === 'events');
    expect(eventsTab?.tab.label).toBe('Events');
    expect(eventsTab?.condition).toBe(true);
  });

  it('children tab is always included', () => {
    renderHook(() => useInstanceTabs(mockInstance, 0, 0, false));

    const childrenTab = capturedConditionalTabs.find(t => t.tab.id === 'children');
    expect(childrenTab?.condition).toBe(true);
  });
});
