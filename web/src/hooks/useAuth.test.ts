// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, act } from '@testing-library/react';
import {
  useUser,
  useCurrentProject,
  useIsAuthenticated,
  useAuth,
  useSessionStatus,
  useSessionError,
  hasPersistedSession,
  matchesNamespacePattern,
  projectAllowsNamespace,
} from './useAuth';
import { useUserStore } from '@/stores/userStore';
import type { LoginUserInfo } from '@/api/auth';
import type { Project } from '@/types/project';

describe('useAuth hooks', () => {
  beforeEach(() => {
    act(() => {
      useUserStore.getState().logout();
    });
    localStorage.clear();
    vi.clearAllMocks();
  });

  const makeUserInfo = (overrides: Partial<LoginUserInfo> = {}): LoginUserInfo => ({
    id: 'user-123',
    email: 'test@example.com',
    displayName: 'Test User',
    projects: ['proj-1'],
    defaultProject: 'proj-1',
    casbinRoles: [],
    ...overrides,
  });

  describe('useUser', () => {
    it('should return null when not authenticated', () => {
      const { result } = renderHook(() => useUser());
      expect(result.current).toBeNull();
    });

    it('should return user when authenticated', () => {
      act(() => {
        useUserStore.getState().login(makeUserInfo());
      });

      const { result } = renderHook(() => useUser());

      expect(result.current).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
    });
  });

  describe('useCurrentProject', () => {
    it('should return null when not authenticated', () => {
      const { result } = renderHook(() => useCurrentProject());
      expect(result.current).toBeNull();
    });

    it('should return null (All Projects) on fresh login', () => {
      act(() => {
        useUserStore.getState().login(makeUserInfo({
          projects: ['proj-1', 'proj-2'],
          defaultProject: 'proj-1',
        }));
      });

      const { result } = renderHook(() => useCurrentProject());
      expect(result.current).toBeNull();
    });

    it('should update when current project changes', () => {
      act(() => {
        useUserStore.getState().login(makeUserInfo({
          projects: ['proj-1', 'proj-2'],
        }));
      });

      const { result } = renderHook(() => useCurrentProject());
      expect(result.current).toBeNull();

      act(() => {
        useUserStore.getState().setCurrentProject('proj-2');
      });

      expect(result.current).toBe('proj-2');
    });
  });

  describe('useIsAuthenticated', () => {
    it('should return false when not authenticated', () => {
      const { result } = renderHook(() => useIsAuthenticated());
      expect(result.current).toBe(false);
    });

    it('should return true when authenticated', () => {
      act(() => {
        useUserStore.getState().login(makeUserInfo());
      });

      const { result } = renderHook(() => useIsAuthenticated());
      expect(result.current).toBe(true);
    });
  });

  describe('useAuth', () => {
    it('should provide authentication state and methods', () => {
      const { result } = renderHook(() => useAuth());

      expect(result.current.user).toBeNull();
      expect(result.current.isAuthenticated).toBe(false);
      expect(typeof result.current.login).toBe('function');
      expect(typeof result.current.logout).toBe('function');
    });

    it('should update state on login', () => {
      const { result } = renderHook(() => useAuth());

      act(() => {
        result.current.login(makeUserInfo());
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.user).not.toBeNull();
      expect(result.current.user?.email).toBe('test@example.com');
    });

    it('should clear state on logout', () => {
      const { result } = renderHook(() => useAuth());

      act(() => {
        result.current.login(makeUserInfo());
      });
      expect(result.current.isAuthenticated).toBe(true);

      act(() => {
        result.current.logout();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
    });
  });

  describe('useSessionStatus', () => {
    it('should return idle initially after logout', () => {
      // After logout, sessionStatus is 'logged_out'
      const { result } = renderHook(() => useSessionStatus());
      expect(result.current).toBe('logged_out');
    });

    it('should return valid after login', () => {
      act(() => {
        useUserStore.getState().login(makeUserInfo());
      });

      const { result } = renderHook(() => useSessionStatus());
      expect(result.current).toBe('valid');
    });
  });

  describe('useSessionError', () => {
    it('should return null initially', () => {
      const { result } = renderHook(() => useSessionError());
      expect(result.current).toBeNull();
    });

    it('should return error after setSessionStatus', () => {
      act(() => {
        useUserStore.getState().setSessionStatus('error', 'Network error');
      });

      const { result } = renderHook(() => useSessionError());
      expect(result.current).toBe('Network error');
    });
  });

  describe('hasPersistedSession', () => {
    it('should return true when localStorage has hasSession: true', () => {
      localStorage.setItem('user-storage', JSON.stringify({
        state: { hasSession: true, currentProject: 'proj-1' },
        version: 0,
      }));

      expect(hasPersistedSession()).toBe(true);
    });

    it('should return true when hasSession is true even if currentProject is null', () => {
      localStorage.setItem('user-storage', JSON.stringify({
        state: { hasSession: true, currentProject: null },
        version: 0,
      }));

      expect(hasPersistedSession()).toBe(true);
    });

    it('should return false when localStorage is empty', () => {
      expect(hasPersistedSession()).toBe(false);
    });

    it('should return false when localStorage has malformed JSON', () => {
      localStorage.setItem('user-storage', 'not valid json');
      expect(hasPersistedSession()).toBe(false);
    });

    it('should return false when hasSession is false', () => {
      localStorage.setItem('user-storage', JSON.stringify({
        state: { hasSession: false, currentProject: null },
        version: 0,
      }));

      expect(hasPersistedSession()).toBe(false);
    });

    it('should return false when hasSession field is missing (legacy format)', () => {
      localStorage.setItem('user-storage', JSON.stringify({
        state: { currentProject: 'proj-1' },
        version: 0,
      }));

      expect(hasPersistedSession()).toBe(false);
    });

    it('should return false when localStorage.getItem throws', () => {
      const originalGetItem = localStorage.getItem;
      localStorage.getItem = () => { throw new Error('SecurityError'); };

      expect(hasPersistedSession()).toBe(false);

      localStorage.getItem = originalGetItem;
    });
  });

  // Tests for helper functions
  describe('matchesNamespacePattern', () => {
    it('should return false for undefined pattern', () => {
      expect(matchesNamespacePattern(undefined, 'production')).toBe(false);
    });

    it('should return true for wildcard pattern "*"', () => {
      expect(matchesNamespacePattern('*', 'production')).toBe(true);
      expect(matchesNamespacePattern('*', 'dev-team1')).toBe(true);
      expect(matchesNamespacePattern('*', 'anything')).toBe(true);
    });

    it('should return true for exact match', () => {
      expect(matchesNamespacePattern('production', 'production')).toBe(true);
      expect(matchesNamespacePattern('dev-team1', 'dev-team1')).toBe(true);
    });

    it('should return false for non-matching exact pattern', () => {
      expect(matchesNamespacePattern('production', 'staging')).toBe(false);
      expect(matchesNamespacePattern('dev-team1', 'dev-team2')).toBe(false);
    });

    it('should return true for prefix wildcard pattern', () => {
      expect(matchesNamespacePattern('dev-*', 'dev-team1')).toBe(true);
      expect(matchesNamespacePattern('dev-*', 'dev-team2')).toBe(true);
      expect(matchesNamespacePattern('dev-*', 'dev-')).toBe(true);
      expect(matchesNamespacePattern('team-*', 'team-alpha')).toBe(true);
    });

    it('should return false for non-matching prefix pattern', () => {
      expect(matchesNamespacePattern('dev-*', 'staging-team1')).toBe(false);
      expect(matchesNamespacePattern('dev-*', 'production')).toBe(false);
      expect(matchesNamespacePattern('team-*', 'dev-team')).toBe(false);
    });
  });

  describe('projectAllowsNamespace', () => {
    it('should return false for undefined project', () => {
      expect(projectAllowsNamespace(undefined, 'production')).toBe(false);
    });

    it('should return false for project with no destinations', () => {
      const project: Project = {
        name: 'test-project',
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'production')).toBe(false);
    });

    it('should return false for project with empty destinations', () => {
      const project: Project = {
        name: 'test-project',
        destinations: [],
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'production')).toBe(false);
    });

    it('should return true when namespace matches a destination', () => {
      const project: Project = {
        name: 'test-project',
        destinations: [
          { namespace: 'production' },
          { namespace: 'staging' },
        ],
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'production')).toBe(true);
      expect(projectAllowsNamespace(project, 'staging')).toBe(true);
    });

    it('should return false when namespace does not match any destination', () => {
      const project: Project = {
        name: 'test-project',
        destinations: [
          { namespace: 'production' },
          { namespace: 'staging' },
        ],
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'dev-team1')).toBe(false);
    });

    it('should handle wildcard namespace in destinations', () => {
      const project: Project = {
        name: 'admin-project',
        destinations: [{ namespace: '*' }],
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'production')).toBe(true);
      expect(projectAllowsNamespace(project, 'any-namespace')).toBe(true);
    });

    it('should handle prefix wildcard namespace in destinations', () => {
      const project: Project = {
        name: 'dev-project',
        destinations: [{ namespace: 'dev-*' }],
        resourceVersion: '1',
        createdAt: new Date().toISOString(),
      };
      expect(projectAllowsNamespace(project, 'dev-team1')).toBe(true);
      expect(projectAllowsNamespace(project, 'dev-team2')).toBe(true);
      expect(projectAllowsNamespace(project, 'production')).toBe(false);
    });
  });
});
