// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useInstanceDialogs } from './useInstanceDialogs';

describe('useInstanceDialogs', () => {
  it('initializes all dialogs as closed', () => {
    const { result } = renderHook(() => useInstanceDialogs());

    expect(result.current.showDeleteDialog).toBe(false);
    expect(result.current.showEditDialog).toBe(false);
    expect(result.current.showRevisionDrawer).toBe(false);
  });

  it('toggles delete dialog', () => {
    const { result } = renderHook(() => useInstanceDialogs());

    act(() => result.current.setShowDeleteDialog(true));
    expect(result.current.showDeleteDialog).toBe(true);

    act(() => result.current.setShowDeleteDialog(false));
    expect(result.current.showDeleteDialog).toBe(false);
  });

  it('toggles edit dialog', () => {
    const { result } = renderHook(() => useInstanceDialogs());

    act(() => result.current.setShowEditDialog(true));
    expect(result.current.showEditDialog).toBe(true);
  });

  it('toggles revision drawer', () => {
    const { result } = renderHook(() => useInstanceDialogs());

    act(() => result.current.setShowRevisionDrawer(true));
    expect(result.current.showRevisionDrawer).toBe(true);
  });
});
