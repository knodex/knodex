// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import { mockRGDListResponse, mockRGDs, API_PATHS } from '../fixture/mock-data'

test.describe('Catalog View', () => {
  // Authenticate as Global Admin to see all catalog features
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    // Mock the RGD API endpoint
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRGDListResponse),
      })
    })

    // Navigate to catalog and wait for content
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
  })

  test('displays the catalog page with header', async ({ page }) => {
    // Verify header is visible
    await expect(page.getByRole('banner')).toBeVisible()

    // Verify navigation links exist in sidebar (using href since sidebar may be collapsed)
    await expect(page.locator('a[href="/catalog"]')).toBeVisible()
    await expect(page.locator('a[href="/instances"]')).toBeVisible()
  })

  test('displays RGD cards in the catalog', async ({ page }) => {
    // Wait for RGD names to appear (may appear in both "Recently Used" card and table row)
    await expect(page.getByText('postgres-database').first()).toBeVisible()
    await expect(page.getByText('redis-cache').first()).toBeVisible()
    await expect(page.getByText('nginx-ingress').first()).toBeVisible()
  })

  test('shows RGD descriptions', async ({ page }) => {
    // Default view is list/table mode which doesn't show descriptions.
    // Switch to grid view to see card descriptions.
    await page.getByRole('button', { name: /grid view/i }).click()

    await expect(
      page.getByText('PostgreSQL database with automated backups and monitoring')
    ).toBeVisible()
    await expect(
      page.getByText('Redis cache cluster for high-performance caching')
    ).toBeVisible()
  })

  test('displays category badges on RGD cards', async ({ page }) => {
    // Wait for content to load
    await expect(page.getByText('postgres-database')).toBeVisible()

    // Check that category information is present somewhere on the page
    // Categories appear as badges or filter options
    const main = page.locator('main')
    await expect(main.getByText(/database/i).first()).toBeVisible()
  })

  test('shows instance count on RGD cards', async ({ page }) => {
    // Wait for table to load
    await expect(page.getByText('postgres-database')).toBeVisible()

    // In list/table view, instance count is shown as a plain number in the Instances column.
    // Columns: name+icon(0), category(1), instances(2)
    const postgresRow = page.getByRole('button', { name: /view details for postgres-database/i })
    await expect(postgresRow).toBeVisible()
    await expect(postgresRow.locator('td').nth(2)).toHaveText('5')
  })

  test('can search for RGDs', async ({ page }) => {
    // Look for search input
    const searchInput = page.getByPlaceholder(/search/i)

    if (await searchInput.isVisible()) {
      await searchInput.fill('postgres')
      // The search should filter the results
      await expect(page.getByText('postgres-database')).toBeVisible()
    }
  })

  test('clicking an RGD card navigates to detail view', async ({ page }) => {
    // Mock detail-view endpoints needed after navigation
    await page.route('**/api/v1/rgds/postgres-database', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRGDs[0]),
      })
    })
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          node: null, upstream: [], downstream: [],
          deploymentOrder: ['postgres-database'], hasCycle: false,
        }),
      })
    })
    await page.route('**/api/v1/schema/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ rgd: 'postgres-database', schema: null, crdFound: false }),
      })
    })
    await setupPermissionMocking(page, { '*:*': true })

    // Click on the first RGD card/row using the role-based selector
    const firstCard = page.getByRole('button', { name: /view details for/i }).first()
    await expect(firstCard).toBeVisible()
    await firstCard.click()

    // Wait for detail view to load
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Should show detail view with Instances tab (confirms navigation to detail page)
    await expect(page.getByRole('tab', { name: /Instances/i })).toBeVisible()
  })

  test('shows Documentation link in sidebar', async ({ page }) => {
    // On the catalog route the sidebar shows category sub-navigation.
    // Navigate to a non-catalog route (e.g., instances) to see the main sidebar
    // which contains the Documentation link.
    await page.goto('/instances')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('a[href="https://knodex.io/docs"]')).toBeVisible()
  })
})
