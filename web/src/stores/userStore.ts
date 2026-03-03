import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { jwtDecode } from 'jwt-decode';
import { logger } from '@/lib/logger';

// JWT Claims from Backend
// ArgoCD-Aligned Authorization Model:
// - casbin_roles: Contains Casbin roles (e.g., ["role:serveradmin"])
// - roles: Project-scoped roles for permission inference
// - permissions: Pre-computed permissions for frontend UI rendering (ArgoCD pattern)
// NOTE: UI visibility checks are separate from backend authorization:
//   - Frontend: Uses permissions map for optimistic UI rendering
//   - Backend: Enforces actual permissions via Casbin CanAccess()
export interface JWTClaims {
  sub: string;
  email: string;
  name?: string;
  projects: string[];
  default_project: string;
  groups?: string[]; // OIDC groups from IdP
  casbin_roles?: string[]; // Casbin roles (e.g., ["role:serveradmin"])
  roles?: Record<string, string>; // projectId -> role name (e.g., { "proj-alpha-team": "developer" })
  permissions?: Record<string, boolean>; // Pre-computed permissions (e.g., {"*:*": true, "settings:get": true})
  exp: number;
  iat: number;
  iss?: string;
  aud?: string;
}

// Helper to check specific permission from JWT claims
// NOTE: For UI visibility only - actual authorization is enforced by backend via Casbin
export const hasPermission = (claims: JWTClaims | null, permission: string): boolean => {
  if (!claims) return false;

  // Admin has all permissions
  if (claims.permissions?.['*:*']) {
    return true;
  }

  return claims.permissions?.[permission] ?? false;
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

export interface UserState {
  // State
  user: User | null;
  projects: string[];
  roles: Record<string, string>; // projectId -> role name (e.g., { "proj-alpha-team": "developer" })
  currentProject: string | null;
  // NOTE: isGlobalAdmin was removed - use useCanI() hook for permission checks
  // Authorization should always go through Casbin, not boolean flags (ArgoCD-aligned)
  isAuthenticated: boolean;
  token: string | null;
  groups: string[]; // OIDC groups from JWT
  issuer: string | null; // JWT issuer (OIDC provider URL or null for local)
  casbinRoles: string[]; // Casbin roles (e.g., ["role:serveradmin"])
  tokenExp: number | null; // Token expiry as Unix timestamp
  tokenIat: number | null; // Token issued-at as Unix timestamp

  // Actions
  login: (token: string) => void;
  logout: () => void;
  setCurrentProject: (projectId: string) => void;
  refreshUser: () => void;
  isTokenExpired: () => boolean;
}

const decodeToken = (token: string): JWTClaims | null => {
  try {
    return jwtDecode<JWTClaims>(token);
  } catch (error) {
    logger.error('[UserStore] Failed to decode JWT:', error);
    return null;
  }
};

export const useUserStore = create<UserState>()(
  persist(
    (set, get) => ({
      user: null,
      projects: [],
      roles: {},
      currentProject: null,
      isAuthenticated: false,
      token: null,
      groups: [],
      issuer: null,
      casbinRoles: [],
      tokenExp: null,
      tokenIat: null,

      login: (token: string) => {
        const claims = decodeToken(token);
        if (!claims) {
          logger.error('[UserStore] Invalid token');
          return;
        }

        // NOTE: isGlobalAdmin was removed - use useCanI() hook for permission checks
        // The JWT permissions map is available via hasPermission() for client-side checks

        const user: User = {
          id: claims.sub,
          email: claims.email,
          name: claims.name,
        };

        set({
          user,
          projects: claims.projects || [],
          roles: claims.roles || {},
          // Fallback to first project if default_project not specified (handles E2E test tokens)
          currentProject: claims.default_project || claims.projects?.[0] || null,
          isAuthenticated: true,
          token,
          groups: claims.groups || [],
          issuer: claims.iss || null,
          casbinRoles: claims.casbin_roles || [],
          tokenExp: claims.exp || null,
          tokenIat: claims.iat || null,
        });

        // Store token in localStorage for axios interceptor
        localStorage.setItem('jwt_token', token);
      },

      logout: () => {
        set({
          user: null,
          projects: [],
          roles: {},
          currentProject: null,
          isAuthenticated: false,
          token: null,
          groups: [],
          issuer: null,
          casbinRoles: [],
          tokenExp: null,
          tokenIat: null,
        });

        // Clear token from localStorage
        localStorage.removeItem('jwt_token');
      },

      setCurrentProject: (projectId: string) => {
        const { projects } = get();
        if (!projects.includes(projectId)) {
          logger.error(`[UserStore] Project ${projectId} not found in user's projects`);
          return;
        }

        set({ currentProject: projectId });
      },

      refreshUser: () => {
        const { token } = get();
        if (!token) return;

        const claims = decodeToken(token);
        if (!claims) {
          get().logout();
          return;
        }

        // Check if token is expired
        if (claims.exp * 1000 < Date.now()) {
          get().logout();
          return;
        }

        // NOTE: isGlobalAdmin was removed - use useCanI() hook for permission checks

        // Update user info from token
        const user: User = {
          id: claims.sub,
          email: claims.email,
          name: claims.name,
        };

        set({
          user,
          projects: claims.projects || [],
          roles: claims.roles || {},
          // Fallback to first project if default_project not specified (handles E2E test tokens)
          currentProject: claims.default_project || claims.projects?.[0] || null,
          isAuthenticated: true,
          groups: claims.groups || [],
          issuer: claims.iss || null,
          casbinRoles: claims.casbin_roles || [],
          tokenExp: claims.exp || null,
          tokenIat: claims.iat || null,
        });
      },

      isTokenExpired: () => {
        const { token } = get();
        if (!token) return true;

        const claims = decodeToken(token);
        if (!claims) return true;

        return claims.exp * 1000 < Date.now();
      },
    }),
    {
      name: 'user-storage',
      partialize: (state) => ({
        currentProject: state.currentProject,
        token: state.token,
        isAuthenticated: state.isAuthenticated,
        roles: state.roles,
        projects: state.projects,
        // NOTE: isGlobalAdmin was removed - permission checks use useCanI() hook
      }),
      onRehydrateStorage: () => (state) => {
        // After rehydrating from localStorage, refresh user data from token
        if (state?.token) {
          state.refreshUser();
        }
      },
    }
  )
);
