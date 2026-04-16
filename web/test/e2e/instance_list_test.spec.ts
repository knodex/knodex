// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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
    const instanceHandler = async (route: import('@playwright/test').Route) => {
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
    }
    await page.route(`**${API_PATHS.instances}**`, instanceHandler)
    await page.route(`**${API_PATHS.namespacedInstances}**`, instanceHandler)

    // Navigate directly to instances page (sidebar is collapsed by default)
    await page.goto('/instances')
    // Wait for instances page to load
    await page.waitForURL('**/instances')
  })

  test('displays the instances page with active tab in sidebar', async ({ page }) => {
    // Hover over sidebar to expand it and show labels
    const sidebar = page.locator('aside')
    await sidebar.hover()

    // Should show instances tab as active (scope to aside to avoid SidebarDrawer duplicate)
    const instancesTab = sidebar.getByRole('link', { name: /instances/i })
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

    // Check that namespace information appears in instance cards
    await expect(page.getByText('production').first()).toBeVisible()
  })

  test('displays health status badges', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // Switch to list view where health labels are always visible
    const listViewButton = page.getByRole('button', { name: /list/i }).or(page.locator('[data-testid="view-list"]'))
    if (await listViewButton.isVisible()) {
      await listViewButton.click()
    }

    // Health status badge should be visible in list view
    await expect(page.getByText('Healthy').first()).toBeVisible({ timeout: 10000 })
  })

  test('shows RGD name for each instance', async ({ page }) => {
    // Wait for instances to load
    await expect(page.getByText('prod-db-1')).toBeVisible()

    // RGD names should be visible in instance cards (RGD: label + name)
    await expect(page.getByText('PostgresDatabase').first()).toBeVisible()
  })

  test('clicking instance navigates to detail view', async ({ page }) => {
    // Click on an instance card
    await page.getByRole('button', { name: /view details for prod-db-1/i }).click()

    // Should show instance detail with heading (breadcrumbs handle navigation)
    await expect(page.getByRole('heading', { name: 'prod-db-1' })).toBeVisible()
  })

  // Removed: "can switch between catalog and instances tabs" — catalog sidebar sub-nav
  // is conditional on category count, making this test environment-dependent.

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

    const instanceHandler2 = async (route: import('@playwright/test').Route) => {
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
    }
    await page.route(`**${API_PATHS.instances}**`, instanceHandler2)
    await page.route(`**${API_PATHS.namespacedInstances}**`, instanceHandler2)

    // Navigate directly to instances page (sidebar is collapsed by default)
    await page.goto('/instances')
    await page.waitForURL('**/instances')
    // Wait for instances to load before clicking
    const instanceCard = page.getByRole('button', { name: /view details for prod-db-1/i })
    await expect(instanceCard).toBeVisible()
    await instanceCard.click()
    // Wait for detail view to load (heading indicates page is rendered)
    await expect(page.getByRole('heading', { name: 'prod-db-1' })).toBeVisible()
  })

  test('displays instance name and heading', async ({ page }) => {
    // Breadcrumbs handle navigation, no back button exists
    await expect(page.getByRole('heading', { name: 'prod-db-1' })).toBeVisible()
  })

  test('shows instance health status', async ({ page }) => {
    await expect(page.getByText('Healthy')).toBeVisible()
  })

  test('displays instance namespace', async ({ page }) => {
    // Use exact match to avoid matching breadcrumb "production/prod-db-1"
    await expect(page.getByText('production', { exact: true })).toBeVisible()
  })

  test('navigating back returns to instances list', async ({ page }) => {
    // Use browser back navigation (breadcrumbs handle navigation, no back button)
    await page.goBack()

    // Wait for navigation to instances URL
    await page.waitForURL('**/instances')

    // Wait for instances list to load (check for items not visible in detail view)
    await expect(page.getByText('staging-cache')).toBeVisible()
    await expect(page.getByText('dev-ingress')).toBeVisible()
    // prod-db-1 should also be visible in the list
    await expect(page.getByText('prod-db-1')).toBeVisible()
  })
})
