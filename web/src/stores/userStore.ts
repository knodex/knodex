// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { logger } from '@/lib/logger';

// Helper to check specific permission from stored user data
// NOTE: For UI visibility only - actual authorization is enforced by backend via Casbin
export const hasPermission = (permissions: Record<string, boolean> | undefined, permission: string): boolean => {
  if (!permissions) return false;

  // Admin has all permissions
  if (permissions['*:*']) {
    return true;
  }

  return permissions[permission] ?? false;
};

export interface User {
  id: string;
  email: string;
  name?: string;
  // NOTE: isGlobalAdmin was removed - use useCanI() hook for permission checks
  // This follows the ArgoCD-aligned pattern where permissions are checked via Casbin
}

export interface Project {
  id: string;
  displayName: string;
}

// LoginUserInfo matches the server's UserInfo struct from LoginResponse.
// After the HttpOnly cookie migration, the frontend receives user info directly
// from the server response (not by decoding the JWT).
export interface LoginUserInfo {
  id: string;
  email: string;
  displayName?: string;
  projects?: string[];
  defaultProject?: string;
  groups?: string[];
  roles?: Record<string, string>;
  casbinRoles?: string[];
  permissions?: Record<string, boolean>;
}

export interface UserState {
  // State
  user: User | null;
  projects: string[];
  roles: Record<string, string>; // projectId -> role name
  currentProject: string | null;
  isAuthenticated: boolean;
  groups: string[]; // OIDC groups
  issuer: string | null; // JWT issuer
  casbinRoles: string[]; // Casbin roles
  permissions: Record<string, boolean>; // Pre-computed permissions
  tokenExp: number | null; // Token expiry as Unix timestamp
  tokenIat: number | null; // Token issued-at as Unix timestamp

  // Actions
  login: (userInfo: LoginUserInfo, expiresAt?: string) => void;
  logout: () => void;
  setCurrentProject: (projectId: string) => void;
  isTokenExpired: () => boolean;
}

export const useUserStore = create<UserState>()(
  persist(
    (set, get) => ({
      user: null,
      projects: [],
      roles: {},
      currentProject: null,
      isAuthenticated: false,
      groups: [],
      issuer: null,
      casbinRoles: [],
      permissions: {},
      tokenExp: null,
      tokenIat: null,

      login: (userInfo: LoginUserInfo, expiresAt?: string) => {
        const user: User = {
          id: userInfo.id,
          email: userInfo.email,
          name: userInfo.displayName,
        };

        const expTimestamp = expiresAt ? Math.floor(new Date(expiresAt).getTime() / 1000) : null;

        set({
          user,
          projects: userInfo.projects || [],
          roles: userInfo.roles || {},
          currentProject: userInfo.defaultProject || userInfo.projects?.[0] || null,
          isAuthenticated: true,
          groups: userInfo.groups || [],
          issuer: null,
          casbinRoles: userInfo.casbinRoles || [],
          permissions: userInfo.permissions || {},
          tokenExp: expTimestamp,
          tokenIat: Math.floor(Date.now() / 1000),
        });
      },

      logout: () => {
        set({
          user: null,
          projects: [],
          roles: {},
          currentProject: null,
          isAuthenticated: false,
          groups: [],
          issuer: null,
          casbinRoles: [],
          permissions: {},
          tokenExp: null,
          tokenIat: null,
        });
      },

      setCurrentProject: (projectId: string) => {
        const { projects } = get();
        if (!projects.includes(projectId)) {
          logger.error(`[UserStore] Project ${projectId} not found in user's projects`);
          return;
        }

        set({ currentProject: projectId });
      },

      isTokenExpired: () => {
        const { tokenExp } = get();
        if (!tokenExp) return true;
        return tokenExp * 1000 < Date.now();
      },
    }),
    {
      // NOTE: This persists non-sensitive user metadata (display name, project list,
      // roles, permissions) to localStorage for session rehydration across page reloads.
      // The JWT itself is stored exclusively in an HttpOnly cookie and is NOT persisted here.
      // Accepted risk: user metadata in localStorage is not security-sensitive.
      name: 'user-storage',
      partialize: (state) => ({
        currentProject: state.currentProject,
        roles: state.roles,
        projects: state.projects,
        user: state.user,
        casbinRoles: state.casbinRoles,
        permissions: state.permissions,
        tokenExp: state.tokenExp,
        groups: state.groups,
      }),
    }
  )
);
