// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { logger } from '@/lib/logger';
import type { LoginUserInfo, AccountInfoResponse } from '@/api/auth';

export interface User {
  id: string;
  email: string;
  name?: string;
}

export interface Project {
  id: string;
  displayName: string;
}

export type SessionStatus = 'idle' | 'validating' | 'valid' | 'error' | 'logged_out';

interface UserStatePayload {
  id: string;
  email: string;
  name?: string;
  projects: string[];
  roles: Record<string, string>;
  groups: string[];
  casbinRoles: string[];
  issuer: string | null;
}

export interface UserState {
  // State
  user: User | null;
  projects: string[];
  roles: Record<string, string>;
  currentProject: string | null;
  isAuthenticated: boolean;
  groups: string[];
  issuer: string | null;
  casbinRoles: string[];
  sessionStatus: SessionStatus;
  sessionError: string | null;
  hasSession: boolean;

  // Actions
  login: (userInfo: LoginUserInfo) => void;
  restoreSession: (info: AccountInfoResponse) => void;
  setSessionStatus: (status: SessionStatus, error?: string) => void;
  logout: () => void;
  setCurrentProject: (projectId: string) => void;
}

function _setUserState(
  payload: UserStatePayload,
  set: (state: Partial<UserState>) => void,
  get: () => UserState
) {
  const currentProject = get().currentProject;
  const projectStillValid = currentProject && payload.projects.includes(currentProject);
  const resolvedProject = projectStillValid
    ? currentProject
    : payload.projects[0] || null;

  if (currentProject && !projectStillValid) {
    logger.warn(
      `[UserStore] Project context changed: '${currentProject}' is no longer accessible. Switching to '${resolvedProject}'.`
    );
  }

  set({
    user: { id: payload.id, email: payload.email, name: payload.name },
    projects: payload.projects,
    roles: payload.roles,
    groups: payload.groups,
    casbinRoles: payload.casbinRoles,
    issuer: payload.issuer,
    currentProject: resolvedProject,
    isAuthenticated: true,
    sessionStatus: 'valid',
    sessionError: null,
    hasSession: true,
  });
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
      sessionStatus: 'idle' as SessionStatus,
      sessionError: null,
      hasSession: false,

      login: (userInfo: LoginUserInfo) => {
        _setUserState(
          {
            id: userInfo.id,
            email: userInfo.email,
            name: userInfo.displayName,
            projects: userInfo.projects || [],
            roles: userInfo.roles || {},
            groups: userInfo.groups || [],
            casbinRoles: userInfo.casbinRoles || [],
            issuer: null,
          },
          set,
          get
        );
      },

      restoreSession: (info: AccountInfoResponse) => {
        _setUserState(
          {
            id: info.userID,
            email: info.email,
            name: info.displayName,
            projects: info.projects,
            roles: info.roles,
            groups: info.groups,
            casbinRoles: info.casbinRoles,
            issuer: info.issuer,
          },
          set,
          get
        );
      },

      setSessionStatus: (status: SessionStatus, error?: string) => {
        set({ sessionStatus: status, sessionError: error ?? null });
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
          sessionStatus: 'logged_out',
          sessionError: null,
          hasSession: false,
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
    }),
    {
      name: 'user-storage',
      partialize: (state) => ({
        hasSession: state.hasSession,
        currentProject: state.currentProject,
      }),
    }
  )
);
