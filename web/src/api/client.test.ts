// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock logger before importing client
vi.mock('@/lib/logger', () => ({
  logger: {
    error: vi.fn(),
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
  },
}));

// Mock userStore
const mockLogout = vi.fn();
vi.mock('@/stores/userStore', () => ({
  useUserStore: {
    getState: () => ({
      logout: mockLogout,
    }),
  },
}));

// Import after mocks are set up
import apiClient, { _resetRedirectState, _getLastRedirectTimestamp } from './client';

describe('API Client 401 Interceptor', () => {
  let originalPathname: PropertyDescriptor | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    _resetRedirectState();

    // Save original pathname descriptor
    originalPathname = Object.getOwnPropertyDescriptor(window, 'location');
  });

  afterEach(() => {
    vi.useRealTimers();
    // Restore original location
    if (originalPathname) {
      Object.defineProperty(window, 'location', originalPathname);
    }
  });

  function mockPathname(pathname: string) {
    Object.defineProperty(window, 'location', {
      value: { pathname, href: `http://localhost${pathname}` },
      writable: true,
      configurable: true,
    });
  }

  // Helper to get the response error interceptor handler
  function getErrorHandler() {
    const interceptors = (apiClient.interceptors.response as unknown as { handlers: Array<{ rejected: (err: unknown) => Promise<unknown> }> }).handlers;
    return interceptors[0].rejected;
  }

  it('skips redirect for /auth/callback path on 401', async () => {
    mockPathname('/auth/callback');

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'missing token' },
      },
      message: 'Unauthorized',
    };

    await expect(getErrorHandler()(error)).rejects.toBeDefined();

    // Should NOT call logout when on auth callback path
    expect(mockLogout).not.toHaveBeenCalled();
  });

  it('skips redirect for /login path on 401', async () => {
    mockPathname('/login');

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'missing token' },
      },
      message: 'Unauthorized',
    };

    await expect(getErrorHandler()(error)).rejects.toBeDefined();

    expect(mockLogout).not.toHaveBeenCalled();
  });

  it('triggers logout on 401 for non-auth paths', async () => {
    mockPathname('/dashboard');

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'token expired' },
      },
      message: 'Unauthorized',
    };

    await expect(getErrorHandler()(error)).rejects.toBeDefined();

    // Should call logout when on non-auth path
    expect(mockLogout).toHaveBeenCalledOnce();
    // Should record redirect timestamp
    expect(_getLastRedirectTimestamp()).toBeGreaterThan(0);
  });

  it('debounces multiple 401 responses - only first triggers redirect', async () => {
    mockPathname('/dashboard');

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'token expired' },
      },
      message: 'Unauthorized',
    };

    const errorHandler = getErrorHandler();

    // Fire multiple 401s simultaneously (within cooldown window)
    await expect(errorHandler(error)).rejects.toBeDefined();
    await expect(errorHandler(error)).rejects.toBeDefined();
    await expect(errorHandler(error)).rejects.toBeDefined();

    // Only the first should trigger logout
    expect(mockLogout).toHaveBeenCalledOnce();
  });

  it('allows redirect again after cooldown period', async () => {
    mockPathname('/dashboard');

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'token expired' },
      },
      message: 'Unauthorized',
    };

    const errorHandler = getErrorHandler();

    // First 401 triggers logout
    await expect(errorHandler(error)).rejects.toBeDefined();
    expect(mockLogout).toHaveBeenCalledOnce();

    // Within cooldown — should NOT trigger logout again
    await expect(errorHandler(error)).rejects.toBeDefined();
    expect(mockLogout).toHaveBeenCalledOnce();

    // Advance past cooldown period
    vi.advanceTimersByTime(2000);

    // After cooldown — should trigger logout again
    await expect(errorHandler(error)).rejects.toBeDefined();
    expect(mockLogout).toHaveBeenCalledTimes(2);
  });

  it('does not trigger redirect for non-401 errors', async () => {
    mockPathname('/dashboard');

    const error = {
      response: {
        status: 500,
        data: { code: 'INTERNAL_ERROR', message: 'server error' },
      },
      message: 'Internal Server Error',
    };

    await expect(getErrorHandler()(error)).rejects.toBeDefined();

    expect(mockLogout).not.toHaveBeenCalled();
  });

  it('sends withCredentials for automatic cookie inclusion', () => {
    expect(apiClient.defaults.withCredentials).toBe(true);
  });
});
