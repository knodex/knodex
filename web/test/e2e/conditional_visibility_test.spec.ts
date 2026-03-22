// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole, TEST_USERS, setupPermissionMocking } from '../fixture'
import { API_PATHS } from '../fixture/mock-data'
import { toggleConditionalField, assertFieldPositioning } from '../fixture/conditional-fields-helpers'

/**
 * Mock RGD for multi-controller conditional visibility testing.
 * Mirrors the webapp-full-featured RGD:
 *   - enableDatabase + enableCache are independent boolean toggles
 *   - hiddenAnnotation is used by BOTH conditional resources (AND-based hiding)
 *   - database object is used ONLY by the database conditional resource
 *   - name, image, port, visibleAnnotation are used by non-conditional resource (always visible)
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
      visibleAnnotation: {
        type: 'string',
        description: 'Annotation used by all resources (always visible)',
        default: '',
      },
      hiddenAnnotation: {
        type: 'string',
        description: 'Annotation used ONLY by conditional resources (hidden when both toggles off)',
        default: '',
      },
      enableDatabase: {
        type: 'boolean',
        description: 'Enable PostgreSQL database',
        default: false,
      },
      enableCache: {
        type: 'boolean',
        description: 'Enable Redis cache',
        default: false,
      },
      database: {
        type: 'object',
        description: 'Database configuration',
        properties: {
          name: {
            type: 'string',
            description: 'Database name',
            default: 'mydb',
          },
        },
      },
    },
    required: ['name'],
    conditionalSections: [
      {
        condition: 'schema.spec.enableDatabase == true',
        controllingField: 'enableDatabase',
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: 'spec.enableDatabase', op: '==', value: true }],
        affectedProperties: ['database', 'hiddenAnnotation'],
      },
      {
        condition: 'schema.spec.enableCache == true',
        controllingField: 'enableCache',
        expectedValue: true,
        clientEvaluable: true,
        rules: [{ field: 'spec.enableCache', op: '==', value: true }],
        affectedProperties: ['hiddenAnnotation'],
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
 * Mocks MUST be set up before calling this function.
 */
async function navigateToDeployForm(page: import('@playwright/test').Page) {
  // Navigate directly to RGD detail page (skip catalog list entirely)
  await page.goto('/catalog/webapp-full-featured')

  // Wait for detail page to render (back button is a reliable indicator)
  await expect(page.getByRole('button', { name: /back/i })).toBeVisible({ timeout: 10000 })

  // Click deploy button
  const deployButton = page.getByRole('button', { name: /deploy/i })
  await expect(deployButton).toBeVisible({ timeout: 10000 })
  await deployButton.click()

  // Wait for the deploy form to render
  await expect(page.getByText('Configuration')).toBeVisible({ timeout: 15000 })
}

test.describe('Multi-Controller Conditional Field Visibility (AND-based hiding)', () => {
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
    await expect(page.getByTestId('field-visibleAnnotation')).toBeVisible()

    // Controller fields themselves should always be visible
    await expect(page.getByTestId('field-enableDatabase')).toBeVisible()
    await expect(page.getByTestId('field-enableCache')).toBeVisible()
  })

  test('exclusive conditional fields are hidden when their controller is off', async ({
    page,
  }) => {
    // database is ONLY in enableDatabase's affectedProperties
    // Both toggles default to false, so database should be hidden
    await expect(page.getByTestId('field-database')).not.toBeVisible()
  })

  test('shared conditional field is hidden when ALL controllers are off (AND-based)', async ({
    page,
  }) => {
    // hiddenAnnotation is in BOTH enableDatabase and enableCache sections
    // Both default false -> hidden (AND: all conditions unmet)
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
  })

  test('enabling enableDatabase shows database and hiddenAnnotation', async ({ page }) => {
    // Use the shared helper for deterministic toggle + wait
    await toggleConditionalField(page, 'enableDatabase', 'database', true)

    // hiddenAnnotation should now be visible (at least one controller is met)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()
  })

  test('enabling enableCache shows hiddenAnnotation but NOT database', async ({ page }) => {
    // Use the shared helper for deterministic toggle + wait
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)

    // database should still be hidden (enableDatabase is still false)
    await expect(page.getByTestId('field-database')).not.toBeVisible()
  })

  test('enabling both controllers shows all conditional fields', async ({ page }) => {
    await toggleConditionalField(page, 'enableDatabase', 'database', true)
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)

    // Both conditional fields should be visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Non-conditional fields should still be visible
    await expect(page.getByTestId('field-name')).toBeVisible()
    await expect(page.getByTestId('field-visibleAnnotation')).toBeVisible()
  })

  test('disabling one controller keeps shared field visible if other is still on', async ({
    page,
  }) => {
    // Enable both
    await toggleConditionalField(page, 'enableDatabase', 'database', true)
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)

    // Verify both conditional fields visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable enableDatabase, keep enableCache on
    await toggleConditionalField(page, 'enableDatabase', 'database', false)

    // hiddenAnnotation should STILL be visible (enableCache is still on)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()
  })

  test('disabling both controllers hides all conditional fields', async ({ page }) => {
    // Enable both first
    await toggleConditionalField(page, 'enableDatabase', 'database', true)
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)

    // Verify visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable both
    await toggleConditionalField(page, 'enableDatabase', 'database', false)
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', false)

    // Both should be hidden
    await expect(page.getByTestId('field-database')).not.toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
  })

  test('controlled fields appear below their last controller in form order', async ({
    page,
  }) => {
    // Enable both toggles using helpers
    await toggleConditionalField(page, 'enableDatabase', 'database', true)
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)

    // Use shared helper for position verification
    const enableDbField = page.getByTestId('field-enableDatabase')
    const enableCacheField = page.getByTestId('field-enableCache')
    const databaseField = page.getByTestId('field-database')
    const hiddenAnnotationField = page.getByTestId('field-hiddenAnnotation')

    // database (exclusive to enableDatabase) should appear after enableDatabase
    const dbDistance = await assertFieldPositioning(enableDbField, databaseField)
    expect(dbDistance).toBeGreaterThanOrEqual(0)

    // hiddenAnnotation (shared) should appear after its LAST controller
    const enableDbBox = await enableDbField.boundingBox()
    const enableCacheBox = await enableCacheField.boundingBox()
    const hiddenAnnotationBox = await hiddenAnnotationField.boundingBox()

    expect(enableDbBox).toBeTruthy()
    expect(enableCacheBox).toBeTruthy()
    expect(hiddenAnnotationBox).toBeTruthy()

    const lastControllerY = Math.max(enableDbBox!.y, enableCacheBox!.y)
    expect(hiddenAnnotationBox!.y).toBeGreaterThan(lastControllerY)
  })

  test('toggle cycle: off -> on -> off preserves correct visibility', async ({ page }) => {
    // Start: both off
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
    await expect(page.getByTestId('field-database')).not.toBeVisible()

    // Enable database
    await toggleConditionalField(page, 'enableDatabase', 'database', true)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable database
    await toggleConditionalField(page, 'enableDatabase', 'database', false)
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()

    // Enable cache
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', true)
    await expect(page.getByTestId('field-database')).not.toBeVisible() // still off

    // Disable cache
    await toggleConditionalField(page, 'enableCache', 'hiddenAnnotation', false)
  })
})
