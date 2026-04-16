// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, act } from '@testing-library/react';
import { useUserStore } from './userStore';
import type { LoginUserInfo, AccountInfoResponse } from '@/api/auth';

describe('useUserStore', () => {
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
    projects: ['org-1', 'org-2'],
    defaultProject: 'org-1',
    casbinRoles: [],
    ...overrides,
  });

  const makeAccountInfo = (overrides: Partial<AccountInfoResponse> = {}): AccountInfoResponse => ({
    userID: 'user-123',
    email: 'test@example.com',
    displayName: 'Test User',
    projects: ['org-1', 'org-2'],
    roles: {},
    groups: [],
    casbinRoles: [],
    issuer: 'https://auth.example.com',
    tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    tokenIssuedAt: Math.floor(Date.now() / 1000) - 600,
    ...overrides,
  });

  describe('login', () => {
    it('should set user state from user info', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.sessionStatus).toBe('valid');
      expect(result.current.hasSession).toBe(true);
      expect(result.current.user).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
      expect(result.current.projects).toEqual(['org-1', 'org-2']);
      expect(result.current.currentProject).toBeNull(); // Defaults to All Projects
    });

    it('should handle admin user (permissions via Casbin)', () => {
      const userInfo = makeUserInfo({
        id: 'admin-123',
        email: 'admin@example.com',
        displayName: 'Admin User',
        projects: [],
        defaultProject: '',
        casbinRoles: ['role:serveradmin'],
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.user).toEqual({
        id: 'admin-123',
        email: 'admin@example.com',
        name: 'Admin User',
      });
      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.sessionStatus).toBe('valid');
    });

    it('should default to All Projects (null) on fresh login', () => {
      const userInfo = makeUserInfo({
        projects: ['org-alpha', 'org-beta'],
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBeNull();
      expect(result.current.projects).toEqual(['org-alpha', 'org-beta']);
    });

    it('should default to All Projects regardless of defaultProject hint', () => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2'],
        defaultProject: 'org-2',
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      // Fresh login always starts on All Projects
      expect(result.current.currentProject).toBeNull();
    });

    it('should set currentProject to null when no projects', () => {
      const userInfo = makeUserInfo({
        projects: [],
        casbinRoles: ['role:serveradmin'],
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBeNull();
    });

    it('should set sessionStatus to valid and hasSession to true on login', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.sessionStatus).toBe('valid');
      expect(result.current.hasSession).toBe(true);
    });
  });

  describe('restoreSession', () => {
    it('should map AccountInfoResponse fields correctly', () => {
      const info = makeAccountInfo({
        userID: 'restored-user',
        email: 'restored@example.com',
        displayName: 'Restored User',
        projects: ['proj-a'],
        roles: { 'proj-a': 'admin' },
        groups: ['eng-team'],
        casbinRoles: ['role:serveradmin'],
        issuer: 'https://issuer.example.com',
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.restoreSession(info);
      });

      expect(result.current.user).toEqual({
        id: 'restored-user',
        email: 'restored@example.com',
        name: 'Restored User',
      });
      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.sessionStatus).toBe('valid');
      expect(result.current.hasSession).toBe(true);
      expect(result.current.projects).toEqual(['proj-a']);
      expect(result.current.roles).toEqual({ 'proj-a': 'admin' });
      expect(result.current.groups).toEqual(['eng-team']);
      expect(result.current.casbinRoles).toEqual(['role:serveradmin']);
      expect(result.current.issuer).toBe('https://issuer.example.com');
    });

    it('should preserve currentProject if it exists in info.projects', () => {
      const { result } = renderHook(() => useUserStore());

      // Login first to set currentProject, then switch to org-2
      act(() => {
        result.current.login(makeUserInfo({
          projects: ['org-1', 'org-2'],
        }));
      });
      act(() => {
        result.current.setCurrentProject('org-2');
      });
      expect(result.current.currentProject).toBe('org-2');

      // Restore session with same projects — should preserve org-2
      act(() => {
        result.current.restoreSession(makeAccountInfo({
          projects: ['org-1', 'org-2'],
        }));
      });

      expect(result.current.currentProject).toBe('org-2');
    });

    it('should fall back to All Projects when current project not in list', () => {
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
      const { result } = renderHook(() => useUserStore());

      // Login and explicitly select a project
      act(() => {
        result.current.login(makeUserInfo({
          projects: ['org-old'],
        }));
        result.current.setCurrentProject('org-old');
      });

      // Restore with different projects (user removed from org-old)
      act(() => {
        result.current.restoreSession(makeAccountInfo({
          projects: ['org-new', 'org-other'],
        }));
      });

      expect(result.current.currentProject).toBeNull();
      consoleSpy.mockRestore();
    });

    it('should set currentProject to null when projects is empty', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.restoreSession(makeAccountInfo({
          projects: [],
        }));
      });

      expect(result.current.currentProject).toBeNull();
    });
  });

  describe('login and restoreSession parity', () => {
    it('should produce identical store state from equivalent data', () => {
      const { result: loginResult } = renderHook(() => useUserStore());

      act(() => {
        loginResult.current.login(makeUserInfo({
          id: 'parity-user',
          email: 'parity@test.com',
          displayName: 'Parity User',
          projects: ['proj-1'],
          groups: ['team-a'],
          roles: { 'proj-1': 'dev' },
          casbinRoles: ['role:serveradmin'],
        }));
      });

      const loginState = {
        user: loginResult.current.user,
        isAuthenticated: loginResult.current.isAuthenticated,
        sessionStatus: loginResult.current.sessionStatus,
        hasSession: loginResult.current.hasSession,
        projects: loginResult.current.projects,
        groups: loginResult.current.groups,
        roles: loginResult.current.roles,
        casbinRoles: loginResult.current.casbinRoles,
        currentProject: loginResult.current.currentProject,
      };

      // Reset store
      act(() => {
        loginResult.current.logout();
      });
      localStorage.clear();

      const { result: restoreResult } = renderHook(() => useUserStore());

      act(() => {
        restoreResult.current.restoreSession(makeAccountInfo({
          userID: 'parity-user',
          email: 'parity@test.com',
          displayName: 'Parity User',
          projects: ['proj-1'],
          groups: ['team-a'],
          roles: { 'proj-1': 'dev' },
          casbinRoles: ['role:serveradmin'],
          issuer: 'https://auth.example.com',
        }));
      });

      const restoreState = {
        user: restoreResult.current.user,
        isAuthenticated: restoreResult.current.isAuthenticated,
        sessionStatus: restoreResult.current.sessionStatus,
        hasSession: restoreResult.current.hasSession,
        projects: restoreResult.current.projects,
        groups: restoreResult.current.groups,
        roles: restoreResult.current.roles,
        casbinRoles: restoreResult.current.casbinRoles,
        currentProject: restoreResult.current.currentProject,
      };

      expect(loginState.user).toEqual(restoreState.user);
      expect(loginState.isAuthenticated).toBe(restoreState.isAuthenticated);
      expect(loginState.sessionStatus).toBe(restoreState.sessionStatus);
      expect(loginState.hasSession).toBe(restoreState.hasSession);
      expect(loginState.projects).toEqual(restoreState.projects);
      expect(loginState.groups).toEqual(restoreState.groups);
      expect(loginState.roles).toEqual(restoreState.roles);
      expect(loginState.casbinRoles).toEqual(restoreState.casbinRoles);
      expect(loginState.currentProject).toBe(restoreState.currentProject);
    });
  });

  describe('setSessionStatus', () => {
    it('should set status and error correctly', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setSessionStatus('error', 'Network error');
      });

      expect(result.current.sessionStatus).toBe('error');
      expect(result.current.sessionError).toBe('Network error');
    });

    it('should clear error when not provided', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setSessionStatus('error', 'Some error');
      });

      act(() => {
        result.current.setSessionStatus('validating');
      });

      expect(result.current.sessionStatus).toBe('validating');
      expect(result.current.sessionError).toBeNull();
    });
  });

  describe('logout', () => {
    it('should clear all user state and set logged_out status', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });
      expect(result.current.isAuthenticated).toBe(true);

      act(() => {
        result.current.logout();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
      expect(result.current.projects).toEqual([]);
      expect(result.current.currentProject).toBeNull();
      expect(result.current.sessionStatus).toBe('logged_out');
      expect(result.current.hasSession).toBe(false);
    });
  });

  describe('setCurrentProject', () => {
    beforeEach(() => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2', 'org-3'],
        // Fresh login defaults to All Projects (null)
      });

      const { result } = renderHook(() => useUserStore());
      act(() => {
        result.current.login(userInfo);
      });
    });

    it('should set current project if user is member', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setCurrentProject('org-2');
      });

      expect(result.current.currentProject).toBe('org-2');
    });

    it('should set any project (backend enforces access)', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setCurrentProject('org-999');
      });

      expect(result.current.currentProject).toBe('org-999');
    });

    it('should set currentProject to null for "All Projects"', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setCurrentProject(null);
      });

      expect(result.current.currentProject).toBeNull();
    });

    it('should persist null currentProject to localStorage', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.setCurrentProject(null);
      });

      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();
      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.currentProject).toBeNull();
      }
    });
  });

  describe('persistence', () => {
    it('should persist only hasSession and currentProject to localStorage', () => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2'],
        // Fresh login defaults to All Projects (null)
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        // Only hasSession and currentProject should be persisted
        expect(parsed.state.hasSession).toBe(true);
        expect(parsed.state.currentProject).toBeNull();
        // These should NOT be persisted
        expect(parsed.state.user).toBeUndefined();
        expect(parsed.state.roles).toBeUndefined();
        expect(parsed.state.projects).toBeUndefined();
        expect(parsed.state.groups).toBeUndefined();
        expect(parsed.state.casbinRoles).toBeUndefined();
      }
    });

    it('should persist currentProject changes', () => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2'],
        // Fresh login defaults to All Projects (null)
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      act(() => {
        result.current.setCurrentProject('org-2');
      });

      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.currentProject).toBe('org-2');
      }
    });

    it('should clear hasSession on logout', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      act(() => {
        result.current.logout();
      });

      const storage = localStorage.getItem('user-storage');
      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.hasSession).toBe(false);
      }
    });
  });
});
