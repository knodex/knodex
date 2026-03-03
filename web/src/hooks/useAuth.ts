import { useUserStore } from '@/stores/userStore';
import type { Project } from '@/types/project';

export function useUser() {
  return useUserStore((state) => state.user);
}

export function useCurrentProject() {
  return useUserStore((state) => state.currentProject);
}

export function useIsAuthenticated() {
  return useUserStore((state) => state.isAuthenticated);
}

export function useAuth() {
  const user = useUser();
  const isAuthenticated = useIsAuthenticated();
  const login = useUserStore((state) => state.login);
  const logout = useUserStore((state) => state.logout);
  const refreshUser = useUserStore((state) => state.refreshUser);
  const isTokenExpired = useUserStore((state) => state.isTokenExpired);

  return {
    user,
    isAuthenticated,
    login,
    logout,
    refreshUser,
    isTokenExpired,
  };
}

/**
 * Helper to check if a namespace matches a destination pattern.
 * Supports wildcards: "*" (any namespace), "dev-*" (prefix match), or exact match.
 *
 * @param pattern - The destination namespace pattern (e.g., "*", "dev-*", "production")
 * @param namespace - The actual namespace to check
 * @returns true if the namespace matches the pattern
 */
export function matchesNamespacePattern(
  pattern: string | undefined,
  namespace: string
): boolean {
  if (!pattern) return false;
  if (pattern === '*') return true;
  if (pattern === namespace) return true;

  // Wildcard suffix matching (e.g., "dev-*" matches "dev-team1")
  if (pattern.endsWith('*')) {
    const prefix = pattern.slice(0, -1);
    return namespace.startsWith(prefix);
  }

  return false;
}

/**
 * Helper to check if a project allows deployment to a specific namespace.
 *
 * @param project - The project to check
 * @param namespace - The namespace to check
 * @returns true if the project has a destination that matches the namespace
 */
export function projectAllowsNamespace(
  project: Project | undefined,
  namespace: string
): boolean {
  if (!project?.destinations) return false;

  return project.destinations.some((dest) =>
    matchesNamespacePattern(dest.namespace, namespace)
  );
}
