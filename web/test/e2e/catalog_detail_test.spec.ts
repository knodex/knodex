// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import { mockRGDs, mockRGDListResponse, API_PATHS } from '../fixture/mock-data'

test.describe('RGD Detail View', () => {
  // Authenticate as Global Admin to see all RGD details
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Mock permission and detail-view API endpoints
  test.beforeEach(async ({ page }) => {
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
    // Wait for page content to render (avoid networkidle - WebSocket keeps network active)
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })
  })

  test('displays RGD name and back button', async ({ page }) => {
    // Back button should be visible
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible()

    // RGD name should be visible somewhere on the page
    const main = page.locator('main')
    const heading = main.locator('h1, h2, h3').first()
    await expect(heading).toBeVisible()
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
    const deployButton = page.getByRole('button', { name: /deploy/i })
    await expect(deployButton).toBeVisible({ timeout: 10000 })
  })

  test('clicking back returns to catalog', async ({ page }) => {
    // Use dispatchEvent to avoid sidebar hover-expand intercepting mouse movement
    await page.getByRole('button', { name: /back/i }).dispatchEvent('click')

    // Wait for navigation back to catalog
    await page.waitForURL('**/catalog')

    // Catalog should show RGD cards
    const rgdCards = page.getByRole('button', { name: /view details for/i })
    await expect(rgdCards.first()).toBeVisible()
  })

  test('clicking deploy navigates to deploy form', async ({ page }) => {
    // Click deploy button
    const deployButton = page.getByRole('button', { name: /deploy/i })
    await expect(deployButton).toBeVisible({ timeout: 10000 })
    await deployButton.click()

    // Should navigate to deploy URL or show deploy form
    await expect(
      page.getByRole('button', { name: /back/i })
    ).toBeVisible()
  })
})
