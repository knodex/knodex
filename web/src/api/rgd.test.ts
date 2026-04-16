// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';

const { mockWarn, mockError } = vi.hoisted(() => ({
  mockWarn: vi.fn(),
  mockError: vi.fn(),
}));

// Mock apiClient
vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
  },
}));

// Mock logger
vi.mock('@/lib/logger', () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: mockWarn,
    error: mockError,
    log: vi.fn(),
  }),
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { listInstances, getInstance, instancePath } from './rgd';

describe('Instance namespace validation at API boundary', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('listInstances', () => {
    it('logs warning for namespaced instance with empty namespace', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          items: [
            { name: 'bad-instance', namespace: '', isClusterScoped: false },
          ],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        },
      });

      const result = await listInstances();

      // In dev mode (Vitest), error is used instead of warn
      expect(mockError).toHaveBeenCalledWith(
        expect.stringContaining('bad-instance')
      );
      expect(mockWarn).not.toHaveBeenCalled();
      // Data passes through unchanged
      expect(result.items).toHaveLength(1);
      expect(result.items[0].name).toBe('bad-instance');
    });

    it('logs warning for cluster-scoped instance with non-empty namespace', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          items: [
            { name: 'bad-cluster', namespace: 'oops', isClusterScoped: true },
          ],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        },
      });

      const result = await listInstances();

      // In dev mode (Vitest), error is used instead of warn
      expect(mockError).toHaveBeenCalledWith(
        expect.stringContaining('bad-cluster')
      );
      expect(mockWarn).not.toHaveBeenCalled();
      expect(result.items).toHaveLength(1);
    });

    it('does not log for valid namespaced instance', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          items: [
            { name: 'good-instance', namespace: 'default', isClusterScoped: false },
          ],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        },
      });

      await listInstances();

      expect(mockWarn).not.toHaveBeenCalled();
      expect(mockError).not.toHaveBeenCalled();
    });

    it('does not log for valid cluster-scoped instance', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: {
          items: [
            { name: 'good-cluster', namespace: '', isClusterScoped: true },
          ],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        },
      });

      await listInstances();

      expect(mockWarn).not.toHaveBeenCalled();
      expect(mockError).not.toHaveBeenCalled();
    });
  });

  describe('getInstance', () => {
    it('logs warning for malformed instance', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { name: 'bad-single', namespace: '', isClusterScoped: false },
      });

      const result = await getInstance('', 'MyKind', 'bad-single');

      // In dev mode (Vitest), error is used instead of warn
      expect(mockError).toHaveBeenCalledWith(
        expect.stringContaining('bad-single')
      );
      expect(mockWarn).not.toHaveBeenCalled();
      // Data passes through unchanged
      expect(result.name).toBe('bad-single');
    });

    it('does not log for valid instance', async () => {
      const { default: apiClient } = await import('./client');
      vi.mocked(apiClient.get).mockResolvedValue({
        data: { name: 'good-single', namespace: 'prod', isClusterScoped: false },
      });

      await getInstance('prod', 'MyKind', 'good-single');

      expect(mockWarn).not.toHaveBeenCalled();
      expect(mockError).not.toHaveBeenCalled();
    });
  });
});

describe('instancePath URL builder', () => {
  it('builds namespaced URL with /v1/namespaces/{ns}/instances/{kind}/{name}', () => {
    expect(instancePath('default', 'WebApp', 'my-app')).toBe(
      '/v1/namespaces/default/instances/WebApp/my-app'
    );
  });

  it('builds cluster-scoped URL with /v1/instances/{kind}/{name}', () => {
    expect(instancePath('', 'ClusterPolicy', 'global-policy')).toBe(
      '/v1/instances/ClusterPolicy/global-policy'
    );
  });

  it('encodes special characters in path segments', () => {
    expect(instancePath('my ns', 'My Kind', 'my name')).toBe(
      '/v1/namespaces/my%20ns/instances/My%20Kind/my%20name'
    );
  });
});
