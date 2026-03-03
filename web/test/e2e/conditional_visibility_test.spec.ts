import { test, expect, TestUserRole } from '../fixture'
import { API_PATHS } from '../fixture/mock-data'

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
 * Helper: set up all API mocks for the webapp-full-featured RGD
 */
async function setupWebAppMocks(page: import('@playwright/test').Page) {
  // Mock RGD endpoints
  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    const url = route.request().url()

    if (url.includes('/webapp-full-featured')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockWebAppSchema),
        })
      } else if (url.includes('/validate-deployment')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ valid: true, errors: [] }),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockWebAppRGD),
        })
      }
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [mockWebAppRGD],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        }),
      })
    }
  })

  // Mock permissions
  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'yes' }),
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
}

/**
 * Navigate from catalog to deploy form
 */
async function navigateToDeployForm(page: import('@playwright/test').Page) {
  await page.goto('/catalog')
  await page.waitForLoadState('networkidle')

  const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
  await expect(rgdCard).toBeVisible({ timeout: 15000 })
  await rgdCard.click()

  await page.waitForURL(/\/catalog\//, { timeout: 10000 })
  await page.waitForLoadState('networkidle')

  const deployButton = page.getByRole('button', { name: /deploy/i })
  await expect(deployButton).toBeVisible({ timeout: 15000 })
  await deployButton.click()

  await page.waitForLoadState('networkidle')
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
    // Both default false → hidden (AND: all conditions unmet)
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
  })

  test('enabling enableDatabase shows database and hiddenAnnotation', async ({ page }) => {
    // Check enableDatabase toggle
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    await dbCheckbox.check()

    // database should now be visible (exclusive to enableDatabase)
    await expect(page.getByTestId('field-database')).toBeVisible()

    // hiddenAnnotation should now be visible (at least one controller is met)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()
  })

  test('enabling enableCache shows hiddenAnnotation but NOT database', async ({ page }) => {
    // Check enableCache toggle
    const cacheCheckbox = page.getByTestId('input-enableCache')
    await cacheCheckbox.check()

    // hiddenAnnotation should be visible (enableCache condition is met)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // database should still be hidden (enableDatabase is still false)
    await expect(page.getByTestId('field-database')).not.toBeVisible()
  })

  test('enabling both controllers shows all conditional fields', async ({ page }) => {
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    const cacheCheckbox = page.getByTestId('input-enableCache')

    await dbCheckbox.check()
    await cacheCheckbox.check()

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
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    const cacheCheckbox = page.getByTestId('input-enableCache')
    await dbCheckbox.check()
    await cacheCheckbox.check()

    // Verify both conditional fields visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable enableDatabase, keep enableCache on
    await dbCheckbox.uncheck()

    // database should be hidden (only controlled by enableDatabase)
    await expect(page.getByTestId('field-database')).not.toBeVisible()

    // hiddenAnnotation should STILL be visible (enableCache is still on)
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()
  })

  test('disabling both controllers hides all conditional fields', async ({ page }) => {
    // Enable both first
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    const cacheCheckbox = page.getByTestId('input-enableCache')
    await dbCheckbox.check()
    await cacheCheckbox.check()

    // Verify visible
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable both
    await dbCheckbox.uncheck()
    await cacheCheckbox.uncheck()

    // Both should be hidden
    await expect(page.getByTestId('field-database')).not.toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
  })

  test('controlled fields appear below their last controller in form order', async ({
    page,
  }) => {
    // Enable both toggles
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    const cacheCheckbox = page.getByTestId('input-enableCache')
    await dbCheckbox.check()
    await cacheCheckbox.check()

    // Wait for conditional fields to appear
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()
    await expect(page.getByTestId('field-database')).toBeVisible()

    // Get bounding boxes for position verification
    const enableDbBox = await page.getByTestId('field-enableDatabase').boundingBox()
    const enableCacheBox = await page.getByTestId('field-enableCache').boundingBox()
    const databaseBox = await page.getByTestId('field-database').boundingBox()
    const hiddenAnnotationBox = await page.getByTestId('field-hiddenAnnotation').boundingBox()

    expect(enableDbBox).toBeTruthy()
    expect(enableCacheBox).toBeTruthy()
    expect(databaseBox).toBeTruthy()
    expect(hiddenAnnotationBox).toBeTruthy()

    // database (exclusive to enableDatabase) should appear after enableDatabase
    expect(databaseBox!.y).toBeGreaterThan(enableDbBox!.y)

    // hiddenAnnotation (shared) should appear after its LAST controller
    // The last controller in form order determines positioning
    const lastControllerY = Math.max(enableDbBox!.y, enableCacheBox!.y)
    expect(hiddenAnnotationBox!.y).toBeGreaterThan(lastControllerY)
  })

  test('toggle cycle: off → on → off preserves correct visibility', async ({ page }) => {
    // Start: both off
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
    await expect(page.getByTestId('field-database')).not.toBeVisible()

    // Enable database
    const dbCheckbox = page.getByTestId('input-enableDatabase')
    await dbCheckbox.check()
    await expect(page.getByTestId('field-database')).toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible()

    // Disable database
    await dbCheckbox.uncheck()
    await expect(page.getByTestId('field-database')).not.toBeVisible()
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()

    // Enable cache
    const cacheCheckbox = page.getByTestId('input-enableCache')
    await cacheCheckbox.check()
    await expect(page.getByTestId('field-database')).not.toBeVisible() // still off
    await expect(page.getByTestId('field-hiddenAnnotation')).toBeVisible() // cache makes it visible

    // Disable cache
    await cacheCheckbox.uncheck()
    await expect(page.getByTestId('field-hiddenAnnotation')).not.toBeVisible()
  })
})
