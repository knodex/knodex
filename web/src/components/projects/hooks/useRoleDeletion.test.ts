// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useRoleDeletion } from './useRoleDeletion';

describe('useRoleDeletion', () => {
  const defaultOptions = {
    roles: [
      { name: 'admin', policies: [], groups: [] },
      { name: 'viewer', policies: [], groups: [] },
    ],
    onUpdate: vi.fn().mockResolvedValue(undefined),
    onSuccess: vi.fn(),
  };

  it('initializes with no role to delete', () => {
    const { result } = renderHook(() => useRoleDeletion(defaultOptions));

    expect(result.current.roleToDelete).toBeNull();
    expect(result.current.isDeleting).toBe(false);
    expect(result.current.deleteRoleError).toBeNull();
  });

  it('sets roleToDelete on handleDeleteRole', () => {
    const { result } = renderHook(() => useRoleDeletion(defaultOptions));

    act(() => result.current.handleDeleteRole('viewer'));
    expect(result.current.roleToDelete).toBe('viewer');
  });

  it('calls onUpdate with filtered roles on confirm', async () => {
    const { result } = renderHook(() => useRoleDeletion(defaultOptions));

    act(() => result.current.handleDeleteRole('viewer'));
    await act(async () => {
      await result.current.confirmDeleteRole();
    });

    expect(defaultOptions.onUpdate).toHaveBeenCalledWith({
      roles: [{ name: 'admin', policies: [], groups: [] }],
    });
    expect(result.current.roleToDelete).toBeNull();
    expect(defaultOptions.onSuccess).toHaveBeenCalled();
  });

  it('clears state on cancelDeleteRole', () => {
    const { result } = renderHook(() => useRoleDeletion(defaultOptions));

    act(() => result.current.handleDeleteRole('viewer'));
    act(() => result.current.cancelDeleteRole());

    expect(result.current.roleToDelete).toBeNull();
    expect(result.current.deleteRoleError).toBeNull();
  });
});
