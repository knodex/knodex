// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, TEST_USERS, setupPermissionMocking } from '../fixture'
import { API_PATHS } from '../fixture/mock-data'

/**
 * Mock RGD for multi-controller conditional visibility testing.
 * Uses the per-feature nested pattern:
 *   - database.enabled + cache.enabled are per-feature boolean toggles
 *   - sharedLabel is used by BOTH conditional resources (AND-based hiding)
 *   - name, image, port are used by non-conditional resource (always visible)
 *   - database/cache containers are always visible (parent of controlling field)
 *   - database peer fields + cache peer fields shown only when enabled
 */
const mockWebAppRGD = {
  name: 'webapp-full-featured',
  namespace: '',
  description: 'Full-featured web app with conditional database and cache',
  version: 'v1.0.0',
  tags: ['webapp', 'conditions', 'test'],
  category: 'applications',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  createdAt: '2026-02-18T10:00:00Z',
  updatedAt: '2026-02-18T10:00:00Z',
}

const mockWebAppSchema = {
  crdFound: true,
  schema: {
    group: 'kro.run',
    version: 'v1alpha1',
    kind: 'WebApp',
    description: 'Full-featured web app with conditional database and cache',
    properties: {
      name: {
        type: 'string',
        description: 'Application name',
      },
      image: {
        type: 'string',
        description: 'Container image',
        default: 'nginx:latest',
      },
      port: {
        type: 'integer',
        description: 'Container port',
        default: 8080,
      },
      sharedLabel: {
        type: 'string',
        description: 'Label used ONLY by conditional resources (hidden when both features off)',
        default: '',
      },
      database: {
        type: 'object',
        description: 'Database feature',
        properties: {
          enabled: {
            type: 'boolean',
            description: 'Enable PostgreSQL database',
            default: false,
          },
          name: {
            type: 'string',
            description: 'Database name',
            default: 'mydb',
          },
        },
      },
      cache: {
        type: 'object',
        description: 'Cache feature',
        properties: {
          enabled: {
            type: 'boolean',
            description: 'Enable Redis cache',
            default: false,
          },
          maxMemory: {
            type: 'string',
            description: 'Max memory for cache',
            default: '256mb',
          },
        },
      },
    },
    required: ['name'],
    conditionalSections: [
      {
        condition: 'schema.spec.database.enabled == true',
        controllingField: 'spec.database.enabled',
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: 'spec.database.enabled', op: '==', value: true }],
        affectedProperties: ['sharedLabel'],
      },
      {
        condition: 'schema.spec.cache.enabled == true',
        controllingField: 'spec.cache.enabled',
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: 'spec.cache.enabled', op: '==', value: true }],
        affectedProperties: ['sharedLabel'],
      },
    ],
  },
}

/**
 * Helper: set up all API mocks for the webapp-full-featured RGD.
 * MUST be called BEFORE any navigation (network-first pattern).
 */
async function setupWebAppMocks(page: import('@playwright/test').Page) {
  const adminUser = TEST_USERS[TestUserRole.GLOBAL_ADMIN]

  // Mock /api/v1/account/info for session restore so tests don't depend on real server
  await page.route('**/api/v1/account/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        userID: adminUser.sub,
        email: adminUser.email,
        displayName: adminUser.displayName,
        groups: [],
        casbinRoles: adminUser.casbinRoles,
        projects: adminUser.projects,
        roles: adminUser.roles || {},
        issuer: 'knodex',
        tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
        tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
      }),
    })
  })

  // Use the proven permission mocking pattern (same as catalog_detail_test)
  await setupPermissionMocking(page, { '*:*': true })

  // Mock RGD detail endpoint (use regex to match only API calls, not page navigation)
  await page.route(/\/api\/v1\/rgds\/webapp-full-featured\/schema/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockWebAppSchema),
    })
  })

  await page.route(/\/api\/v1\/rgds\/webapp-full-featured\/validate-deployment/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ valid: true, errors: [] }),
    })
  })

  // Match the RGD detail endpoint (not schema/validate sub-paths, not page navigation)
  await page.route(/\/api\/v1\/rgds\/webapp-full-featured(\?|$)/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockWebAppRGD),
    })
  })

  // Mock dependencies
  await page.route('**/api/v1/dependencies/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: null,
        upstream: [],
        downstream: [],
        deploymentOrder: ['webapp-full-featured'],
        hasCycle: false,
      }),
    })
  })

  // Mock K8s resources (empty - no ExternalRef in this RGD)
  await page.route('**/api/v1/resources**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], count: 0 }),
    })
  })

  // Mock projects and namespaces (required by DeployPage's project selector)
  await page.route('**/api/v1/projects**', async (route) => {
    const url = route.request().url()
    if (url.includes('/namespaces')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ namespaces: ['default'] }),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{ name: 'default-project', destinations: [{ namespace: 'default' }] }],
          totalCount: 1,
        }),
      })
    }
  })

  // Mock repositories (required by DeployPage's deployment mode selector)
  await page.route('**/api/v1/repositories**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], totalCount: 0 }),
    })
  })
}

/**
 * Navigate directly to the deploy form for webapp-full-featured.
 * Goes to detail page first (like catalog_detail_test pattern), then clicks deploy.
 * The deploy modal is a 3-step wizard: Target -> Configure -> Review.
 * This helper fills in the Target step and advances to Configure.
 * Mocks MUST be set up before calling this function.
 */
async function navigateToDeployForm(page: import('@playwright/test').Page) {
  // Navigate directly to RGD detail page (skip catalog list entirely)
  await page.goto('/catalog/webapp-full-featured')

  // Wait for detail page to render (heading is a reliable indicator)
  await expect(page.locator('h1').first()).toBeVisible({ timeout: 10000 })

  // Click deploy button
  const deployButton = page.getByRole('button', { name: /deploy/i }).first()
  await expect(deployButton).toBeVisible({ timeout: 10000 })
  await deployButton.click()

  // Step 1: Target — fill instance name, select project & namespace
  await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
  await page.getByPlaceholder('my-instance').fill('test-deploy')

  // Project auto-selects when only one exists; select namespace
  const nsSelect = page.getByTestId('namespace-select')
  await expect(nsSelect).toBeEnabled({ timeout: 5000 })
  await nsSelect.click()
  await page.getByRole('option', { name: 'default' }).click()

  // Advance to Configure step
  await page.getByRole('button', { name: /continue/i }).click()
  await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })
}

/**
 * Toggle a per-feature enabled checkbox inside an ObjectField.
 * The checkbox is rendered as a child of the feature's collapsible section.
 */
async function toggleFeatureEnabled(
  page: import('@playwright/test').Page,
  featureName: string,
  expectEnabled: boolean,
) {
  // The feature ObjectField should be visible (parent of controlling field)
  const featureField = page.getByTestId(`field-${featureName}`)
  await expect(featureField).toBeVisible({ timeout: 5000 })

  // Expand the section if collapsed (ObjectFields start closed by default)
  const enabledCheckbox = featureField.locator(`input[name="${featureName}.enabled"]`)
  if (!(await enabledCheckbox.isVisible())) {
    await featureField.getByRole('button').first().click()
  }

  await expect(enabledCheckbox).toBeVisible({ timeout: 5000 })

  const isChecked = await enabledCheckbox.isChecked()
  if (isChecked !== expectEnabled) {
    await enabledCheckbox.click()
    // Wait for React state to propagate
    await page.waitForTimeout(300)
  }
}

test.describe('Per-Feature Conditional Field Visibility', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test.beforeEach(async ({ page }) => {
    await setupWebAppMocks(page)
    await navigateToDeployForm(page)
  })

  test('non-conditional fields are always visible regardless of toggle state', async ({
    page,
  }) => {
    // Fields used by the non-conditional "app" resource should always be visible
    await expect(page.getByTestId('field-name')).toBeVisible()
    await expect(page.getByTestId('field-image')).toBeVisible()
    await expect(page.getByTestId('field-port')).toBeVisible()
  })

  test('feature containers are always visible (parent of controlling field)', async ({
    page,
  }) => {
    // database and cache section wrappers are always visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-cache')).toBeVisible()

    // Expand sections to access enabled checkboxes (sections start collapsed)
    await page.getByTestId('field-database').getByRole('button').first().click()
    await page.getByTestId('field-cache').getByRole('button').first().click()

    await expect(page.getByTestId('field-database.enabled')).toBeVisible()
    await expect(page.getByTestId('field-cache.enabled')).toBeVisible()
  })

  test('feature peer fields are hidden when enabled is false', async ({ page }) => {
    // Both features default to enabled=false
    // Peer fields (database.name, cache.maxMemory) should be hidden
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()
    await expect(page.getByTestId('field-cache.maxMemory')).not.toBeVisible()
  })

  test('shared conditional field is hidden when ALL features are disabled (AND-based)', async ({
    page,
  }) => {
    // sharedLabel is in BOTH database and cache conditional sections
    // Both default false -> hidden (AND: all conditions unmet)
    await expect(page.getByTestId('field-sharedLabel')).not.toBeVisible()
  })

  test('enabling database shows database peer fields and sharedLabel', async ({ page }) => {
    await toggleFeatureEnabled(page, 'database', true)

    // database.name should now be visible
    await expect(page.getByTestId('field-database.name')).toBeVisible()

    // sharedLabel should now be visible (at least one controller is met)
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()
  })

  test('enabling cache shows cache peer fields and sharedLabel but NOT database peers', async ({
    page,
  }) => {
    await toggleFeatureEnabled(page, 'cache', true)

    // cache.maxMemory should now be visible
    await expect(page.getByTestId('field-cache.maxMemory')).toBeVisible()

    // sharedLabel should now be visible
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()

    // database.name should still be hidden (database.enabled is still false)
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()
  })

  test('enabling both features shows all fields', async ({ page }) => {
    await toggleFeatureEnabled(page, 'database', true)
    await toggleFeatureEnabled(page, 'cache', true)

    // All peer fields should be visible
    await expect(page.getByTestId('field-database.name')).toBeVisible()
    await expect(page.getByTestId('field-cache.maxMemory')).toBeVisible()
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()

    // Non-conditional fields should still be visible
    await expect(page.getByTestId('field-name')).toBeVisible()
  })

  test('disabling one feature keeps sharedLabel visible if other is still on', async ({
    page,
  }) => {
    // Enable both
    await toggleFeatureEnabled(page, 'database', true)
    await toggleFeatureEnabled(page, 'cache', true)

    // Verify all visible
    await expect(page.getByTestId('field-database.name')).toBeVisible()
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()

    // Disable database, keep cache on
    await toggleFeatureEnabled(page, 'database', false)

    // database peer fields hidden
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()

    // sharedLabel should STILL be visible (cache is still on)
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()
  })

  test('disabling both features hides all conditional fields', async ({ page }) => {
    // Enable both first
    await toggleFeatureEnabled(page, 'database', true)
    await toggleFeatureEnabled(page, 'cache', true)

    // Verify visible
    await expect(page.getByTestId('field-database.name')).toBeVisible()
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()

    // Disable both
    await toggleFeatureEnabled(page, 'database', false)
    await toggleFeatureEnabled(page, 'cache', false)

    // Peer fields hidden, sharedLabel hidden (AND: all unmet)
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()
    await expect(page.getByTestId('field-cache.maxMemory')).not.toBeVisible()
    await expect(page.getByTestId('field-sharedLabel')).not.toBeVisible()
  })

  test('toggle cycle: off -> on -> off preserves correct visibility', async ({ page }) => {
    // Start: both off
    await expect(page.getByTestId('field-sharedLabel')).not.toBeVisible()
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()

    // Enable database
    await toggleFeatureEnabled(page, 'database', true)
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()
    await expect(page.getByTestId('field-database.name')).toBeVisible()

    // Disable database
    await toggleFeatureEnabled(page, 'database', false)
    await expect(page.getByTestId('field-sharedLabel')).not.toBeVisible()
    await expect(page.getByTestId('field-database.name')).not.toBeVisible()

    // Enable cache
    await toggleFeatureEnabled(page, 'cache', true)
    await expect(page.getByTestId('field-sharedLabel')).toBeVisible()
    await expect(page.getByTestId('field-database.name')).not.toBeVisible() // still off

    // Disable cache
    await toggleFeatureEnabled(page, 'cache', false)
    await expect(page.getByTestId('field-sharedLabel')).not.toBeVisible()
  })
})
