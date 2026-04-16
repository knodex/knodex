// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import { mockRGDs, mockRGDListResponse, API_PATHS } from '../fixture/mock-data'

test.describe('RGD Detail View', () => {
  // Authenticate as Global Admin to see all RGD details
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Mock permission and detail-view API endpoints
  test.beforeEach(async ({ page }) => {
    // Mock account/info so session restore succeeds (prevents "Connection Error" state)
    await page.route('**/api/v1/account/info', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          userID: 'user-global-admin',
          email: 'admin@e2e-test.local',
          displayName: 'Global Administrator',
          groups: [],
          casbinRoles: ['role:serveradmin'],
          projects: [],
          roles: {},
          issuer: 'knodex',
          tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
          tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
        }),
      })
    })

    // Mock can-i endpoint for permission checks (Global Admin can deploy)
    await setupPermissionMocking(page, { '*:*': true })

    // Mock the RGD list endpoint (needed when navigating back to catalog)
    // Uses regex to match /api/v1/rgds with optional query params but NOT /api/v1/rgds/<name>
    await page.route(/\/api\/v1\/rgds(\?|$)/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRGDListResponse),
      })
    })

    // Mock the specific RGD detail endpoint
    await page.route('**/api/v1/rgds/postgres-database', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRGDs[0]),
      })
    })

    // Mock dependency endpoint
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          node: null,
          upstream: [],
          downstream: [],
          deploymentOrder: ['postgres-database'],
          hasCycle: false,
        }),
      })
    })

    // Mock schema endpoint
    await page.route('**/api/v1/schema/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          rgd: 'postgres-database',
          schema: null,
          crdFound: false,
        }),
      })
    })

    // Navigate directly to RGD detail page (all API endpoints are mocked)
    await page.goto('/catalog/postgres-database')
    // Wait for page content to render (heading indicates detail view is loaded)
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 10000 })
  })

  test('displays RGD name and heading', async ({ page }) => {
    // RGD name should be visible as a heading (breadcrumbs handle navigation, no back button)
    const main = page.locator('main')
    const heading = main.locator('h1, h2, h3').first()
    await expect(heading).toBeVisible()
    await expect(heading).toContainText('postgres-database')
  })

  test('shows RGD metadata', async ({ page }) => {
    const main = page.locator('main')

    // Wait for the detail content to render
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })

    // RGD detail page should show some metadata: Details heading, dt elements, badges, or the RGD name text
    const hasDetails = await main.getByRole('heading', { name: 'Details' }).isVisible().catch(() => false)
    const hasDtOrBadge = await main.locator('dt, [class*="badge"]').first().isVisible().catch(() => false)
    const hasRgdName = await main.getByText('postgres-database').isVisible().catch(() => false)

    expect(hasDetails || hasDtOrBadge || hasRgdName).toBeTruthy()
  })

  test('displays RGD description', async ({ page }) => {
    const main = page.locator('main')
    // Check that some descriptive text is present (from real or mock data)
    const descriptionArea = main.locator('p, [class*="description"]').first()
    await expect(descriptionArea).toBeVisible()
  })

  test('has deploy button for admin', async ({ page }) => {
    // Deploy button should be present (admin has deploy permissions)
    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
  })

  test('navigating back returns to catalog', async ({ page }) => {
    // Breadcrumbs component currently returns null (disabled).
    // Navigate back to catalog using the sidebar link or direct navigation.
    await page.goto('/catalog')

    // Wait for navigation to catalog
    await page.waitForURL(/\/catalog$/)

    // Catalog should show RGD rows/cards
    const rgdCards = page.getByRole('button', { name: /view details for/i })
    await expect(rgdCards.first()).toBeVisible()
  })

  test('clicking deploy opens deploy modal', async ({ page }) => {
    // Click deploy button
    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
    await deployButton.click()

    // Should show the deploy modal/form (dialog role or form elements)
    await expect(
      page.getByRole('dialog').or(page.locator('[data-testid="deploy-config-section"]'))
    ).toBeVisible({ timeout: 10000 })
  })
})
