// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { AccountInfoResponse } from '@/api/auth';
import { ApiError } from '@/api/client';

// Mock getAccountInfo
const mockGetAccountInfo = vi.fn();
vi.mock('@/api/auth', () => ({
  getAccountInfo: (...args: unknown[]) => mockGetAccountInfo(...args),
}));

// Mock React Query
const mockInvalidateQueries = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

// Mock store actions
const mockRestoreSession = vi.fn();
const mockSetSessionStatus = vi.fn();
const mockLogout = vi.fn();
let mockSessionStatus = 'idle';

// Track subscribers for simulating store transitions
type Subscriber = (state: { sessionStatus: string }, prevState: { sessionStatus: string }) => void;
const subscribers: Subscriber[] = [];

vi.mock('@/stores/userStore', () => ({
  useUserStore: Object.assign(
    () => ({}),
    {
      getState: () => ({
        sessionStatus: mockSessionStatus,
        restoreSession: mockRestoreSession,
        setSessionStatus: (status: string, error?: string) => {
          const prev = mockSessionStatus;
          mockSessionStatus = status;
          mockSetSessionStatus(status, error);
          // Notify subscribers of state change
          for (const fn of subscribers) {
            fn({ sessionStatus: status }, { sessionStatus: prev });
          }
        },
        logout: () => {
          const prev = mockSessionStatus;
          mockSessionStatus = 'logged_out';
          mockLogout();
          for (const fn of subscribers) {
            fn({ sessionStatus: 'logged_out' }, { sessionStatus: prev });
          }
        },
      }),
      subscribe: (fn: Subscriber) => {
        subscribers.push(fn);
        return () => {
          const idx = subscribers.indexOf(fn);
          if (idx >= 0) subscribers.splice(idx, 1);
        };
      },
    }
  ),
}));

// Import after mocks
import { useSessionRestore } from './useSessionRestore';

const makeAccountInfo = (): AccountInfoResponse => ({
  userID: 'user-123',
  email: 'test@example.com',
  displayName: 'Test User',
  projects: ['proj-1'],
  roles: {},
  groups: [],
  casbinRoles: [],
  issuer: 'https://auth.example.com',
  tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
  tokenIssuedAt: Math.floor(Date.now() / 1000) - 600,
});

describe('useSessionRestore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSessionStatus = 'idle';
    subscribers.length = 0;
  });

  it('calls getAccountInfo and restoreSession on success when idle', async () => {
    const accountInfo = makeAccountInfo();
    mockGetAccountInfo.mockResolvedValue(accountInfo);

    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockGetAccountInfo).toHaveBeenCalledOnce();
    });

    await vi.waitFor(() => {
      expect(mockSetSessionStatus).toHaveBeenCalledWith('validating', undefined);
      expect(mockRestoreSession).toHaveBeenCalledWith(accountInfo);
      expect(mockInvalidateQueries).toHaveBeenCalled();
    });
  });

  it('calls logout on 401 response', async () => {
    const error = new ApiError('UNAUTHORIZED', 'session expired', 401);
    mockGetAccountInfo.mockRejectedValue(error);

    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockLogout).toHaveBeenCalledOnce();
    });
  });

  it('calls logout on 403 response', async () => {
    const error = new ApiError('FORBIDDEN', 'access denied', 403);
    mockGetAccountInfo.mockRejectedValue(error);

    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockLogout).toHaveBeenCalledOnce();
    });
  });

  it('sets error status on network error', async () => {
    const error = { name: 'Error', message: 'Network Error' };
    mockGetAccountInfo.mockRejectedValue(error);

    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockSetSessionStatus).toHaveBeenCalledWith(
        'error',
        'Unable to connect to server. Please check your connection.'
      );
    });
    expect(mockLogout).not.toHaveBeenCalled();
  });

  it('does NOT trigger when sessionStatus is validating', () => {
    mockSessionStatus = 'validating';

    renderHook(() => useSessionRestore());

    expect(mockGetAccountInfo).not.toHaveBeenCalled();
  });

  it('does NOT trigger when sessionStatus is valid', () => {
    mockSessionStatus = 'valid';

    renderHook(() => useSessionRestore());

    expect(mockGetAccountInfo).not.toHaveBeenCalled();
  });

  it('calls queryClient.invalidateQueries after successful restore', async () => {
    mockGetAccountInfo.mockResolvedValue(makeAccountInfo());

    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockInvalidateQueries).toHaveBeenCalled();
    });
  });

  it('retries when sessionStatus transitions back to idle', async () => {
    // Start with error status (no initial restore)
    mockSessionStatus = 'error';
    mockGetAccountInfo.mockResolvedValue(makeAccountInfo());

    renderHook(() => useSessionRestore());

    // No API call yet (status is 'error')
    expect(mockGetAccountInfo).not.toHaveBeenCalled();

    // Simulate retry button: status transitions from 'error' to 'idle'
    // This calls setSessionStatus which notifies subscribers
    const { setSessionStatus } = (await import('@/stores/userStore')).useUserStore.getState();
    setSessionStatus('idle');

    await vi.waitFor(() => {
      expect(mockGetAccountInfo).toHaveBeenCalledOnce();
    });

    await vi.waitFor(() => {
      expect(mockRestoreSession).toHaveBeenCalled();
    });
  });

  it('StrictMode: cleanup resets validating to idle for remount', async () => {
    mockGetAccountInfo.mockResolvedValue(makeAccountInfo());

    // Mount and unmount (StrictMode first pass)
    const { unmount } = renderHook(() => useSessionRestore());

    // Wait for the API call to start
    expect(mockGetAccountInfo).toHaveBeenCalledOnce();

    // Unmount triggers cleanup — should reset 'validating' to 'idle'
    unmount();
    expect(mockSessionStatus).toBe('idle');

    // Clear mocks for fresh count
    mockGetAccountInfo.mockClear();
    mockRestoreSession.mockClear();

    // Remount (StrictMode second pass) — status is 'idle' so it retries
    renderHook(() => useSessionRestore());

    await vi.waitFor(() => {
      expect(mockGetAccountInfo).toHaveBeenCalledOnce();
    });
  });

  it('AbortController aborts request on unmount', async () => {
    // Never resolves — simulates slow network
    mockGetAccountInfo.mockImplementation(
      (opts: { signal?: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          opts?.signal?.addEventListener('abort', () => {
            reject({ name: 'CanceledError' });
          });
        })
    );

    const { unmount } = renderHook(() => useSessionRestore());

    expect(mockGetAccountInfo).toHaveBeenCalledOnce();

    // Unmount triggers abort
    unmount();

    // Should NOT call logout or restoreSession after abort
    await new Promise((r) => setTimeout(r, 50));
    expect(mockLogout).not.toHaveBeenCalled();
    expect(mockRestoreSession).not.toHaveBeenCalled();
  });

  it('unsubscribes from store on unmount', () => {
    mockSessionStatus = 'valid'; // Don't trigger restore

    const { unmount } = renderHook(() => useSessionRestore());

    // Subscriber should be registered
    expect(subscribers.length).toBe(1);

    unmount();

    // Subscriber should be removed
    expect(subscribers.length).toBe(0);
  });
});
