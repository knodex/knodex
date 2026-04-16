// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, setupPermissionMocking } from '../fixture'

const RGD_WITH_DOCS = {
  name: 'webapp-with-docs',
  namespace: 'default',
  description: 'Web application with documentation URL',
  version: 'v1.0.0',
  tags: ['webapp'],
  category: 'applications',
  labels: {},
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'WebApp',
  status: 'Active',
  docsUrl: 'https://docs.example.com/webapp',
  createdAt: '2026-01-15T10:30:00Z',
  updatedAt: '2026-01-20T14:45:00Z',
}

const RGD_WITHOUT_DOCS = {
  name: 'webapp-no-docs',
  namespace: 'default',
  description: 'Web application without documentation URL',
  version: 'v1.0.0',
  tags: ['webapp'],
  category: 'applications',
  labels: {},
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'WebApp',
  status: 'Active',
  createdAt: '2026-01-15T10:30:00Z',
  updatedAt: '2026-01-20T14:45:00Z',
}

function setupDetailMocks(page: Parameters<typeof setupPermissionMocking>[0], rgd: typeof RGD_WITH_DOCS | typeof RGD_WITHOUT_DOCS) {
  return Promise.all([
    setupPermissionMocking(page, { '*:*': true }),
    page.route(/\/api\/v1\/rgds(\?|$)/, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [rgd], totalCount: 1, page: 1, pageSize: 10 }),
      })
    ),
    page.route(`**/api/v1/rgds/${rgd.name}`, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rgd) })
    ),
    page.route('**/api/v1/dependencies/**', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ node: null, upstream: [], downstream: [], deploymentOrder: [], hasCycle: false }) })
    ),
    page.route('**/api/v1/schema/**', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ rgd: rgd.name, schema: null, crdFound: false }) })
    ),
  ])
}

test.describe('RGD Catalog Documentation URL', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Skipped: docsUrl was rendered on the Overview tab which was replaced by Instances tab.
  // Re-enable once docsUrl display is re-added to the detail view.
  test.skip('shows Documentation link when docsUrl annotation is set', async ({ page }) => {
    await setupDetailMocks(page, RGD_WITH_DOCS)
    await page.goto('/catalog/webapp-with-docs')
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 10000 })

    const docsLink = page.getByRole('link', { name: /view docs/i })
    await expect(docsLink).toBeVisible()
    await expect(docsLink).toHaveAttribute('href', 'https://docs.example.com/webapp')
    await expect(docsLink).toHaveAttribute('target', '_blank')
    await expect(docsLink).toHaveAttribute('rel', 'noopener noreferrer')
  })

  // Skipped: docsUrl was rendered on the Overview tab which was replaced by Instances tab.
  test.skip('hides Documentation row when docsUrl annotation is absent', async ({ page }) => {
    await setupDetailMocks(page, RGD_WITHOUT_DOCS)
    await page.goto('/catalog/webapp-no-docs')
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 10000 })

    await expect(page.getByRole('link', { name: /view docs/i })).not.toBeVisible()
  })
})
