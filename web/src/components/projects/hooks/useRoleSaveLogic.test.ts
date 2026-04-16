// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useRoleSaveLogic } from './useRoleSaveLogic';

describe('useRoleSaveLogic', () => {
  const roles = [
    { name: 'admin', description: 'Admin role', policies: ['p1'], groups: ['g1'] },
    { name: 'viewer', description: 'Viewer role', policies: ['p2'], groups: ['g2'] },
  ];
  const defaultOptions = {
    roles,
    onUpdate: vi.fn().mockResolvedValue(undefined),
  };

  it('initializes with no editing role and no pending changes', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    expect(result.current.editingRole).toBeNull();
    expect(result.current.hasUnsavedChanges('admin')).toBe(false);
  });

  it('getRoleData returns original role when no pending changes', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));
    expect(result.current.getRoleData('admin')).toEqual(roles[0]);
  });

  it('tracks pending policy changes', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    act(() => result.current.handlePoliciesChange('admin', ['p1', 'p3']));

    expect(result.current.hasUnsavedChanges('admin')).toBe(true);
    expect(result.current.getRoleData('admin')?.policies).toEqual(['p1', 'p3']);
  });

  it('tracks pending group changes', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    act(() => result.current.handleGroupsChange('viewer', ['g2', 'g3']));

    expect(result.current.hasUnsavedChanges('viewer')).toBe(true);
    expect(result.current.getRoleData('viewer')?.groups).toEqual(['g2', 'g3']);
  });

  it('saves role and clears pending changes', async () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    act(() => result.current.handlePoliciesChange('admin', ['p1', 'p3']));
    await act(async () => {
      await result.current.handleSaveRole('admin');
    });

    expect(defaultOptions.onUpdate).toHaveBeenCalled();
    expect(result.current.hasUnsavedChanges('admin')).toBe(false);
    expect(result.current.editingRole).toBeNull();
  });

  it('cancels edit and clears pending changes for the role', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    act(() => {
      result.current.setEditingRole('admin');
      result.current.handlePoliciesChange('admin', ['p1', 'p3']);
    });
    act(() => result.current.handleCancelEdit('admin'));

    expect(result.current.hasUnsavedChanges('admin')).toBe(false);
    expect(result.current.editingRole).toBeNull();
  });

  it('keeps pending changes when handleSaveRole fails', async () => {
    const options = {
      ...defaultOptions,
      onUpdate: vi.fn().mockRejectedValue(new Error('save failed')),
    };
    const { result } = renderHook(() => useRoleSaveLogic(options));

    act(() => result.current.handlePoliciesChange('admin', ['p1', 'p3']));
    await act(async () => {
      await result.current.handleSaveRole('admin');
    });

    expect(result.current.hasUnsavedChanges('admin')).toBe(true);
    expect(result.current.getRoleData('admin')?.policies).toEqual(['p1', 'p3']);
  });

  it('clearAllPendingChanges clears all pending changes', () => {
    const { result } = renderHook(() => useRoleSaveLogic(defaultOptions));

    act(() => {
      result.current.handlePoliciesChange('admin', ['new']);
      result.current.handleGroupsChange('viewer', ['new']);
    });
    act(() => result.current.clearAllPendingChanges());

    expect(result.current.hasUnsavedChanges('admin')).toBe(false);
    expect(result.current.hasUnsavedChanges('viewer')).toBe(false);
  });
});
