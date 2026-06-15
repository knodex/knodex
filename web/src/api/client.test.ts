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

// Mock userStore — currentProject feeds the X-Knodex-Project audit-lens header.
const mockLogout = vi.fn();
let mockSessionStatus = 'valid';
let mockCurrentProject: string | null = null;

vi.mock('@/stores/userStore', () => ({
  useUserStore: {
    getState: () => ({
      logout: mockLogout,
      sessionStatus: mockSessionStatus,
      currentProject: mockCurrentProject,
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
    mockSessionStatus = 'valid';

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

  it('triggers logout on 401 for non-auth paths when session is valid', async () => {
    mockPathname('/dashboard');
    mockSessionStatus = 'valid';

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

  it('does NOT trigger logout on 401 during session validation', async () => {
    mockPathname('/dashboard');
    mockSessionStatus = 'validating';

    const error = {
      response: {
        status: 401,
        data: { code: 'UNAUTHORIZED', message: 'token expired' },
      },
      message: 'Unauthorized',
    };

    await expect(getErrorHandler()(error)).rejects.toBeDefined();

    // Should NOT call logout during session restore
    expect(mockLogout).not.toHaveBeenCalled();
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

describe('API Client X-Knodex-Project audit-lens header', () => {
  // EXPECTED_REQUEST_INTERCEPTORS pins how many request interceptors
  // client.ts registers. The X-Knodex-Project audit-lens stamping lives in
  // the single, unified request interceptor at the top of the file (it also
  // sets the JWT Bearer header). If a future change adds OR splits
  // interceptors, this count must move in lockstep with the runner below —
  // otherwise we would silently invoke the wrong interceptor and fail to
  // notice the regression.
  const EXPECTED_REQUEST_INTERCEPTORS = 1;

  // Helper to invoke the request interceptor handler directly. Asserts the
  // expected interceptor count before running so a structural change loudly
  // fails this test rather than silently testing the wrong handler.
  function runRequestInterceptor(config: { headers: Record<string, string> }) {
    const interceptors = (apiClient.interceptors.request as unknown as {
      handlers: Array<{ fulfilled: (cfg: typeof config) => typeof config }>;
    }).handlers;
    expect(
      interceptors.length,
      `expected ${EXPECTED_REQUEST_INTERCEPTORS} request interceptor(s); ` +
        `got ${interceptors.length}. Update EXPECTED_REQUEST_INTERCEPTORS + ` +
        `the runner if client.ts now registers more than one interceptor.`,
    ).toBe(EXPECTED_REQUEST_INTERCEPTORS);
    // Always target the LAST interceptor (the one that stamps the lens).
    // Pinned by the count assertion above.
    return interceptors[interceptors.length - 1].fulfilled(config);
  }

  beforeEach(() => {
    mockCurrentProject = null;
  });

  it('stamps X-Knodex-Project when currentProject is set', () => {
    mockCurrentProject = 'alpha';
    const cfg = runRequestInterceptor({ headers: {} });
    expect(cfg.headers['X-Knodex-Project']).toBe('alpha');
  });

  it('omits X-Knodex-Project on "All Projects" (currentProject=null)', () => {
    mockCurrentProject = null;
    const cfg = runRequestInterceptor({ headers: {} });
    expect(cfg.headers['X-Knodex-Project']).toBeUndefined();
  });

  it('omits X-Knodex-Project when currentProject is an empty string', () => {
    mockCurrentProject = '';
    const cfg = runRequestInterceptor({ headers: {} });
    expect(cfg.headers['X-Knodex-Project']).toBeUndefined();
  });
});
