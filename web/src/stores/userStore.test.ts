// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, act } from '@testing-library/react';
import { useUserStore } from './userStore';
import type { LoginUserInfo } from './userStore';

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

  describe('login', () => {
    it('should set user state from user info', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, '2099-01-01T00:00:00Z');
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.user).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
      expect(result.current.projects).toEqual(['org-1', 'org-2']);
      expect(result.current.currentProject).toBe('org-1');
    });

    it('should handle admin user (permissions via Casbin)', () => {
      const userInfo = makeUserInfo({
        id: 'admin-123',
        email: 'admin@example.com',
        displayName: 'Admin User',
        projects: [],
        defaultProject: '',
        casbinRoles: ['role:serveradmin'],
        permissions: { '*:*': true },
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, '2099-01-01T00:00:00Z');
      });

      expect(result.current.user).toEqual({
        id: 'admin-123',
        email: 'admin@example.com',
        name: 'Admin User',
      });
      expect(result.current.isAuthenticated).toBe(true);
    });

    it('should fallback to first project when defaultProject is missing', () => {
      const userInfo = makeUserInfo({
        id: 'platform-admin-alpha',
        email: 'platformadmin@test.com',
        displayName: 'Platform Admin User',
        projects: ['org-alpha', 'org-beta'],
        defaultProject: undefined,
        casbinRoles: [],
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBe('org-alpha');
      expect(result.current.projects).toEqual(['org-alpha', 'org-beta']);
    });

    it('should set currentProject to null when no projects and no defaultProject', () => {
      const userInfo = makeUserInfo({
        id: 'global-admin',
        email: 'admin@test.com',
        displayName: 'Global Admin',
        projects: [],
        defaultProject: undefined,
        casbinRoles: ['role:serveradmin'],
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBeNull();
    });
  });

  describe('logout', () => {
    it('should clear all user state', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, '2099-01-01T00:00:00Z');
      });
      expect(result.current.isAuthenticated).toBe(true);

      act(() => {
        result.current.logout();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
      expect(result.current.projects).toEqual([]);
      expect(result.current.currentProject).toBeNull();
    });
  });

  describe('setCurrentProject', () => {
    beforeEach(() => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2', 'org-3'],
        defaultProject: 'org-1',
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

    it('should not set current project if user is not member', () => {
      const { result } = renderHook(() => useUserStore());
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      act(() => {
        result.current.setCurrentProject('org-999');
      });

      expect(result.current.currentProject).toBe('org-1');
      consoleSpy.mockRestore();
    });
  });

  describe('isTokenExpired', () => {
    it('should return true if no expiry exists', () => {
      const { result } = renderHook(() => useUserStore());
      expect(result.current.isTokenExpired()).toBe(true);
    });

    it('should return true if token is expired', () => {
      const pastDate = new Date(Date.now() - 100_000).toISOString();
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, pastDate);
      });

      expect(result.current.isTokenExpired()).toBe(true);
    });

    it('should return false if token is valid', () => {
      const futureDate = new Date(Date.now() + 3_600_000).toISOString();
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, futureDate);
      });

      expect(result.current.isTokenExpired()).toBe(false);
    });
  });

  describe('persistence', () => {
    it('should persist currentProject to localStorage', () => {
      const userInfo = makeUserInfo({
        projects: ['org-1', 'org-2'],
        defaultProject: 'org-1',
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

    it('should persist user data to localStorage (isAuthenticated excluded from partialize)', () => {
      const userInfo = makeUserInfo();
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(userInfo, '2099-01-01T00:00:00Z');
      });

      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        // isAuthenticated is intentionally excluded from partialize;
        // it is rehydrated from tokenExp on page load instead.
        expect(parsed.state.user).toBeTruthy();
        expect(parsed.state.roles).toBeTruthy();
      }
    });

    it('should rehydrate authentication state on page load', () => {
      const userInfo = makeUserInfo();

      const { result: firstRender } = renderHook(() => useUserStore());
      act(() => {
        firstRender.current.login(userInfo, '2099-01-01T00:00:00Z');
      });

      expect(firstRender.current.isAuthenticated).toBe(true);

      // Capture localStorage before simulating refresh
      const savedStorage = localStorage.getItem('user-storage');
      expect(savedStorage).toBeTruthy();

      // Simulate page refresh: reset in-memory store to defaults, keep localStorage intact
      act(() => {
        useUserStore.setState({
          isAuthenticated: false,
          user: null,
          projects: [],
          tokenExp: null,
        });
      });
      // Restore localStorage (setState may have triggered persist)
      localStorage.setItem('user-storage', savedStorage!);

      // Trigger rehydration from localStorage
      act(() => {
        useUserStore.persist.rehydrate();
      });

      const { result: secondRender } = renderHook(() => useUserStore());

      expect(secondRender.current.isAuthenticated).toBe(true);
      expect(secondRender.current.user).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
      expect(secondRender.current.projects).toEqual(['org-1', 'org-2']);
    });

    it('should not rehydrate isAuthenticated when token is expired', () => {
      const userInfo = makeUserInfo();
      const pastDate = new Date(Date.now() - 100_000).toISOString();

      const { result } = renderHook(() => useUserStore());
      act(() => {
        result.current.login(userInfo, pastDate);
      });

      // Capture localStorage, reset store, restore localStorage, then rehydrate
      const savedStorage = localStorage.getItem('user-storage');
      act(() => {
        useUserStore.setState({ isAuthenticated: false });
      });
      localStorage.setItem('user-storage', savedStorage!);
      act(() => {
        useUserStore.persist.rehydrate();
      });

      expect(result.current.isAuthenticated).toBe(false);
    });
  });
});
