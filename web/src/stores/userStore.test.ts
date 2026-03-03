import { renderHook, act } from '@testing-library/react';
import { useUserStore } from './userStore';
import { jwtDecode } from 'jwt-decode';

// Mock jwt-decode
vi.mock('jwt-decode');

describe('useUserStore', () => {
  beforeEach(() => {
    // Clear store before each test
    act(() => {
      useUserStore.getState().logout();
    });
    // Clear localStorage
    localStorage.clear();
    // Reset all mocks
    vi.clearAllMocks();
  });

  describe('login', () => {
    it('should decode JWT and set user state', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
        projects: ['org-1', 'org-2'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(true);
      // NOTE: isGlobalAdmin was removed from User - use useCanI() hook for permission checks
      expect(result.current.user).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
      expect(result.current.projects).toEqual(['org-1', 'org-2']);
      expect(result.current.currentProject).toBe('org-1');
      expect(result.current.token).toBe(mockToken);
      expect(localStorage.getItem('jwt_token')).toBe(mockToken);
    });

    it('should handle admin user (permissions via Casbin)', () => {
      // NOTE: With ArgoCD-aligned pattern, admin status is determined by Casbin
      // permission checks at runtime via useCanI() hook, not stored as a boolean
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'admin-123',
        email: 'admin@example.com',
        name: 'Admin User',
        projects: [],
        default_project: '',
        casbin_roles: ['role:serveradmin'], // Admin role stored in JWT for Casbin
        permissions: { '*:*': true }, // Pre-computed permissions for UI hints
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      // User is stored without isGlobalAdmin - actual permission checks use useCanI()
      expect(result.current.user).toEqual({
        id: 'admin-123',
        email: 'admin@example.com',
        name: 'Admin User',
      });
      expect(result.current.isAuthenticated).toBe(true);
    });

    it('should not set state if token decode fails', () => {
      const mockToken = 'invalid.token';

      vi.mocked(jwtDecode).mockImplementation(() => {
        throw new Error('Invalid token');
      });

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
    });

    it('should fallback to first project when default_project is missing (E2E test token scenario)', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'platform-admin-alpha',
        email: 'platformadmin@test.com',
        name: 'Platform Admin User',
        projects: ['org-alpha', 'org-beta'],
        // default_project is intentionally missing (E2E test tokens don't include it)
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBe('org-alpha'); // Should fallback to projects[0]
      expect(result.current.projects).toEqual(['org-alpha', 'org-beta']);
    });

    it('should set currentProject to null when no projects and no default_project', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'global-admin',
        email: 'admin@test.com',
        name: 'Global Admin',
        projects: [], // No projects
        // default_project is intentionally missing
        casbin_roles: ['role:serveradmin'], // Global admin via Casbin role
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.currentProject).toBeNull(); // Should be null when no projects
      // NOTE: isGlobalAdmin was removed - permission checks use useCanI() hook
    });
  });

  describe('logout', () => {
    it('should clear all user state', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      // Login first
      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(true);

      // Logout
      act(() => {
        result.current.logout();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
      expect(result.current.projects).toEqual([]);
      expect(result.current.currentProject).toBeNull();
      expect(result.current.token).toBeNull();
      expect(localStorage.getItem('jwt_token')).toBeNull();
    });
  });

  describe('setCurrentProject', () => {
    beforeEach(() => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1', 'org-2', 'org-3'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());
      act(() => {
        result.current.login(mockToken);
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

      expect(result.current.currentProject).toBe('org-1'); // Should remain unchanged
      expect(consoleSpy).toHaveBeenCalledWith(
        expect.stringContaining("Project org-999 not found")
      );

      consoleSpy.mockRestore();
    });
  });

  describe('refreshUser', () => {
    it('should update user info from token', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        name: 'Original Name',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      // Update mock to return different claims
      const updatedClaims = {
        ...mockClaims,
        name: 'Updated Name',
        projects: ['org-1', 'org-2'],
      };
      vi.mocked(jwtDecode).mockReturnValue(updatedClaims);

      act(() => {
        result.current.refreshUser();
      });

      expect(result.current.user?.name).toBe('Updated Name');
      expect(result.current.projects).toEqual(['org-1', 'org-2']);
      expect(result.current.isAuthenticated).toBe(true);
    });

    it('should logout if token is expired', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) - 100, // Expired
        iat: Math.floor(Date.now() / 1000) - 3700,
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      // Login with valid token first
      const validClaims = { ...mockClaims, exp: Math.floor(Date.now() / 1000) + 3600 };
      vi.mocked(jwtDecode).mockReturnValue(validClaims);

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isAuthenticated).toBe(true);

      // Now set expired claims and refresh
      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      act(() => {
        result.current.refreshUser();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.user).toBeNull();
    });

    it('should do nothing if no token exists', () => {
      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.refreshUser();
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(vi.mocked(jwtDecode)).not.toHaveBeenCalled();
    });

    it('should fallback to first project when default_project missing on refresh', () => {
      const mockToken = 'mock.jwt.token';
      const initialClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(initialClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.currentProject).toBe('org-1');

      // Update mock to simulate E2E token without default_project
      const updatedClaims = {
        ...initialClaims,
        projects: ['org-alpha', 'org-beta'],
        default_project: undefined, // E2E test token scenario
      };
      vi.mocked(jwtDecode).mockReturnValue(updatedClaims as any);

      act(() => {
        result.current.refreshUser();
      });

      expect(result.current.currentProject).toBe('org-alpha'); // Should fallback to projects[0]
      expect(result.current.projects).toEqual(['org-alpha', 'org-beta']);
    });
  });

  describe('isTokenExpired', () => {
    it('should return true if no token exists', () => {
      const { result } = renderHook(() => useUserStore());

      expect(result.current.isTokenExpired()).toBe(true);
    });

    it('should return true if token is expired', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) - 100, // Expired
        iat: Math.floor(Date.now() / 1000) - 3700,
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isTokenExpired()).toBe(true);
    });

    it('should return false if token is valid', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600, // Valid for 1 hour
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      expect(result.current.isTokenExpired()).toBe(false);
    });
  });

  describe('persistence', () => {
    it('should persist currentProject to localStorage', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1', 'org-2'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      act(() => {
        result.current.setCurrentProject('org-2');
      });

      // Check that currentProject was persisted
      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.currentProject).toBe('org-2');
      }
    });

    it('should persist token to localStorage', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      // Check that token was persisted
      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.token).toBe(mockToken);
      }
    });

    it('should persist isAuthenticated to localStorage', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        projects: ['org-1'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      const { result } = renderHook(() => useUserStore());

      act(() => {
        result.current.login(mockToken);
      });

      // Check that isAuthenticated was persisted
      const storage = localStorage.getItem('user-storage');
      expect(storage).toBeTruthy();

      if (storage) {
        const parsed = JSON.parse(storage);
        expect(parsed.state.isAuthenticated).toBe(true);
      }
    });

    it('should rehydrate authentication state and refresh user on page load', () => {
      const mockToken = 'mock.jwt.token';
      const mockClaims = {
        sub: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
        projects: ['org-1', 'org-2'],
        default_project: 'org-1',
        casbin_roles: [], // Non-admin users have empty casbin_roles
        exp: Math.floor(Date.now() / 1000) + 3600,
        iat: Math.floor(Date.now() / 1000),
        iss: 'knodex',
        aud: 'knodex',
      };

      vi.mocked(jwtDecode).mockReturnValue(mockClaims);

      // Simulate a user logging in
      const { result: firstRender } = renderHook(() => useUserStore());
      act(() => {
        firstRender.current.login(mockToken);
      });

      // Verify state is persisted
      expect(firstRender.current.isAuthenticated).toBe(true);
      expect(firstRender.current.token).toBe(mockToken);

      // Simulate a page refresh by creating a new hook instance
      // The store should rehydrate from localStorage
      const { result: secondRender } = renderHook(() => useUserStore());

      // After rehydration, isAuthenticated and token should be restored
      expect(secondRender.current.isAuthenticated).toBe(true);
      expect(secondRender.current.token).toBe(mockToken);

      // User data should also be refreshed from the token
      // NOTE: isGlobalAdmin was removed - permission checks use useCanI() hook
      expect(secondRender.current.user).toEqual({
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
      });
      expect(secondRender.current.projects).toEqual(['org-1', 'org-2']);
    });
  });
});
