/**
 * Custom Playwright fixtures for E2E testing
 * Extends base Playwright test with authentication helpers
 */

/* eslint-disable react-hooks/rules-of-hooks */
import { test as base, expect } from '@playwright/test'
import {
  TestUserRole,
  setupAuth,
  switchRole,
  clearAuth,
  getCurrentUser,
  isAuthenticated,
} from './auth-helper'

/**
 * Extended test fixtures with authentication helpers
 */
type AuthFixtures = {
  /**
   * Authenticate as a specific user role before the test
   * Usage: test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })
   */
  authenticateAs: TestUserRole

  /**
   * Auth helper functions available in test context
   */
  auth: {
    setupAs: (role: TestUserRole) => Promise<void>
    switchTo: (role: TestUserRole) => Promise<void>
    clear: () => Promise<void>
    getCurrentUser: () => Promise<unknown>
    isAuthenticated: () => Promise<boolean>
  }
}

/**
 * Custom test with authentication fixtures
 */
export const test = base.extend<AuthFixtures>({
  // Auto-authenticate based on test configuration
  authenticateAs: [TestUserRole.UNAUTHENTICATED, { option: true }],

  // Setup authentication before each test if configured
  page: async ({ page, authenticateAs }, use) => {
    if (authenticateAs !== TestUserRole.UNAUTHENTICATED) {
      await setupAuth(page, authenticateAs)
    }
    await use(page)
  },

  // Provide auth helper methods in test context
  auth: async ({ page }, use) => {
    await use({
      setupAs: async (role: TestUserRole) => setupAuth(page, role),
      switchTo: async (role: TestUserRole) => switchRole(page, role),
      clear: async () => clearAuth(page),
      getCurrentUser: async () => getCurrentUser(page),
      isAuthenticated: async () => isAuthenticated(page),
    })
  },
})

// Re-export expect
export { expect }

// Re-export auth types and constants
export { TestUserRole } from './auth-helper'
export { TEST_USERS } from './auth-helper'

// Re-export auth functions for direct use
export {
  setupAuth,
  setupAuthAndNavigate,
  setupAuthWithToken,
  authenticateAs,
  authenticateWithToken,
  generateTestToken,
  switchRole,
  clearAuth,
  getCurrentUser,
  isAuthenticated,
  // Permission mocking for UI permission tests
  setupPermissionMocking,
  setupAuthWithMocking,
} from './auth-helper'

// Re-export types
export type { TestUser, PreloadedTestUser } from './auth-helper'

// Re-export enterprise helpers
