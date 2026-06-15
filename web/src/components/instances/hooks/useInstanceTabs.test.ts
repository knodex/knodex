// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useInstanceTabs } from './useInstanceTabs';
import type { InstanceTabCounts } from './useInstanceTabs';
import type { Tab, ConditionalTab } from '@/hooks/useDynamicTabs';
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
  updatedAt: '2024-01-15T11:00:00Z',
  deploymentMode: 'direct',
};

const zeroCounts: InstanceTabCounts = {
  events: 0,
  externalRefs: 0,
  resourcesReady: 0,
  resourcesTotal: 0,
  history: 0,
};

describe('useInstanceTabs', () => {
  const mockSetActiveTab = vi.fn();
  let capturedBaseTabs: Tab<InstanceTabId>[] = [];
  let capturedConditionalTabs: ConditionalTab<InstanceTabId>[] = [];

  beforeEach(async () => {
    vi.clearAllMocks();
    capturedBaseTabs = [];
    capturedConditionalTabs = [];

    const { useRGDList } = await import('@/hooks/useRGDs');
    const { useDynamicTabs } = await import('@/hooks/useDynamicTabs');

    vi.mocked(useRGDList).mockReturnValue({
      data: { items: [], totalCount: 0 },
      isLoading: false,
      error: null,
    } as any);

    vi.mocked(useDynamicTabs).mockImplementation((baseTabs, conditionalTabs) => {
      capturedBaseTabs = baseTabs as Tab<InstanceTabId>[];
      capturedConditionalTabs = conditionalTabs as ConditionalTab<InstanceTabId>[];
      return {
        tabs: [{ id: 'status', label: 'Overview', icon: null }],
        activeTab: 'status',
        setActiveTab: mockSetActiveTab,
      };
    });
  });

  it('returns tabs, activeTab, and setActiveTab', () => {
    const { result } = renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    expect(result.current.tabs).toBeDefined();
    expect(result.current.activeTab).toBe('status');
    expect(result.current.setActiveTab).toBe(mockSetActiveTab);
  });

  it('calls useRGDList with extendsKind and pageSize 100', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    expect(useRGDList).toHaveBeenCalledWith({ extendsKind: 'TestResource', pageSize: 100 });
  });

  it('calls useRGDList with undefined when kind is falsy', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    const noKindInstance: Instance = { ...mockInstance, kind: '' };
    renderHook(() => useInstanceTabs(noKindInstance, zeroCounts, false));

    expect(useRGDList).toHaveBeenCalledWith(undefined);
  });

  it('base tabs are Overview, Resources, Events, History in order', () => {
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    expect(capturedBaseTabs.map((t) => t.id)).toEqual([
      'status',
      'children',
      'events',
      'deployment-history',
    ]);
    expect(capturedBaseTabs.map((t) => t.label)).toEqual([
      'Overview',
      'Resources',
      'Events',
      'History',
    ]);
  });

  it('includes addons tab with count pill when addOnsCount > 0', async () => {
    const { useRGDList } = await import('@/hooks/useRGDs');
    vi.mocked(useRGDList).mockReturnValue({
      data: { items: [], totalCount: 3 },
      isLoading: false,
      error: null,
    } as any);

    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    const addonsTab = capturedConditionalTabs.find(t => t.tab.id === 'addons');
    expect(addonsTab?.condition).toBe(true);
    expect(addonsTab?.tab.label).toBe('Add-ons');
    expect(addonsTab?.tab.count).toBe('3');
  });

  it('addons tab condition is false when addOnsCount is 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    const addonsTab = capturedConditionalTabs.find(t => t.tab.id === 'addons');
    expect(addonsTab?.condition).toBe(false);
  });

  it('includes References tab with count pill when externalRefs > 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, { ...zeroCounts, externalRefs: 2 }, false));

    const refsTab = capturedConditionalTabs.find(t => t.tab.id === 'external-refs');
    expect(refsTab?.condition).toBe(true);
    expect(refsTab?.tab.label).toBe('References');
    expect(refsTab?.tab.count).toBe('2');
  });

  it('external-refs tab condition is false when externalRefs is 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    const refsTab = capturedConditionalTabs.find(t => t.tab.id === 'external-refs');
    expect(refsTab?.condition).toBe(false);
  });

  it('includes spec tab when hasSpec is true', () => {
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, true));

    const specTab = capturedConditionalTabs.find(t => t.tab.id === 'spec');
    expect(specTab?.condition).toBe(true);
  });

  it('spec tab condition is false when hasSpec is false', () => {
    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));

    const specTab = capturedConditionalTabs.find(t => t.tab.id === 'spec');
    expect(specTab?.condition).toBe(false);
  });

  it('shows events count pill when events > 0 and none when 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, { ...zeroCounts, events: 5 }, false));
    expect(capturedBaseTabs.find(t => t.id === 'events')?.count).toBe('5');

    renderHook(() => useInstanceTabs(mockInstance, zeroCounts, false));
    expect(capturedBaseTabs.find(t => t.id === 'events')?.count).toBeUndefined();
  });

  it('shows ready/total resources pill with warn variant when not all ready', () => {
    renderHook(() =>
      useInstanceTabs(mockInstance, { ...zeroCounts, resourcesReady: 6, resourcesTotal: 7 }, false)
    );

    const childrenTab = capturedBaseTabs.find(t => t.id === 'children');
    expect(childrenTab?.count).toBe('6/7');
    expect(childrenTab?.countVariant).toBe('warn');
  });

  it('shows default-variant resources pill when all ready', () => {
    renderHook(() =>
      useInstanceTabs(mockInstance, { ...zeroCounts, resourcesReady: 7, resourcesTotal: 7 }, false)
    );

    const childrenTab = capturedBaseTabs.find(t => t.id === 'children');
    expect(childrenTab?.count).toBe('7/7');
    expect(childrenTab?.countVariant).toBe('default');
  });

  it('shows history count pill when history > 0', () => {
    renderHook(() => useInstanceTabs(mockInstance, { ...zeroCounts, history: 40 }, false));

    expect(capturedBaseTabs.find(t => t.id === 'deployment-history')?.count).toBe('40');
  });
});
