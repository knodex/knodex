import { test, expect, TestUserRole } from '../fixture'
import {
  mockRGDListResponse,
  mockInstanceListResponse,
  mockInstances,
  API_PATHS,
} from '../fixture/mock-data'

test.describe('Instance List View', () => {
  // Authenticate as Global Admin to see all instances
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

    // Mock the instances API endpoint
    await page.route(`**${API_PATHS.instances}**`, async (route) => {
      const url = route.request().url()

      if (url.includes('/prod-db-1')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockInstances[0]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockInstanceListResponse),
        })
      }
    })

    // Navigate directly to instances page (sidebar is collapsed by default)
    await page.goto('/instances')
    // Wait for instances page to load
    await page.waitForURL('**/instances')
  })

  test('displays the instances page with active tab in sidebar', async ({ page }) => {
    // Hover over sidebar to expand it and show labels
    const sidebar = page.locator('aside')
    await sidebar.hover()

    // Should show instances tab as active (Link element in sidebar)
    const instancesTab = page.getByRole('link', { name: /instances/i })
    await expect(instancesTab).toBeVisible()
  })

  test('displays instance list', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()
    await expect(page.getByText('staging-cache')).toBeVisible()
    await expect(page.getByText('dev-ingress')).toBeVisible()
  })

  test('shows instance namespaces', async ({ page }) => {
    // Wait for content to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // Check that namespace information appears in instance cards (exclude options)
    await expect(page.locator('p:has-text("production")').first()).toBeVisible()
  })

  test('displays health status badges', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // Health status badge should be visible (look for span elements, not options)
    await expect(page.locator('span:has-text("Healthy")').first()).toBeVisible()
  })

  test('shows RGD name for each instance', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // RGD names should be visible in instance cards (exclude options)
    await expect(page.locator('p:has-text("postgres-database"), span:has-text("postgres-database")').first()).toBeVisible()
  })

  test('clicking instance navigates to detail view', async ({ page }) => {
    // Click on an instance card
    await page.getByRole('button', { name: /view details for prod-db-1/i }).click()

    // Should show instance detail with back button
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible()
  })

  test('can switch between catalog and instances tabs', async ({ page }) => {
    // Should be on instances tab
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // Hover over sidebar to expand it (sidebar is collapsed by default)
    const sidebar = page.locator('aside')
    await sidebar.hover()

    // Switch to catalog (Link element in sidebar)
    await page.getByRole('link', { name: /catalog/i }).click()
    await page.waitForURL('**/catalog')

    // Should show RGDs
    await expect(page.getByText('postgres-database')).toBeVisible()
    await expect(page.getByText('PostgreSQL database with automated backups and monitoring')).toBeVisible()

    // Hover over sidebar again to expand it
    await sidebar.hover()

    // Switch back to instances (Link element in sidebar)
    await page.getByRole('link', { name: /instances/i }).click()
    await page.waitForURL('**/instances')

    // Should show instances again
    await expect(page.getByText('prod-db-1')).toBeVisible()
  })

  test('shows kind information for instances', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // Kind information from mock data
    await expect(page.getByText('PostgresDatabase')).toBeVisible()
  })
})

test.describe('Instance Detail View', () => {
  // Authenticate as Global Admin to see all instance details
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    // Mock APIs
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRGDListResponse),
      })
    })

    await page.route(`**${API_PATHS.instances}**`, async (route) => {
      const url = route.request().url()

      if (url.includes('/prod-db-1')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockInstances[0]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockInstanceListResponse),
        })
      }
    })

    // Navigate directly to instances page (sidebar is collapsed by default)
    await page.goto('/instances')
    await page.waitForURL('**/instances')
    // Wait for instances to load before clicking
    const instanceCard = page.getByRole('button', { name: /view details for prod-db-1/i })
    await expect(instanceCard).toBeVisible()
    await instanceCard.click()
    // Wait for detail view to load
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible()
  })

  test('displays instance name and back button', async ({ page }) => {
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible()
    // Use heading role to avoid matching breadcrumb text
    await expect(page.getByRole('heading', { name: 'prod-db-1' })).toBeVisible()
  })

  test('shows instance health status', async ({ page }) => {
    await expect(page.getByText('Healthy')).toBeVisible()
  })

  test('displays instance namespace', async ({ page }) => {
    // Use exact match to avoid matching breadcrumb "production/prod-db-1"
    await expect(page.getByText('production', { exact: true })).toBeVisible()
  })

  test('clicking back returns to instances list', async ({ page }) => {
    await page.getByRole('button', { name: /back/i }).click()

    // Wait for navigation to instances URL
    await page.waitForURL('**/instances')

    // Wait for instances list to load (check for items not visible in detail view)
    await expect(page.getByText('staging-cache')).toBeVisible()
    await expect(page.getByText('dev-ingress')).toBeVisible()
    // prod-db-1 should also be visible in the list
    await expect(page.getByText('prod-db-1')).toBeVisible()
  })
})
