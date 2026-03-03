import { describe, it, expect, vi, beforeEach } from 'vitest';
import { canI } from './auth';

// Mock apiClient
vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

// Mock logger
vi.mock('@/lib/logger', () => ({
  logger: {
    error: vi.fn(),
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
  },
}));

describe('canI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns true when server responds with "yes"', async () => {
    const { default: apiClient } = await import('./client');
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { value: 'yes' },
    });

    const result = await canI('projects', 'create');
    expect(result).toBe(true);
    expect(apiClient.get).toHaveBeenCalledWith('/v1/account/can-i/projects/create/-');
  });

  it('returns false when server responds with "no"', async () => {
    const { default: apiClient } = await import('./client');
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { value: 'no' },
    });

    const result = await canI('instances', 'delete', 'my-project');
    expect(result).toBe(false);
    expect(apiClient.get).toHaveBeenCalledWith('/v1/account/can-i/instances/delete/my-project');
  });

  it('returns false on 403 response (explicit deny)', async () => {
    const { default: apiClient } = await import('./client');
    const error = Object.assign(new Error('Forbidden'), {
      response: { status: 403 },
    });
    vi.mocked(apiClient.get).mockRejectedValue(error);

    const result = await canI('settings', 'update');
    expect(result).toBe(false);
  });

  it('throws on network error (for React Query retry)', async () => {
    const { default: apiClient } = await import('./client');
    const error = new Error('Network Error');
    vi.mocked(apiClient.get).mockRejectedValue(error);

    await expect(canI('projects', 'create')).rejects.toThrow('Network Error');
  });

  it('throws on 500 server error (for React Query retry)', async () => {
    const { default: apiClient } = await import('./client');
    const error = Object.assign(new Error('Internal Server Error'), {
      response: { status: 500 },
    });
    vi.mocked(apiClient.get).mockRejectedValue(error);

    await expect(canI('projects', 'create')).rejects.toThrow('Internal Server Error');
  });

  it('throws on 401 unauthorized (for React Query retry)', async () => {
    const { default: apiClient } = await import('./client');
    const error = Object.assign(new Error('Unauthorized'), {
      response: { status: 401 },
    });
    vi.mocked(apiClient.get).mockRejectedValue(error);

    await expect(canI('projects', 'create')).rejects.toThrow('Unauthorized');
  });

  it('encodes special characters in resource/action/subresource', async () => {
    const { default: apiClient } = await import('./client');
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { value: 'yes' },
    });

    await canI('instances', 'create', 'project/with-slash');
    expect(apiClient.get).toHaveBeenCalledWith(
      '/v1/account/can-i/instances/create/project%2Fwith-slash'
    );
  });
});
