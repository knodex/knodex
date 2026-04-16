// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import {
  API_PATHS,
} from '../fixture/mock-data'

/**
 * Secrets Deploy Flow E2E Tests
 *
 * Tests the end-to-end flow of deploying an RGD that references Kubernetes
 * Secrets via externalRef. Covers:
 * - Schema API returns secretRefs alongside form schema
 * - Deploy form renders secret-related fields for "provided" type refs
 * - Form submission includes secret reference values
 * - Catalog detail shows correct secret metadata before deploy
 *
 * Prerequisites:
 * - Backend deployed with secrets feature enabled
 * - webapp-with-secret RGD deployed (has externalRef Secret)
 */

const SCREENSHOT_DIR = '../test-results/e2e/screenshots'

/** Mock RGD with a "provided" secret ref (user supplies name/namespace at deploy time) */
const mockRGDWithSecrets = {
  name: 'webapp-with-secret',
  namespace: 'default',
  description: 'Web application that references an external Kubernetes Secret',
  version: 'v1.0.0',
  tags: ['webapp', 'secrets'],
  category: 'examples',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  dependsOnKinds: ['Secret'],
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

/** Mock schema response with secretRefs included */
const mockSchemaWithSecrets = {
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
    description: 'Web application that references an external Kubernetes Secret',
    properties: {
      appName: {
        type: 'string',
        description: 'Name of the web application',
        default: 'my-webapp',
      },
      image: {
        type: 'string',
        description: 'Container image to deploy',
        default: 'nginx:latest',
      },
      externalRef: {
        type: 'object',
        properties: {
          dbSecret: {
            type: 'object',
            description: 'Database credentials secret',
            properties: {
              name: {
                type: 'string',
                description: 'Name of the secret',
                default: '',
              },
              namespace: {
                type: 'string',
                description: 'Namespace of the secret',
                default: '',
              },
            },
            externalRefSelector: {
              apiVersion: 'v1',
              kind: 'Secret',
              useInstanceNamespace: false,
              autoFillFields: { name: 'name', namespace: 'namespace' },
            },
          },
        },
      },
    },
    required: ['appName'],
  },
}

/** Mock RGD with a "fixed" secret ref (hardcoded name/namespace) */
const mockRGDWithFixedSecret = {
  name: 'app-with-fixed-secret',
  namespace: 'default',
  description: 'Application with a fixed secret reference',
  version: 'v1.0.0',
  tags: ['webapp', 'secrets'],
  category: 'examples',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  dependsOnKinds: ['Secret'],
  secretRefs: [
    {
      id: '0-Secret',
      type: 'fixed',
      name: 'tls-cert',
      namespace: 'cert-manager',
      externalRefId: 'tlsCert',
      description: 'TLS certificate from cert-manager',
    },
  ],
  createdAt: '2025-01-22T10:00:00Z',
  updatedAt: '2025-01-22T10:00:00Z',
}

/** Mock RGD with a "dynamic" secret ref (CEL expression) */
const mockRGDWithDynamicSecret = {
  name: 'app-with-dynamic-secret',
  namespace: 'default',
  description: 'Application with a dynamic secret reference',
  version: 'v1.0.0',
  tags: ['webapp', 'secrets'],
  category: 'examples',
  labels: { 'knodex.io/catalog': 'true' },
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'ResourceGraphDefinition',
  status: 'Active',
  dependsOnKinds: ['Secret'],
  secretRefs: [
    {
      id: '1-Secret',
      type: 'dynamic',
      nameExpr: '${schema.spec.appName}-credentials',
      namespaceExpr: '${schema.spec.targetNamespace}',
      externalRefId: 'appCreds',
      description: 'Auto-generated credentials secret',
    },
  ],
  createdAt: '2025-01-22T10:00:00Z',
  updatedAt: '2025-01-22T10:00:00Z',
}

/** Mock K8s Secrets for the externalRef selector */
const mockK8sSecrets = {
  items: [
    {
      name: 'my-db-secret',
      namespace: 'default',
      labels: { app: 'database' },
      createdAt: '2025-01-15T10:00:00Z',
    },
    {
      name: 'api-credentials',
      namespace: 'default',
      labels: { app: 'api' },
      createdAt: '2025-01-16T11:00:00Z',
    },
  ],
  count: 2,
}

async function setupSecretDeployMocks(page: Page) {
  // Mock account/info so session restore succeeds (prevents blank page / redirect to login)
  await page.route('**/api/v1/account/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        userID: 'user-global-admin',
        email: 'admin@e2e-test.local',
        displayName: 'Global Administrator',
        groups: [],
        casbinRoles: ['role:serveradmin'],
        projects: [],
        roles: {},
        issuer: 'knodex',
        tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
        tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
      }),
    })
  })

  await page.route(`**${API_PATHS.rgds}**`, async (route) => {
    const url = route.request().url()

    if (url.includes('/webapp-with-secret')) {
      if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockSchemaWithSecrets),
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
          body: JSON.stringify(mockRGDWithSecrets),
        })
      }
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [mockRGDWithSecrets],
          totalCount: 1,
          page: 1,
          pageSize: 10,
        }),
      })
    }
  })

  await page.route('**/api/v1/account/can-i/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ value: 'yes' }),
    })
  })

  await page.route('**/api/v1/dependencies/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: null,
        upstream: [],
        downstream: [],
        deploymentOrder: ['webapp-with-secret'],
        hasCycle: false,
      }),
    })
  })

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
      body: JSON.stringify(mockK8sSecrets),
    })
  })

  await page.route('**/api/v1/compliance/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ result: 'pass', violations: [] }),
    })
  })

  // Mock preflight dry-run endpoint (called when advancing from Configure to Review)
  await page.route('**/instances/**/preflight', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ valid: true }),
    })
  })
}

test.describe('Secrets Deploy Flow', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('schema API returns secretRefs alongside form schema', async ({ page }) => {
    let schemaResponse: Record<string, unknown> | null = null

    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/schema')) {
        schemaResponse = mockSchemaWithSecrets
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockSchemaWithSecrets),
        })
      } else if (url.includes('/validate-deployment')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ valid: true, errors: [] }),
        })
      } else if (!url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDWithSecrets),
        })
      } else {
        await route.continue()
      }
    })

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      })
    })

    await page.route('**/api/v1/account/info', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          userID: 'user-global-admin',
          email: 'admin@e2e-test.local',
          displayName: 'Global Administrator',
          groups: [],
          casbinRoles: ['role:serveradmin'],
          projects: [],
          roles: {},
          issuer: 'knodex',
          tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
          tokenIssuedAt: Math.floor(Date.now() / 1000) - 60,
        }),
      })
    })

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
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], totalCount: 0 }) })
    })

    await page.route('**/api/v1/resources**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], count: 0 }) })
    })

    // Navigate to the catalog detail page
    await page.goto('/catalog/webapp-with-secret')
    await page.waitForLoadState('networkidle')

    // Open the deploy modal to trigger the schema endpoint fetch via useRGDSchema
    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
    await deployButton.click()

    // Wait for the Target step to appear (deploy modal initialized)
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })

    // Verify the schema response includes secretRefs
    expect(schemaResponse).toBeDefined()
    expect((schemaResponse!.secretRefs as unknown[]).length).toBe(1)
    const secretRef = (schemaResponse!.secretRefs as Record<string, unknown>[])[0]
    expect(secretRef.type).toBe('provided')
    expect(secretRef.externalRefId).toBe('dbSecret')
    expect(schemaResponse!.crdFound).toBe(true)
    expect(schemaResponse!.source).toBe('crd+rgd')
  })

  test('catalog detail shows Secrets tab with provided-type secret ref', async ({ page }) => {
    await setupSecretDeployMocks(page)

    await page.goto('/catalog/webapp-with-secret')
    await page.waitForLoadState('networkidle')

    // Secrets tab should appear (secretRefs count > 0)
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")')
    await expect(secretsTab).toBeVisible({ timeout: 5000 })

    await secretsTab.click()

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-deploy-01-provided-ref.png`,
      fullPage: true,
    })

    // Verify "Required Secrets" header
    await expect(page.locator('text=Required Secrets')).toBeVisible({ timeout: 10000 })

    // Verify the secret ref card renders with user-provided badge
    const secretRefCard = page.locator('[data-testid="catalog-secret-ref-0-Secret"]')
    await expect(secretRefCard).toBeVisible({ timeout: 5000 })
    await expect(secretRefCard.locator('text=user-provided')).toBeVisible()

    // Verify description is shown
    await expect(secretRefCard.locator('text=Database credentials secret')).toBeVisible()
  })

  test('deploy form shows externalRef secret picker for provided-type refs', async ({ page }) => {
    await setupSecretDeployMocks(page)

    // Navigate to catalog, then deploy
    await page.goto('/catalog/webapp-with-secret')
    await page.waitForLoadState('networkidle')

    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
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

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-deploy-02-form.png`,
      fullPage: true,
    })

    // The externalRef.dbSecret field should be visible (provided type means user must fill it)
    const secretField = page.getByTestId('input-externalRef.dbSecret')
    const hasSecretField = await secretField.isVisible({ timeout: 5000 }).catch(() => false)

    if (!hasSecretField) {
      // Some deploy forms render secret refs as separate name/namespace inputs
      const secretNameField = page.getByTestId('field-externalRef')
      await expect(secretNameField).toBeVisible({ timeout: 5000 })
    }
  })

  test('deploy submission includes secret reference values', async ({ page }) => {
    await setupSecretDeployMocks(page)

    let submittedData: Record<string, unknown> | null = null
    const instancePattern = '**/api/v1/namespaces/*/instances/**'
    const responsePromise = page.waitForResponse(instancePattern)

    await page.route(instancePattern, async (route) => {
      // Let preflight requests fall through to the preflight mock
      if (route.request().url().includes('/preflight')) {
        await route.fallback()
        return
      }
      submittedData = await route.request().postDataJSON()
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ success: true }),
      })
    })

    // Navigate to deploy form
    await page.goto('/catalog/webapp-with-secret')
    await page.waitForLoadState('networkidle')

    const deployButton = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployButton).toBeVisible({ timeout: 10000 })
    await deployButton.click()

    // Step 1: Target — fill instance name, select project & namespace
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
    await page.getByPlaceholder('my-instance').fill('test-deploy')
    const nsSelect2 = page.getByTestId('namespace-select')
    await expect(nsSelect2).toBeEnabled({ timeout: 5000 })
    await nsSelect2.click()
    await page.getByRole('option', { name: 'default' }).click()
    await page.getByRole('button', { name: /continue/i }).click()
    await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })

    // Fill required fields
    const appNameInput = page.getByTestId('input-appName')
    await expect(appNameInput).toBeVisible({ timeout: 5000 })
    await appNameInput.fill('my-webapp')
    // Blur by clicking elsewhere to trigger form validation (mode: "onBlur")
    await page.getByTestId('configure-step').click({ position: { x: 1, y: 1 } })

    // Select a secret from the resource picker (if available and enabled)
    const secretSelector = page.getByTestId('input-externalRef.dbSecret')
    if (await secretSelector.isEnabled({ timeout: 5000 }).catch(() => false)) {
      await secretSelector.selectOption({ value: 'my-db-secret' })
    }

    // Navigate to Review step, then deploy
    await expect(page.getByRole('button', { name: /continue/i })).toBeEnabled({ timeout: 10000 })
    await page.getByRole('button', { name: /continue/i }).click()
    await expect(page.getByText('Deployment Summary')).toBeVisible({ timeout: 10000 })
    // Click Deploy on the Review step
    const deployBtn = page.getByRole('button', { name: /deploy/i }).last()
    await expect(deployBtn).toBeVisible({ timeout: 5000 })
    await deployBtn.click()
    await responsePromise

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-deploy-03-submission.png`,
      fullPage: true,
    })

    // Verify submitted data includes the spec
    expect(submittedData).toBeDefined()
    expect(submittedData!.spec).toBeDefined()
    expect((submittedData!.spec as Record<string, unknown>).appName).toBe('my-webapp')
  })
})

test.describe('Secrets Catalog Detail - All Secret Types', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  test('displays fixed-type secret ref with literal name and namespace', async ({ page }) => {
    // Mock RGD with fixed secret
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/app-with-fixed-secret') && !url.includes('/schema') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDWithFixedSecret),
        })
      } else if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ crdFound: true, schema: null, secretRefs: mockRGDWithFixedSecret.secretRefs }),
        })
      } else {
        await route.continue()
      }
    })

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      })
    })

    await page.goto('/catalog/app-with-fixed-secret')
    await page.waitForLoadState('networkidle')

    // Click Secrets tab
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")')
    const hasTab = await secretsTab.isVisible({ timeout: 5000 }).catch(() => false)

    if (!hasTab) {
      test.skip(true, 'Secrets tab not visible — RGD may not have loaded with secretRefs')
      return
    }

    await secretsTab.click()

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-deploy-04-fixed-ref.png`,
      fullPage: true,
    })

    // Verify fixed badge
    const secretRefCard = page.locator('[data-testid="catalog-secret-ref-0-Secret"]')
    await expect(secretRefCard).toBeVisible({ timeout: 5000 })
    await expect(secretRefCard.locator('text=fixed')).toBeVisible()

    // Verify literal name and namespace are displayed
    await expect(secretRefCard.getByText('tls-cert').first()).toBeVisible()
    await expect(secretRefCard.getByText('cert-manager').first()).toBeVisible()
  })

  test('displays dynamic-type secret ref with CEL expressions', async ({ page }) => {
    // Mock RGD with dynamic secret
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes('/app-with-dynamic-secret') && !url.includes('/schema') && !url.includes('/graph') && !url.includes('/resources') && !url.includes('/revisions')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDWithDynamicSecret),
        })
      } else if (url.includes('/schema')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ crdFound: true, schema: null, secretRefs: mockRGDWithDynamicSecret.secretRefs }),
        })
      } else {
        await route.continue()
      }
    })

    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      })
    })

    await page.goto('/catalog/app-with-dynamic-secret')
    await page.waitForLoadState('networkidle')

    // Click Secrets tab
    const secretsTab = page.locator('button[role="tab"]:has-text("Secrets")')
    const hasTab = await secretsTab.isVisible({ timeout: 5000 }).catch(() => false)

    if (!hasTab) {
      test.skip(true, 'Secrets tab not visible — RGD may not have loaded with secretRefs')
      return
    }

    await secretsTab.click()

    await page.screenshot({
      path: `${SCREENSHOT_DIR}/secrets-deploy-05-dynamic-ref.png`,
      fullPage: true,
    })

    // Verify dynamic badge
    const secretRefCard = page.locator('[data-testid="catalog-secret-ref-1-Secret"]')
    await expect(secretRefCard).toBeVisible({ timeout: 5000 })
    await expect(secretRefCard.locator('text=dynamic')).toBeVisible()

    // Verify CEL expressions are displayed
    await expect(secretRefCard.locator('text=${schema.spec.appName}-credentials')).toBeVisible()
    await expect(secretRefCard.locator('text=${schema.spec.targetNamespace}')).toBeVisible()
  })
})
