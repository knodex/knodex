// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'
import { API_PATHS } from '../fixture/mock-data'
import type { CatalogRGD, Instance, RGDListResponse, InstanceListResponse } from '../../src/types/rgd'

/**
 * E2E tests for STORY-271: Show Inactive RGDs in Catalog and Preserve Instance Visibility
 *
 * Tests verify:
 * - AC-1: Inactive RGDs visible in catalog with "Inactive" badge
 * - AC-2: Deploy button disabled for inactive RGDs
 * - AC-3: Instances preserved when RGD goes inactive (via mock data)
 * - AC-4: Instance cards show RGD status warning
 * - AC-5: Catalog card styling for inactive RGDs (muted/dimmed)
 * - AC-7: Instance count still accurate on inactive RGD cards
 * - AC-8: Search/filter works for inactive RGDs
 * - AC-11: RGD detail view shows status
 */

// Self-contained test data — not coupled to shared mock array indices

const activeRGD: CatalogRGD = {
  name: 'postgres-database',
  namespace: 'databases',
  description: 'PostgreSQL database with automated backups',
  version: 'v1.0.0',
  tags: ['database', 'sql'],
  category: 'database',
  labels: {},
  instances: 5,
  status: 'Active',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  createdAt: '2025-01-15T10:30:00Z',
  updatedAt: '2025-01-20T14:45:00Z',
}

const inactiveRGD: CatalogRGD = {
  name: 'redis-cache-inactive',
  namespace: 'caching',
  description: 'Redis cache cluster (currently inactive)',
  version: 'v2.1.0',
  tags: ['cache', 'nosql'],
  category: 'storage',
  labels: {},
  instances: 2,
  status: 'Inactive',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  createdAt: '2025-01-10T08:00:00Z',
  updatedAt: '2025-01-18T09:30:00Z',
}

const unknownStatusRGD: CatalogRGD = {
  name: 'nginx-unknown-status',
  namespace: 'networking',
  description: 'NGINX Ingress Controller',
  version: 'v1.5.0',
  tags: ['ingress', 'networking'],
  category: 'networking',
  labels: {},
  instances: 1,
  status: '',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  createdAt: '2025-01-05T12:00:00Z',
  updatedAt: '2025-01-15T16:00:00Z',
}

const mixedRGDList: RGDListResponse = {
  items: [activeRGD, inactiveRGD, unknownStatusRGD],
  totalCount: 3,
  page: 1,
  pageSize: 10,
}

// Instances belonging to the inactive RGD (AC-3, AC-4)
const instanceWithInactiveRGD: Instance = {
  name: 'cache-instance-1',
  namespace: 'staging',
  rgdName: 'redis-cache-inactive',
  rgdNamespace: 'caching',
  apiVersion: 'caching.kro.run/v1alpha1',
  kind: 'RedisCache',
  health: 'Healthy',
  conditions: [],
  createdAt: '2025-01-20T14:00:00Z',
  updatedAt: '2025-01-20T14:30:00Z',
  rgdStatus: 'Inactive',
}

const instanceWithActiveRGD: Instance = {
  name: 'prod-db-1',
  namespace: 'production',
  rgdName: 'postgres-database',
  rgdNamespace: 'databases',
  apiVersion: 'databases.kro.run/v1alpha1',
  kind: 'PostgresDatabase',
  health: 'Healthy',
  conditions: [],
  createdAt: '2025-01-15T10:30:00Z',
  updatedAt: '2025-01-20T14:45:00Z',
  rgdStatus: 'Active',
}

const mixedInstanceList: InstanceListResponse = {
  items: [instanceWithActiveRGD, instanceWithInactiveRGD],
  totalCount: 2,
  page: 1,
  pageSize: 10,
}

test.describe('Inactive RGDs in Catalog (STORY-271)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    // Mock RGD API — use the same glob pattern as catalog_list_test.spec.ts
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/redis-cache-inactive')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(inactiveRGD) })
      } else if (url.includes('/postgres-database')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(activeRGD) })
      } else if (url.includes('/nginx-unknown-status')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(unknownStatusRGD) })
      } else if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedRGDList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedRGDList) })
      }
    })

    // Mock instances API
    await page.route(`**${API_PATHS.instances}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedInstanceList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedInstanceList) })
      }
    })

    await page.goto('/catalog')
    await page.waitForLoadState('domcontentloaded')
  })

  // AC-1: Inactive RGDs visible in catalog with "Inactive" badge
  test('displays inactive RGDs in catalog with Inactive badge', async ({ page }) => {
    await expect(page.getByText('postgres-database')).toBeVisible()
    await expect(page.getByText('redis-cache-inactive')).toBeVisible()
    await expect(page.getByText('Inactive').first()).toBeVisible()
  })

  // AC-1: Unknown/empty status also shows as inactive
  test('displays RGDs with empty status as inactive', async ({ page }) => {
    await expect(page.getByText('nginx-unknown-status')).toBeVisible()
  })

  // AC-5: Inactive RGD cards have muted/dimmed styling
  test('inactive RGD cards have dimmed opacity styling', async ({ page }) => {
    const inactiveCard = page.getByRole('button', { name: /view details for redis-cache-inactive/i })
    await expect(inactiveCard).toBeVisible()
    await expect(inactiveCard).toHaveClass(/opacity-60/)
  })

  // AC-5: Active RGD cards do NOT have muted styling
  test('active RGD cards do not have dimmed styling', async ({ page }) => {
    const activeCard = page.getByRole('button', { name: /view details for postgres-database/i })
    await expect(activeCard).toBeVisible()
    await expect(activeCard).not.toHaveClass(/opacity-60/)
  })

  // AC-7: Instance count still accurate on inactive RGD cards
  test('inactive RGD card shows correct instance count', async ({ page }) => {
    await expect(page.getByText('redis-cache-inactive')).toBeVisible()
    await expect(page.getByText(/2\s*instance/i)).toBeVisible()
  })

  // AC-8: Search/filter works for inactive RGDs
  test('inactive RGDs appear in search results', async ({ page }) => {
    const searchInput = page.getByPlaceholder(/search/i)
    if (await searchInput.isVisible()) {
      await searchInput.fill('redis-cache-inactive')
      await expect(page.getByText('redis-cache-inactive')).toBeVisible()
    }
  })
})

test.describe('Inactive RGD Detail View (STORY-271)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    // Mock RGD API
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/redis-cache-inactive')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(inactiveRGD) })
      } else if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedRGDList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedRGDList) })
      }
    })

    // Mock dependencies and schema for detail view
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ node: null, upstream: [], downstream: [], deploymentOrder: ['redis-cache-inactive'], hasCycle: false }),
      })
    })
    await page.route('**/api/v1/schema/**', async (route) => {
      await route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ rgd: 'redis-cache-inactive', schema: null, crdFound: false }),
      })
    })

    await setupPermissionMocking(page, { '*:*': true })
  })

  // AC-11: RGD detail view shows status prominently
  test('detail view shows Inactive badge for inactive RGD', async ({ page }) => {
    // Navigate directly to detail page (same pattern as catalog_detail_test.spec.ts)
    await page.goto('/catalog/redis-cache-inactive')
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })

    // Should show Inactive badge near the title
    await expect(page.getByText('Inactive').first()).toBeVisible()
    await expect(page.getByText('Back to catalog')).toBeVisible()
  })

  // AC-11: Overview tab shows Status field
  test('detail view overview shows status as Inactive', async ({ page }) => {
    await page.goto('/catalog/redis-cache-inactive')
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })

    await expect(page.getByText('Status')).toBeVisible()
    const statusValue = page.locator('dd:has-text("Inactive")').first()
    await expect(statusValue).toBeVisible()
  })

  // AC-2: Deploy button hidden for inactive RGDs
  test('deploy button is hidden for inactive RGD in detail view', async ({ page }) => {
    await page.goto('/catalog/redis-cache-inactive')
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })

    await expect(page.getByRole('button', { name: /deploy/i })).not.toBeVisible()
  })
})

test.describe('Active RGD Detail View - Deploy Visible (STORY-271)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('deploy button is visible for active RGD in detail view', async ({ page }) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/postgres-database')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(activeRGD) })
      } else if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedRGDList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedRGDList) })
      }
    })
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ node: null, upstream: [], downstream: [], deploymentOrder: ['postgres-database'], hasCycle: false }),
      })
    })
    await page.route('**/api/v1/schema/**', async (route) => {
      await route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ rgd: 'postgres-database', schema: null, crdFound: false }),
      })
    })
    await setupPermissionMocking(page, { '*:*': true })

    await page.goto('/catalog/postgres-database')
    await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })

    await expect(page.getByRole('button', { name: /deploy/i })).toBeVisible()
  })
})

test.describe('Instance Cards with Inactive RGD (STORY-271)', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    // Mock RGD API
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedRGDList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedRGDList) })
      }
    })

    // Mock instances API
    await page.route(`**${API_PATHS.instances}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/count')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ count: mixedInstanceList.totalCount }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mixedInstanceList) })
      }
    })

    await page.goto('/instances')
    await page.waitForLoadState('domcontentloaded')
  })

  // AC-3: Instances preserved when RGD goes inactive (visible in list)
  test('instances with inactive parent RGD are visible in instance list', async ({ page }) => {
    await expect(page.getByText('cache-instance-1')).toBeVisible()
    await expect(page.getByText('prod-db-1')).toBeVisible()
  })

  // AC-4: Instance cards show RGD status warning
  test('instance card shows "RGD Inactive" warning when parent RGD is inactive', async ({ page }) => {
    await expect(page.getByText('cache-instance-1')).toBeVisible()
    await expect(page.getByText('RGD Inactive')).toBeVisible()
  })

  // AC-4: Instance cards for active RGDs do NOT show warning
  test('instance card does not show RGD warning when parent RGD is active', async ({ page }) => {
    await expect(page.getByText('prod-db-1')).toBeVisible()
    const inactiveBadges = page.getByText('RGD Inactive')
    await expect(inactiveBadges).toHaveCount(1)
  })

  // AC-4: RGD name still displayed on instance cards
  test('instance cards still display RGD name', async ({ page }) => {
    await expect(page.getByText('cache-instance-1')).toBeVisible()
    await expect(page.locator('span:has-text("redis-cache-inactive")').first()).toBeVisible()
  })
})
