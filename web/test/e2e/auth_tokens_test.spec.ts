/**
 * Authentication E2E Tests using JWT Token Injection
 * Based on approach - bypasses login forms by injecting tokens
 */

import { test, expect, TestUserRole } from '../fixture'

test.describe('Authentication with Token Injection', () => {
  test.describe('Global Admin Authentication', () => {
    // Authenticate as Global Admin before each test in this group
    test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

    test('should be authenticated as global admin', async ({ page, auth }) => {
      await page.goto('/')

      // Verify authentication
      const isAuth = await auth.isAuthenticated()
      expect(isAuth).toBe(true)

      // Verify user context
      // Global admin is now determined by casbin_roles containing 'role:serveradmin'
      const user = await auth.getCurrentUser()
      expect(user.casbinRoles).toContain('role:serveradmin')
      expect(user.email).toBe('admin@e2e-test.local')
    })

    test('should have access to dashboard', async ({ page }) => {
      await page.goto('/')

      // Should show dashboard, not login page
      await expect(page).not.toHaveURL(/\/login/)

      // Should show authenticated user interface (user name displayed proves user is logged in)
      // The admin username is shown in the header/topbar
      await expect(page.getByText('admin')).toBeVisible({ timeout: 5000 })
    })
  })

  test.describe('Project User Authentication', () => {
    test.use({ authenticateAs: TestUserRole.ORG_DEVELOPER })

    test('should be authenticated as project developer', async ({ page, auth }) => {
      await page.goto('/')

      // Non-admin users have empty casbinRoles (no 'role:serveradmin')
      const user = await auth.getCurrentUser()
      expect(user.casbinRoles).not.toContain('role:serveradmin')
      expect(user.projects).toContain('proj-alpha-team')
      expect(user.roles?.['proj-alpha-team']).toBe('developer')
    })
  })

  test.describe('Unauthenticated Access', () => {
    // Don't authenticate - test as unauthenticated user
    test('should redirect to login when not authenticated', async ({ page }) => {
      await page.goto('/')

      // Should redirect to login
      await expect(page).toHaveURL(/\/login/)
    })
  })

  test.describe('Role Switching', () => {
    test('should allow switching between roles', async ({ page, auth }) => {
      // Start as global admin
      await auth.setupAs(TestUserRole.GLOBAL_ADMIN)
      await page.goto('/')

      // Global admin is determined by casbin_roles
      let user = await auth.getCurrentUser()
      expect(user.casbinRoles).toContain('role:serveradmin')

      // Switch to viewer
      await auth.switchTo(TestUserRole.ORG_VIEWER)
      await page.goto('/')

      user = await auth.getCurrentUser()
      expect(user.casbinRoles).not.toContain('role:serveradmin')
      expect(user.roles?.['proj-alpha-team']).toBe('viewer')
    })
  })

  test.describe('Logout', () => {
    test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

    test('should clear authentication on logout', async ({ page, auth }) => {
      await page.goto('/')

      // Verify authenticated
      expect(await auth.isAuthenticated()).toBe(true)

      // Logout
      await auth.clear()

      // Should no longer be authenticated
      expect(await auth.isAuthenticated()).toBe(false)

      // Should redirect to login
      await page.goto('/')
      await expect(page).toHaveURL(/\/login/)
    })
  })
})
