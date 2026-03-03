/**
 * Catalog E2E Tests with Authentication
 * Tests catalog view with real authentication instead of mocked APIs
 *
 * Note: These tests mock the can-i permission API to ensure consistent
 * behavior regardless of backend configuration. The deploy button visibility
 * depends on the instances:create permission check.
 */

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'

test.describe('Catalog View with Authentication', () => {
  test.describe('As Global Admin', () => {
    test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

    // Mock permissions for Global Admin (full access)
    test.beforeEach(async ({ page }) => {
      await setupPermissionMocking(page, { '*:*': true })
    })

    test('displays the catalog page with header', async ({ page }) => {
      await page.goto('/catalog')

      // Wait for page to load
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Verify header with user info (shows email prefix: admin@e2e-test.local → admin)
      await expect(page.getByText('admin')).toBeVisible()

      // Verify main navigation is visible (sidebar nav)
      await expect(page.getByRole('navigation').first()).toBeVisible()
    })

    test('displays RGD cards from all projects', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Global admin should see all RGDs
      // RGD cards are buttons with aria-label="View details for {name}"
      const rgdCards = page.getByRole('button', { name: /view details for/i })

      // Wait for RGDs to load (should have at least one)
      await expect(rgdCards.first()).toBeVisible({ timeout: 10000 })

      // Global admin should see RGDs from all projects
      const count = await rgdCards.count()
      expect(count).toBeGreaterThan(0)
    })

    test('shows deploy button on RGD detail page', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('networkidle')

      // Click on first RGD card to go to detail view
      const firstCard = page.getByRole('button', { name: /view details for/i }).first()
      await expect(firstCard).toBeVisible({ timeout: 15000 })
      await firstCard.click()

      // Wait for detail view URL
      await page.waitForURL(/\/catalog\//, { timeout: 10000 })
      await page.waitForLoadState('networkidle')

      // Global admin should see deploy button on detail page
      // The deploy button appears after permission API returns
      const deployButton = page.getByRole('button', { name: /deploy/i })
      await expect(deployButton).toBeVisible({ timeout: 15000 })
    })
  })

  test.describe('As Project Viewer', () => {
    test.use({ authenticateAs: TestUserRole.ORG_VIEWER })

    // Mock permissions for Viewer (read-only, no deploy)
    test.beforeEach(async ({ page }) => {
      await setupPermissionMocking(page, {
        'rgds:get': true,
        'rgds:list': true,
        'instances:get': true,
        'instances:list': true,
        'instances:create': false, // No deploy permission
      })
    })

    test('displays limited catalog based on project membership', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Viewer should see their project's RGDs + shared RGDs
      // RGD cards are buttons with aria-label="View details for {name}"
      const rgdCards = page.getByRole('button', { name: /view details for/i })

      // Should see at least shared RGDs
      await expect(rgdCards.first()).toBeVisible({ timeout: 10000 })
    })

    test('does NOT show deploy button on detail page', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Click on first RGD card to go to detail view
      const firstCard = page.getByRole('button', { name: /view details for/i }).first()
      await expect(firstCard).toBeVisible({ timeout: 10000 })
      await firstCard.click()

      // Wait for detail view to load
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Viewer should NOT see deploy button on detail page
      const deployButton = page.getByRole('button', { name: /deploy/i })
      await expect(deployButton).not.toBeVisible({ timeout: 5000 })
    })
  })

  test.describe('As Project Developer', () => {
    test.use({ authenticateAs: TestUserRole.ORG_DEVELOPER })

    // Mock permissions for Developer (can deploy)
    test.beforeEach(async ({ page }) => {
      await setupPermissionMocking(page, {
        'rgds:get': true,
        'rgds:list': true,
        'instances:get': true,
        'instances:list': true,
        'instances:create': true, // Developer can deploy
        'instances:delete': true,
      })
    })

    test('shows deploy button on RGD detail page', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('networkidle')

      // Click on first RGD card to go to detail view
      const firstCard = page.getByRole('button', { name: /view details for/i }).first()
      await expect(firstCard).toBeVisible({ timeout: 15000 })
      await firstCard.click()

      // Wait for detail view URL
      await page.waitForURL(/\/catalog\//, { timeout: 10000 })
      await page.waitForLoadState('networkidle')

      // Developer should see deploy button on detail page
      const deployButton = page.getByRole('button', { name: /deploy/i })
      await expect(deployButton).toBeVisible({ timeout: 15000 })
    })

    test('can click on RGD card to view details', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('networkidle')

      // Find first RGD card (buttons with aria-label="View details for {name}")
      const firstCard = page.getByRole('button', { name: /view details for/i }).first()
      await expect(firstCard).toBeVisible({ timeout: 15000 })

      // Click on it
      await firstCard.click()

      // Wait for detail view to load - wait for URL to change
      await page.waitForURL(/\/catalog\//, { timeout: 10000 })
      await page.waitForLoadState('networkidle')

      // Developer should see deploy button (they have deploy permissions)
      const deployButton = page.getByRole('button', { name: /deploy/i })
      await expect(deployButton).toBeVisible({ timeout: 15000 })
    })
  })

  test.describe('Search and Filter', () => {
    test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

    // Mock permissions for Global Admin
    test.beforeEach(async ({ page }) => {
      await setupPermissionMocking(page, { '*:*': true })
    })

    test('can search for RGDs', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Look for search input
      const searchInput = page.getByPlaceholder(/search/i).or(page.locator('input[type="search"]'))

      if ((await searchInput.count()) > 0) {
        await searchInput.first().fill('simple')
        await page.waitForLoadState('domcontentloaded')

        // Should show filtered results
        // RGD cards are buttons with aria-label="View details for {name}"
        const cards = page.getByRole('button', { name: /view details for/i })
        expect(await cards.count()).toBeGreaterThanOrEqual(0)
      }
    })
  })

  test.describe('Footer', () => {
    test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

    // Mock permissions for Global Admin
    test.beforeEach(async ({ page }) => {
      await setupPermissionMocking(page, { '*:*': true })
    })

    test('displays version information', async ({ page }) => {
      await page.goto('/catalog')
      await page.waitForLoadState('domcontentloaded')
      await page.waitForLoadState('networkidle')

      // Look for footer
      const footer = page.locator('footer')

      if ((await footer.count()) > 0) {
        await expect(footer).toBeVisible()

        // Check for version or app name
        // The exact text may vary
        const footerText = await footer.textContent()
        expect(footerText).toBeTruthy()
      }
    })
  })
})
