// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useRoleAddition } from './useRoleAddition';

describe('useRoleAddition', () => {
  const defaultOptions = {
    projectName: 'test-project',
    roles: [],
    onUpdate: vi.fn().mockResolvedValue(undefined),
    onSuccess: vi.fn(),
  };

  it('initializes with empty form state', () => {
    const { result } = renderHook(() => useRoleAddition(defaultOptions));

    expect(result.current.showAddForm).toBe(false);
    expect(result.current.newRoleName).toBe('');
    expect(result.current.newRoleDescription).toBe('');
    expect(result.current.isAdding).toBe(false);
    expect(result.current.addRoleError).toBeNull();
  });

  it('rejects duplicate role names', async () => {
    const options = {
      ...defaultOptions,
      roles: [{ name: 'admin', policies: [], groups: [] }],
    };
    const { result } = renderHook(() => useRoleAddition(options));

    act(() => result.current.setNewRoleName('admin'));
    await act(async () => {
      await result.current.handleAddRole();
    });

    expect(result.current.addRoleError).toBe('Role "admin" already exists');
    expect(options.onUpdate).not.toHaveBeenCalled();
  });

  it('calls onUpdate with new role on successful add', async () => {
    const { result } = renderHook(() => useRoleAddition(defaultOptions));

    act(() => result.current.setNewRoleName('deployer'));
    await act(async () => {
      await result.current.handleAddRole();
    });

    expect(defaultOptions.onUpdate).toHaveBeenCalledWith({
      roles: expect.arrayContaining([
        expect.objectContaining({ name: 'deployer' }),
      ]),
    });
    expect(defaultOptions.onSuccess).toHaveBeenCalled();
  });

  it('sets error message when onUpdate rejects', async () => {
    const onSuccess = vi.fn();
    const options = {
      ...defaultOptions,
      onUpdate: vi.fn().mockRejectedValue(new Error('network error')),
      onSuccess,
    };
    const { result } = renderHook(() => useRoleAddition(options));

    act(() => result.current.setNewRoleName('deployer'));
    await act(async () => {
      await result.current.handleAddRole();
    });

    expect(result.current.addRoleError).toBe('Failed to add role. Please try again.');
    expect(result.current.isAdding).toBe(false);
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('resets form on resetForm', () => {
    const { result } = renderHook(() => useRoleAddition(defaultOptions));

    act(() => {
      result.current.setNewRoleName('test');
      result.current.setNewRoleDescription('desc');
    });
    act(() => result.current.resetForm());

    expect(result.current.newRoleName).toBe('');
    expect(result.current.newRoleDescription).toBe('');
  });
});
