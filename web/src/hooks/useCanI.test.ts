import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useCanI, useCanIAll, useCanIAny } from './useCanI';

// Mock dependencies
vi.mock('./useAuth', () => ({
  useIsAuthenticated: vi.fn(() => true),
}));

vi.mock('@/api/auth', () => ({
  canI: vi.fn(),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useCanI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns { allowed: undefined, isLoading: true } initially', async () => {
    const { canI } = await import('@/api/auth');
    // Never resolves during this test
    vi.mocked(canI).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    // Initially, data is undefined and isLoading is true
    expect(result.current.allowed).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
  });

  it('returns { allowed: true, isLoading: false } when permission granted', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(true);

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(true);
  });

  it('returns { allowed: false, isLoading: false } when permission denied', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(false);

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(false);
  });

  it('does not use placeholderData (allowed is undefined during loading, not false)', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(
      () => useCanI('instances', 'create'),
      { wrapper: createWrapper() }
    );

    // The critical fix: allowed should be undefined during loading, NOT false
    // This prevents buttons from being hidden during the loading period
    expect(result.current.allowed).not.toBe(false);
    expect(result.current.allowed).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
  });

  it('does not fetch when not authenticated', async () => {
    const { useIsAuthenticated } = await import('./useAuth');
    vi.mocked(useIsAuthenticated).mockReturnValue(false);

    const { canI } = await import('@/api/auth');

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    expect(canI).not.toHaveBeenCalled();
    expect(result.current.isLoading).toBe(false);

    // Restore
    vi.mocked(useIsAuthenticated).mockReturnValue(true);
  });

  it('passes correct parameters to canI API', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(true);

    renderHook(
      () => useCanI('instances', 'delete', 'my-project'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(canI).toHaveBeenCalledWith('instances', 'delete', 'my-project');
    });
  });

  it('defaults subresource to "-"', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(true);

    renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(canI).toHaveBeenCalledWith('projects', 'create', '-');
    });
  });
});

describe('useCanI error handling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('stays in loading state when canI throws (allows React Query to retry)', async () => {
    const { canI } = await import('@/api/auth');
    // Simulate a persistent network error - canI throws
    vi.mocked(canI).mockRejectedValue(new Error('Network Error'));

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    // While retrying, allowed should remain undefined (not false)
    // This ensures buttons don't disappear during transient failures
    expect(result.current.allowed).toBeUndefined();
  });

  it('resolves to false when canI returns false (explicit deny)', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(false);

    const { result } = renderHook(
      () => useCanI('projects', 'delete'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(false);
  });

  it('exposes isError=true when all retries fail (for optimistic UI)', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockRejectedValue(new Error('Network Error'));

    const { result } = renderHook(
      () => useCanI('projects', 'create'),
      { wrapper: createWrapper() }
    );

    // Hook has retry:2, retryDelay:1000 — wait for all retries to exhaust (~3s)
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    }, { timeout: 5000 });

    // After error, allowed is undefined (not false) and isLoading is false
    expect(result.current.allowed).toBeUndefined();
    expect(result.current.isLoading).toBe(false);
  });
});

describe('useCanIAll', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns undefined during loading', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(
      () => useCanIAll([['projects', 'update', 'test'], ['projects', 'delete', 'test']]),
      { wrapper: createWrapper() }
    );

    expect(result.current.allowed).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
  });

  it('returns true when all permissions are granted', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(true);

    const { result } = renderHook(
      () => useCanIAll([['projects', 'update', 'test'], ['projects', 'delete', 'test']]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(true);
  });

  it('returns false when any permission is denied', async () => {
    const { canI } = await import('@/api/auth');
    // First call grants, second denies
    vi.mocked(canI)
      .mockResolvedValueOnce(true)
      .mockResolvedValueOnce(false);

    const { result } = renderHook(
      () => useCanIAll([['projects', 'update', 'test'], ['projects', 'delete', 'test']]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(false);
  });
});

describe('useCanIAny', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns undefined during loading', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockImplementation(() => new Promise(() => {}));

    const { result } = renderHook(
      () => useCanIAny([['instances', 'create', 'project-a'], ['instances', 'create', 'project-b']]),
      { wrapper: createWrapper() }
    );

    expect(result.current.allowed).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
  });

  it('returns true when any permission is granted', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI)
      .mockResolvedValueOnce(false)
      .mockResolvedValueOnce(true);

    const { result } = renderHook(
      () => useCanIAny([['instances', 'create', 'project-a'], ['instances', 'create', 'project-b']]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(true);
  });

  it('returns false when all permissions are denied', async () => {
    const { canI } = await import('@/api/auth');
    vi.mocked(canI).mockResolvedValue(false);

    const { result } = renderHook(
      () => useCanIAny([['instances', 'create', 'project-a'], ['instances', 'create', 'project-b']]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.allowed).toBe(false);
  });
});
