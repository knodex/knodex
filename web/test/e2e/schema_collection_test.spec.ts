// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import {
  mockRGDs,
  mockMicroservicesPlatformRGD,
  mockMicroservicesPlatformSchema,
  API_PATHS,
} from '../fixture/mock-data'

/**
 * Schema Collection E2E Tests
 *
 * Tests the schema API and UI rendering across multiple RGDs:
 * - Schema endpoint returns correct structure (properties, conditionalSections, secretRefs)
 * - Catalog detail renders schema-derived information (Kind, fields, deployment form)
 * - Multiple RGDs each serve their own schema independently
 * - Degraded schema mode (rgd-only) is handled gracefully
 * - Schema with secretRefs are surfaced in the catalog detail
 *
 * These tests use mocked API responses to verify the frontend correctly
 * processes and renders schema data from the /api/v1/rgds/{name}/schema endpoint.
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots'

/** Schema response with full CRD+RGD source */
const mockPostgresSchema = {
  rgd: 'postgres-database',
  crdFound: true,
  source: 'crd+rgd' as const,
  secretRefs: [],
  schema: {
    group: 'databases.kro.run',
    version: 'v1alpha1',
    kind: 'PostgresDatabase',
    description: 'PostgreSQL database with automated backups and monitoring',
    properties: {
      replicas: {
        type: 'integer',
        description: 'Number of database replicas',
        default: 3,
      },
      storage: {
        type: 'string',
        description: 'Storage size (e.g., 100Gi)',
        default: '50Gi',
      },
      version: {
        type: 'string',
        description: 'PostgreSQL version',
        default: '16',
      },
      enableMonitoring: {
        type: 'boolean',
        description: 'Enable Prometheus monitoring',
        default: true,
      },
    },
    required: ['replicas', 'storage'],
  },
}

/** Schema response with degraded source (CRD not found) */
const mockDegradedSchema = {
  rgd: 'redis-cache',
  crdFound: false,
  source: 'rgd-only' as const,
  secretRefs: [],
  warnings: ['CRD not found — schema derived from RGD spec only; validation constraints may be missing'],
  schema: {
    group: 'caching.kro.run',
    version: 'v1alpha1',
    kind: 'RedisCache',
    description: 'Redis cache cluster for high-performance caching',
    properties: {
      replicas: {
        type: 'integer',
        description: 'Number of cache replicas',
      },
      memory: {
        type: 'string',
        description: 'Memory limit per replica',
      },
    },
    required: ['replicas'],
  },
}

/** Schema response with no schema (error case) */
const mockErrorSchema = {
  rgd: 'nginx-ingress',
  crdFound: false,
  source: undefined,
  secretRefs: [],
  error: 'RGD has no spec.schema block',
  schema: null,
}

/** Schema with conditional sections and secret refs */
const mockSchemaWithConditionals = {
  rgd: 'webapp-with-secret',
  crdFound: true,
  source: 'crd+rgd' as const,
  secretRefs: [
    {
      id: '0-Secret',
      type: 'provided',
      externalRefId: 'dbSecret',
      description: 'Database credentials secret',
    },
  ],
  schema: {
    group: 'kro.run',
    version: 'v1alpha1',
    kind: 'WebAppWithSecret',
    description: 'Web application with external secret',
    properties: {
      appName: {
        type: 'string',
        description: 'Application name',
      },
      useExternalDB: {
        type: 'boolean',
        description: 'Use external database',
        default: false,
      },
      externalRef: {
        type: 'object',
        properties: {
          dbSecret: {
            type: 'object',
            properties: {
              name: { type: 'string', default: '' },
              namespace: { type: 'string', default: '' },
            },
            externalRefSelector: {
              apiVersion: 'v1',
              kind: 'Secret',
              useInstanceNamespace: true,
              autoFillFields: { name: 'name', namespace: 'namespace' },
            },
          },
        },
      },
    },
    required: ['appName'],
    conditionalSections: [
      {
        controllingField: 'spec.useExternalDB',
        condition: '${schema.spec.useExternalDB == true}',
        expectedValue: true,
        affectedProperties: ['externalRef'],
      },
    ],
  },
}

/** Mock RGD with secret refs on the detail response */
const mockRGDWithSecretRefs = {
  name: 'webapp-with-secret',
  namespace: 'default',
  description: 'Web application with external secret',
  version: 'v1.0.0',
  tags: ['webapp', 'secrets'],
  category: 'examples',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 1,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'WebAppWithSecret',
  status: 'Active',
  secretRefs: [
    {
      id: '0-Secret',
      type: 'provided',
      externalRefId: 'dbSecret',
      description: 'Database credentials secret',
    },
  ],
  createdAt: '2025-01-22T10:00:00Z',
  updatedAt: '2025-01-22T10:00:00Z',
}

const MOCK_ACCOUNT_INFO = {
  userID: 'user-global-admin',
  email: 'admin@e2e-test.local',
  displayName: 'Global Administrator',
  groups: [],
  casbinRoles: ['role:serveradmin'],
  projects: ['proj-alpha-team'],
  roles: {},
  issuer: 'knodex',
  tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
  tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
}

function setupCommonRoutes(page: Page) {
  return Promise.all([
    // Mock account/info so session restore succeeds (prevents blank page / redirect to login)
    page.route('**/api/v1/account/info', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ACCOUNT_INFO),
      })
    }),
    page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      })
    }),
    page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          node: null,
          upstream: [],
          downstream: [],
          deploymentOrder: [],
          hasCycle: false,
        }),
      })
    }),
    page.route('**/api/v1/projects**', async (route) => {
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
    }),
    page.route('**/api/v1/repositories**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0 }),
      })
    }),
    page.route('**/api/v1/resources**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], count: 0 }),
      })
    }),
  ])
}

test.describe('Schema Collection — Per-RGD Schema Retrieval', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('schema endpoint returns correct structure with properties and required fields', async ({
    page,
  }) => {
    let capturedSchemaResponse: Record<string, unknown> | null = null

    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/postgres-database/schema') || url.includes('/postgres-database%2Fschema')) {
        capturedSchemaResponse = mockPostgresSchema
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockPostgresSchema),
        })
      } else if (url.includes('/validate-deployment')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ valid: true, errors: [] }),
        })
      } else if (url.includes('/postgres-database') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[0]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: mockRGDs, totalCount: mockRGDs.length, page: 1, pageSize: 10 }),
        })
      }
    })

    await setupCommonRoutes(page)

    // Navigate to the catalog detail page
    await page.goto('/catalog/postgres-database')
    await page.waitForLoadState('networkidle')

    // Open the deploy modal — this triggers the schema endpoint fetch via useRGDSchema
    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
    await deployButton.click()

    // Wait for the schema to be fetched (deploy modal initialization triggers useRGDSchema)
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })

    // Verify schema was fetched with correct structure
    expect(capturedSchemaResponse).toBeDefined()
    expect(capturedSchemaResponse!.crdFound).toBe(true)
    expect(capturedSchemaResponse!.source).toBe('crd+rgd')
    expect(capturedSchemaResponse!.schema).toBeDefined()

    const schema = capturedSchemaResponse!.schema as Record<string, unknown>
    expect(schema.kind).toBe('PostgresDatabase')
    expect(schema.properties).toBeDefined()
    expect(schema.required).toEqual(['replicas', 'storage'])
    expect(capturedSchemaResponse!.secretRefs).toEqual([])
  })

  test('catalog detail renders RGD metadata from schema', async ({ page }) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/postgres-database/schema') || url.includes('/postgres-database%2Fschema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockPostgresSchema),
        })
      } else if (url.includes('/postgres-database') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[0]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: mockRGDs, totalCount: mockRGDs.length, page: 1, pageSize: 10 }),
        })
      }
    })

    await setupCommonRoutes(page)

    await page.goto('/catalog/postgres-database')
    await page.waitForLoadState('networkidle')

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-01-detail.png`,
      fullPage: true,
    })

    // RGD name/title should be visible
    const main = page.locator('main')
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })

    // Description should render
    await expect(main.locator('text=PostgreSQL database')).toBeVisible({ timeout: 5000 })
  })

  test('degraded schema (rgd-only) renders without errors', async ({ page }) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/redis-cache/schema') || url.includes('/redis-cache%2Fschema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockDegradedSchema),
        })
      } else if (url.includes('/redis-cache') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[1]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: mockRGDs, totalCount: mockRGDs.length, page: 1, pageSize: 10 }),
        })
      }
    })

    await setupCommonRoutes(page)

    await page.goto('/catalog/redis-cache')
    await page.waitForLoadState('networkidle')

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-02-degraded.png`,
      fullPage: true,
    })

    // Page should render without crash
    const main = page.locator('main')
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })

    // Should still show description
    await expect(main.locator('text=Redis cache')).toBeVisible({ timeout: 5000 })
  })

  test('schema error (no spec.schema) renders gracefully', async ({ page }) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/nginx-ingress/schema') || url.includes('/nginx-ingress%2Fschema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockErrorSchema),
        })
      } else if (url.includes('/nginx-ingress') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[2]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: mockRGDs, totalCount: mockRGDs.length, page: 1, pageSize: 10 }),
        })
      }
    })

    await setupCommonRoutes(page)

    await page.goto('/catalog/nginx-ingress')
    await page.waitForLoadState('networkidle')

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-03-error.png`,
      fullPage: true,
    })

    // Page should render without crash
    const main = page.locator('main')
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })
  })
})

test.describe('Schema Collection — Conditional Sections and SecretRefs', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('schema with conditionalSections drives field visibility in deploy form', async ({
    page,
  }) => {
    // Use the microservices-platform schema which has conditionalSections
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/microservices-platform')) {
        if (url.includes('/schema')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(mockMicroservicesPlatformSchema),
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
            body: JSON.stringify(mockMicroservicesPlatformRGD),
          })
        }
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [mockMicroservicesPlatformRGD],
            totalCount: 1,
            page: 1,
            pageSize: 10,
          }),
        })
      }
    })

    await setupCommonRoutes(page)

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

    await page.route('**/api/v1/repositories**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], totalCount: 0 }),
      })
    })

    await page.route('**/api/v1/resources**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], count: 0 }),
      })
    })

    // Navigate to catalog and deploy
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')

    const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
    await expect(rgdCard).toBeVisible({ timeout: 15000 })
    await rgdCard.click()

    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 15000 })
    await deployButton.click()

    // Step 1: Target — fill instance name, select project & namespace
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
    await page.getByPlaceholder('my-instance').fill('test-deploy')
    const nsSelect = page.getByTestId('namespace-select')
    await expect(nsSelect).toBeEnabled({ timeout: 5000 })
    await nsSelect.click()
    await page.getByRole('option', { name: 'default' }).click()
    await page.getByRole('button', { name: /continue/i }).click()
    await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })

    // externalRef should be hidden by default (conditionalSection controls it)
    await expect(page.getByTestId('field-externalRef')).not.toBeVisible()

    // Enable the controlling field
    const checkbox = page.getByTestId('input-useExistingDatabase')
    await checkbox.check()

    // Now the conditional field should appear
    await expect(page.getByTestId('field-externalRef')).toBeVisible({ timeout: 5000 })

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-04-conditional.png`,
      fullPage: true,
    })
  })

  test('schema with secretRefs shows Secrets tab in catalog detail', async ({ page }) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/webapp-with-secret')) {
        if (url.includes('/schema')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(mockSchemaWithConditionals),
          })
        } else if (!url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(mockRGDWithSecretRefs),
          })
        } else {
          await route.continue()
        }
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [mockRGDWithSecretRefs],
            totalCount: 1,
            page: 1,
            pageSize: 10,
          }),
        })
      }
    })

    await setupCommonRoutes(page)

    await page.goto('/catalog/webapp-with-secret')
    await page.waitForLoadState('networkidle')

    // Secrets tab should be visible (RGD has secretRefs)
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")')
    await expect(secretsTab).toBeVisible({ timeout: 5000 })

    // Count badge should show 1
    await expect(secretsTab).toContainText('1')

    await secretsTab.click()

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-05-secrets-tab.png`,
      fullPage: true,
    })

    // Required Secrets header should be visible
    await expect(page.locator('text=Required Secrets')).toBeVisible({ timeout: 10000 })

    // Secret ref card should render
    const card = page.locator('[data-testid="catalog-secret-ref-0-Secret"]')
    await expect(card).toBeVisible({ timeout: 5000 })
    await expect(card.locator('text=user-provided')).toBeVisible()
    await expect(card.locator('text=Database credentials secret')).toBeVisible()
  })

  test('multiple RGDs serve independent schemas', async ({ page }) => {
    const schemaRequests: string[] = []

    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()

      // Track schema requests
      if (url.includes('/schema')) {
        const match = url.match(/\/rgds\/([^/]+)\/schema/)
        if (match) schemaRequests.push(decodeURIComponent(match[1]))
      }

      if (url.includes('/postgres-database/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockPostgresSchema),
        })
      } else if (url.includes('/redis-cache/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockDegradedSchema),
        })
      } else if (url.includes('/validate-deployment')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ valid: true, errors: [] }),
        })
      } else if (url.includes('/postgres-database') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[0]),
        })
      } else if (url.includes('/redis-cache') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[1]),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: mockRGDs, totalCount: mockRGDs.length, page: 1, pageSize: 10 }),
        })
      }
    })

    await setupCommonRoutes(page)

    const main = page.locator('main')

    // Visit first RGD and open deploy modal to trigger schema fetch
    await page.goto('/catalog/postgres-database')
    await page.waitForLoadState('networkidle')
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })

    const deployBtn1 = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn1).toBeVisible({ timeout: 10000 })
    await deployBtn1.click()
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })

    // Close modal and navigate to second RGD
    await page.keyboard.press('Escape')
    await page.waitForTimeout(300)

    await page.goto('/catalog/redis-cache')
    await page.waitForLoadState('networkidle')
    await expect(main.locator('h1, h2, h3').first()).toBeVisible({ timeout: 10000 })

    const deployBtn2 = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn2).toBeVisible({ timeout: 10000 })
    await deployBtn2.click()
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/schema-collection-06-multi-rgd.png`,
      fullPage: true,
    })

    // Both schemas should have been requested independently
    expect(schemaRequests).toContain('postgres-database')
    expect(schemaRequests).toContain('redis-cache')
    expect(schemaRequests.length).toBeGreaterThanOrEqual(2)
  })
})
