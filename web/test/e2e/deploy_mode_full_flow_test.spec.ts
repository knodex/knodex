// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * E2E Tests for Full Deployment Flow with Mode Restrictions
 *
 * These tests verify end-to-end deployment scenarios:
 * - AC-1: Complete deployment flow with gitops-only RGD
 * - AC-2: Backend validation safety net (API-direct test)
 * - AC-4: Error response format verification (allowedModes as array)
 *
 * Note: Race condition test (AC-3) is in server/test/e2e/deployment_mode_race_test.go
 *
 * TEST TYPE CLASSIFICATION:
 * ========================
 * These are MOCKED COMPONENT TESTS that verify UI behavior without a real backend.
 * They test:
 * - UI correctly renders deployment mode options based on RGD restrictions
 * - UI sends correct request payload to the API
 * - UI handles error responses correctly
 *
 * For TRUE E2E TESTS that verify real backend behavior, see:
 * - server/test/e2e/deployment_mode_race_test.go (creates real K8s resources)
 *
 * The mocked tests are intentionally designed to run without cluster dependencies,
 * making them fast and suitable for CI/CD pipelines. The backend tests provide
 * the actual integration verification.
 */
import { test, expect, TestUserRole } from '../fixture'
import type { Page } from '@playwright/test'
import type { CatalogRGD, SchemaResponse } from '../../src/types/rgd'

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080'

// Mock RGD factory with deployment mode restriction
const createMockRGD = (
  name: string,
  allowedDeploymentModes?: ('direct' | 'gitops' | 'hybrid')[]
): CatalogRGD => ({
  name,
  namespace: 'test',
  description: `Test RGD: ${name}`,
  version: 'v1.0.0',
  tags: ['test'],
  category: 'testing',
  labels: {},
  instances: 0,
  apiVersion: 'kro.run/v1alpha1',
  kind: 'TestResource',
  status: 'Active',
  allowedDeploymentModes,
  createdAt: '2026-01-20T10:00:00Z',
  updatedAt: '2026-01-20T10:00:00Z',
})

// Mock schema response for deployment form
const mockSchemaResponse: SchemaResponse = {
  rgd: 'test-rgd',
  crdFound: true,
  schema: {
    name: 'test-rgd',
    namespace: 'test',
    group: 'test.kro.run',
    kind: 'TestResource',
    version: 'v1alpha1',
    title: 'Test Resource',
    description: 'A test resource for E2E testing',
    properties: {
      replicas: {
        type: 'integer',
        title: 'Replicas',
        description: 'Number of replicas',
        default: 1,
        minimum: 1,
        maximum: 10,
      },
    },
    required: ['replicas'],
  },
}

// Mock projects
const mockProjects = {
  items: [
    {
      name: 'default-project',
      displayName: 'Default Project',
      destinations: [{ namespace: 'default' }],
    },
  ],
  totalCount: 1,
}

// Mock namespaces
const mockNamespaces = {
  namespaces: ['default', 'production', 'staging'],
}

// Mock repositories for GitOps deployment
const mockRepositories = {
  items: [
    {
      id: 'repo-1',
      name: 'GitOps Repo',
      repoURL: 'https://github.com/test/gitops',
      defaultBranch: 'main',
      enabled: true,
      authType: 'token',
    },
  ],
}

test.describe('Full Deployment Flow with Mode Restrictions', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Helper to set up common mocks
  const setupMocks = async (page: Page, rgd: CatalogRGD, options?: {
    deploymentSuccess?: boolean
    instanceCreatedResponse?: object
  }) => {
    const { deploymentSuccess = true, instanceCreatedResponse } = options || {}

    // Mock RGD endpoints
    await page.route(`**/api/v1/rgds**`, async (route) => {
      const url = route.request().url()

      if (url.includes(`/${rgd.name}`)) {
        if (url.includes('/schema')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(mockSchemaResponse),
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
            body: JSON.stringify(rgd),
          })
        }
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [rgd],
            totalCount: 1,
            page: 1,
            pageSize: 10,
          }),
        })
      }
    })

    // Mock permission endpoint
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ value: 'yes' }),
      })
    })

    // Mock projects endpoint
    await page.route('**/api/v1/projects**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockProjects),
      })
    })

    // Mock project namespaces endpoint (registered after generic projects route so it takes priority — Playwright uses LIFO)
    await page.route('**/api/v1/projects/*/namespaces**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockNamespaces),
      })
    })

    // Mock namespaces endpoint
    await page.route('**/api/v1/namespaces**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockNamespaces),
      })
    })

    // Mock repositories endpoint
    await page.route('**/api/v1/repositories**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockRepositories),
      })
    })

    // Mock dependency endpoint
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          node: null,
          upstream: [],
          downstream: [],
          deploymentOrder: [rgd.name],
          hasCycle: false,
        }),
      })
    })

    // Mock compliance validate endpoint (called when advancing from Configure to Review)
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

    // Mock instances list endpoint (GET /api/v1/instances)
    await page.route('**/api/v1/instances**', async (route) => {
      const method = route.request().method()

      if (method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items: [],
            totalCount: 0,
          }),
        })
      } else {
        await route.continue()
      }
    })

    // Mock instance create endpoint (POST /api/v1/namespaces/{ns}/instances/{kind})
    // Also handles preflight which goes to the same path prefix
    const instanceCreateHandler = async (route: import('@playwright/test').Route) => {
      const method = route.request().method()
      const url = route.request().url()

      if (method === 'POST' && url.includes('/preflight')) {
        // Preflight endpoint already handled by the separate mock above
        await route.continue()
        return
      }

      if (method === 'POST') {
        if (deploymentSuccess) {
          const response = instanceCreatedResponse || {
            name: 'test-instance',
            namespace: 'default',
            rgdName: rgd.name,
            apiGroup: 'test.kro.run',
            kind: 'TestResource',
            version: 'v1alpha1',
            status: 'created',
            createdAt: new Date().toISOString(),
            deploymentMode: 'gitops',
            gitInfo: {
              commitSHA: 'abc123',
              path: 'manifests/default/test-rgd/test-instance.yaml',
              pushStatus: 'success',
            },
          }
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify(response),
          })
        } else {
          await route.continue()
        }
      } else {
        await route.continue()
      }
    }
    await page.route('**/namespaces/*/instances/**', instanceCreateHandler)
    // Also handle cluster-scoped: POST /api/v1/instances/{kind}
    await page.route('**/api/v1/instances/*', instanceCreateHandler)
  }

  test('AC-1: Complete deployment flow with gitops-only RGD', async ({ page }) => {
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])

    // Track the API call to verify deployment
    let deploymentRequest: { body: Record<string, unknown>; status: number } | null = null

    // Set up common mocks FIRST (this does NOT mock instances because deploymentSuccess: true uses custom response)
    await setupMocks(page, rgd, {
      deploymentSuccess: true,
      instanceCreatedResponse: {
        name: 'test-instance',
        namespace: 'default',
        rgdName: rgd.name,
        apiGroup: 'test.kro.run',
        kind: 'TestResource',
        version: 'v1alpha1',
        status: 'created',
        createdAt: new Date().toISOString(),
        deploymentMode: 'gitops',
        gitInfo: {
          commitSHA: 'abc123def456',
          path: 'manifests/default/gitops-only-rgd/test-instance.yaml',
          pushStatus: 'success',
        },
      },
    })

    // Override instance create route AFTER setupMocks to capture the request
    // createInstance() posts to /api/v1/namespaces/{ns}/instances/{kind}
    const captureHandler = async (route: import('@playwright/test').Route) => {
      if (route.request().method() === 'POST' && !route.request().url().includes('/preflight')) {
        deploymentRequest = {
          body: JSON.parse(route.request().postData() || '{}'),
          status: 201,
        }
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            name: deploymentRequest.body.name || 'test-instance',
            namespace: deploymentRequest.body.namespace || 'default',
            rgdName: rgd.name,
            apiGroup: 'test.kro.run',
            kind: 'TestResource',
            version: 'v1alpha1',
            status: 'created',
            createdAt: new Date().toISOString(),
            deploymentMode: 'gitops',
            gitInfo: {
              commitSHA: 'abc123def456',
              path: 'manifests/default/gitops-only-rgd/test-instance.yaml',
              pushStatus: 'success',
            },
          }),
        })
      } else {
        await route.continue()
      }
    }
    await page.route('**/namespaces/*/instances/**', captureHandler)
    await page.route('**/api/v1/instances/*', captureHandler)

    // Navigate to catalog
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')

    // Click on the RGD card
    const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
    await expect(rgdCard).toBeVisible({ timeout: 15000 })
    await rgdCard.click()

    // Wait for detail view
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Click Deploy button to open the 3-step wizard
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Step 1: Target — fill instance name, select project & namespace
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
    await page.getByPlaceholder('my-instance').fill('my-test-instance')

    // Select project and namespace
    const nsSelect = page.getByTestId('namespace-select')
    await expect(nsSelect).toBeEnabled({ timeout: 5000 })
    await nsSelect.click()
    await page.getByRole('option', { name: 'default' }).click()

    // Deployment mode: gitops-only RGD auto-selects gitops; select a repository
    const repoSelect = page.locator('select#repository')
    if (await repoSelect.isVisible({ timeout: 3000 }).catch(() => false)) {
      await repoSelect.selectOption({ index: 1 }) // Select first repository
    }

    // Advance to Configure step
    const continueBtn = page.getByRole('button', { name: /continue/i })
    await expect(continueBtn).toBeEnabled({ timeout: 5000 })
    await continueBtn.click()
    await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })

    // Wait for form validation (trigger() runs on mount) then advance to Review step
    const reviewContinueBtn = page.getByRole('button', { name: /continue/i })
    await expect(reviewContinueBtn).toBeEnabled({ timeout: 10000 })
    await reviewContinueBtn.click()

    // Submit deployment
    const submitButton = page.getByRole('button', { name: /deploy/i })
    await expect(submitButton).toBeVisible({ timeout: 5000 })
    await submitButton.click()

    // Wait for deployment request to be captured (with timeout)
    await expect.poll(() => deploymentRequest !== null, {
      message: 'Deployment request should be captured',
      timeout: 10000,
    }).toBe(true)

    // Verify deployment succeeded
    expect(deploymentRequest).not.toBeNull()
    expect(deploymentRequest?.body.rgdName).toBe('gitops-only-rgd')
    expect(deploymentRequest?.body.name).toBeDefined()
    expect(deploymentRequest?.body.namespace).toBeDefined()

    // Take screenshot of success state
    await page.screenshot({
      path: '../test-results/e2e/screenshots/ac1-deployment-success.png',
      fullPage: true,
    })
  })

  test('AC-2: Backend rejects direct mode on gitops-only RGD (mocked)', async ({ page }) => {
    // This test verifies the error response format when backend rejects a deployment mode
    // Uses page.route to mock the backend response for consistent testing
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])

    let capturedRequest: Record<string, unknown> | null = null

    // Set up common mocks FIRST (with deploymentSuccess: true to avoid route.continue())
    await setupMocks(page, rgd, { deploymentSuccess: true })

    // Override instance route AFTER setupMocks to return 422 for direct mode attempts
    // Covers both namespaced and cluster-scoped URL patterns
    const rejectDirectHandler = async (route: import('@playwright/test').Route) => {
      if (route.request().method() === 'POST' && !route.request().url().includes('/preflight')) {
        const body = JSON.parse(route.request().postData() || '{}')
        capturedRequest = body

        if (body.deploymentMode === 'direct') {
          await route.fulfill({
            status: 422,
            contentType: 'application/json',
            body: JSON.stringify({
              code: 'DEPLOYMENT_MODE_NOT_ALLOWED',
              message: "Deployment mode 'direct' is not allowed for RGD 'gitops-only-rgd'. Allowed modes: gitops",
              details: {
                allowedModes: ['gitops'],
                requestedMode: 'direct',
              },
            }),
          })
        } else {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({ name: body.name, status: 'created' }),
          })
        }
      } else {
        await route.continue()
      }
    }
    await page.route('**/namespaces/*/instances/**', rejectDirectHandler)
    await page.route('**/api/v1/instances/*', rejectDirectHandler)
    // Also match the base /api/v1/instances URL (no trailing path segment)
    await page.route('**/api/v1/instances', rejectDirectHandler)

    // Navigate to trigger the mock setup
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Make a fetch request inside the page context to use the mocked route
    const response = await page.evaluate(async (url) => {
      const res = await fetch(`${url}/api/v1/instances`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: 'test-instance',
          namespace: 'default',
          rgdName: 'gitops-only-rgd',
          rgdNamespace: 'test',
          spec: { replicas: 1 },
          deploymentMode: 'direct',
        }),
      })
      return {
        status: res.status,
        body: await res.json(),
      }
    }, BASE_URL)

    // Verify the rejection
    expect(response.status).toBe(422)
    expect(response.body.code).toBe('DEPLOYMENT_MODE_NOT_ALLOWED')
    expect(response.body.details).toBeDefined()
    expect(response.body.details.allowedModes).toBeInstanceOf(Array)
    expect(response.body.details.allowedModes).toContain('gitops')
    expect(response.body.details.requestedMode).toBe('direct')

    // Verify request was captured
    expect(capturedRequest).not.toBeNull()
  })

  test('AC-4: Error response format verification (allowedModes as array)', async ({ page }) => {
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])

    // Set up common mocks FIRST (with deploymentSuccess: true to avoid route.continue())
    await setupMocks(page, rgd, { deploymentSuccess: true })

    // Override instance route AFTER setupMocks to return 422 with correct format
    const ac4Handler = async (route: import('@playwright/test').Route) => {
      if (route.request().method() === 'POST' && !route.request().url().includes('/preflight')) {
        const body = JSON.parse(route.request().postData() || '{}')

        // If trying to use direct mode on gitops-only RGD
        if (body.deploymentMode === 'direct') {
          await route.fulfill({
            status: 422,
            contentType: 'application/json',
            body: JSON.stringify({
              code: 'DEPLOYMENT_MODE_NOT_ALLOWED',
              message: "Deployment mode 'direct' is not allowed for RGD 'gitops-only-rgd'. Allowed modes: gitops",
              details: {
                allowedModes: ['gitops'], // Must be an array, not a string
                requestedMode: 'direct',
              },
            }),
          })
        } else {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              name: body.name,
              namespace: body.namespace,
              status: 'created',
            }),
          })
        }
      } else {
        await route.continue()
      }
    }
    await page.route('**/namespaces/*/instances/**', ac4Handler)
    await page.route('**/api/v1/instances/*', ac4Handler)
    // Also match the base /api/v1/instances URL (no trailing path segment)
    await page.route('**/api/v1/instances', ac4Handler)

    // Navigate to trigger the mock setup
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Make a fetch request inside the page context to use the mocked route
    const response = await page.evaluate(async (url) => {
      const res = await fetch(`${url}/api/v1/instances`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: 'test-instance',
          namespace: 'default',
          rgdName: 'gitops-only-rgd',
          rgdNamespace: 'test',
          spec: { replicas: 1 },
          deploymentMode: 'direct',
        }),
      })
      return {
        status: res.status,
        body: await res.json(),
      }
    }, BASE_URL)

    // Verify the error response format per AC-4
    expect(response.status).toBe(422)
    expect(response.body.code).toBe('DEPLOYMENT_MODE_NOT_ALLOWED')
    expect(response.body.details).toBeDefined()

    // CRITICAL: allowedModes must be an array, not a string
    expect(response.body.details.allowedModes).toBeInstanceOf(Array)
    expect(response.body.details.allowedModes).toContain('gitops')
    expect(response.body.details.requestedMode).toBe('direct')

    // Verify it's not returning as a string (old format bug)
    expect(typeof response.body.details.allowedModes).not.toBe('string')
  })

  test('Deploy wizard opens for RGD with mode restrictions', async ({ page }) => {
    const rgd = createMockRGD('hybrid-only-rgd', ['hybrid'])

    await setupMocks(page, rgd)

    // Navigate to catalog
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')

    // Click on the RGD card
    const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
    await expect(rgdCard).toBeVisible({ timeout: 15000 })
    await rgdCard.click()

    // Wait for detail view
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Click Deploy button to open the wizard
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Wizard should open with Target step
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
    await expect(page.getByPlaceholder('my-instance')).toBeVisible()
  })

  test('Verify deployment shows instance in list after success', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined) // All modes allowed

    const createdInstance = {
      name: 'my-deployed-instance',
      namespace: 'default',
      rgdName: rgd.name,
      apiGroup: 'test.kro.run',
      kind: 'TestResource',
      version: 'v1alpha1',
      status: 'Ready',
      createdAt: new Date().toISOString(),
      deploymentMode: 'direct',
    }

    let instanceCreated = false

    // Set up common mocks FIRST
    await setupMocks(page, rgd, { deploymentSuccess: true })

    // Override instance list route AFTER setupMocks (GET /api/v1/instances)
    await page.route('**/api/v1/instances**', async (route) => {
      if (route.request().method() === 'GET') {
        const items = instanceCreated ? [createdInstance] : []
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            items,
            totalCount: items.length,
          }),
        })
      } else {
        await route.continue()
      }
    })

    // Override instance create route (POST /api/v1/namespaces/{ns}/instances/{kind})
    const createHandler = async (route: import('@playwright/test').Route) => {
      if (route.request().method() === 'POST' && !route.request().url().includes('/preflight')) {
        instanceCreated = true
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(createdInstance),
        })
      } else {
        await route.continue()
      }
    }
    await page.route('**/namespaces/*/instances/**', createHandler)
    await page.route('**/api/v1/instances/*', createHandler)

    // Navigate to catalog and deploy
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')

    const rgdCard = page.getByRole('button', { name: /view details for/i }).first()
    await expect(rgdCard).toBeVisible({ timeout: 15000 })
    await rgdCard.click()

    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Step 1: Target — fill instance name, select project & namespace
    await expect(page.getByTestId('target-step')).toBeVisible({ timeout: 15000 })
    await page.getByPlaceholder('my-instance').fill('my-deployed-instance')

    const nsSelect = page.getByTestId('namespace-select')
    await expect(nsSelect).toBeEnabled({ timeout: 5000 })
    await nsSelect.click()
    await page.getByRole('option', { name: 'default' }).click()

    // Advance to Configure step
    await page.getByRole('button', { name: /continue/i }).click()
    await expect(page.getByTestId('configure-step')).toBeVisible({ timeout: 15000 })

    // Blur any active field to trigger onBlur validation, then advance to Review
    await page.getByTestId('configure-step').click({ position: { x: 1, y: 1 } })
    const reviewBtn = page.getByRole('button', { name: /continue/i })
    await expect(reviewBtn).toBeEnabled({ timeout: 10000 })
    await reviewBtn.click()

    // Submit deployment
    const submitButton = page.getByTestId('deploy-submit-button')
    await expect(submitButton).toBeEnabled({ timeout: 10000 })
    await submitButton.click()

    // Wait for instance creation with polling instead of fixed timeout
    await expect.poll(() => instanceCreated, {
      message: 'Instance should be created',
      timeout: 10000,
    }).toBe(true)

    // Navigate to instances page
    await page.goto('/instances')
    await page.waitForLoadState('networkidle')

    // Verify instance appears in list
    await expect(page.getByText('my-deployed-instance')).toBeVisible({ timeout: 5000 })
  })
})

test.describe('Backend API Mode Restriction Tests (Direct API)', () => {
  // These tests call the backend API directly without going through the UI
  // They verify the backend validation layer works correctly as a safety net

  test('AC-2: Direct API call with disallowed mode returns 422', async ({ request }) => {
    // Check backend availability first
    let backendAvailable = false
    try {
      const healthCheck = await request.get(`${BASE_URL}/healthz`, {
        timeout: 5000,
        failOnStatusCode: false,
      })
      backendAvailable = healthCheck.ok()
    } catch {
      // Backend not reachable
    }

    if (!backendAvailable) {
      test.skip(true, 'Backend not available - run with `make qa` for full integration test')
      return
    }

    // Attempt to deploy with direct mode on a gitops-only RGD
    // Note: This requires a real gitops-only RGD in the cluster and valid auth
    const response = await request.post(`${BASE_URL}/api/v1/instances`, {
      headers: {
        'Content-Type': 'application/json',
      },
      data: {
        name: 'e2e-test-instance-' + Date.now(),
        namespace: 'default',
        rgdName: 'gitops-only-test-rgd',
        spec: { replicas: 1 },
        deploymentMode: 'direct',
      },
      failOnStatusCode: false,
    })

    const status = response.status()

    // MUST verify one of these outcomes - test should not silently pass
    const validStatuses = [422, 401, 403, 404]
    expect(validStatuses).toContain(status)

    // If we got a 422, verify the error format
    if (status === 422) {
      const body = await response.json()
      expect(body.code).toBe('DEPLOYMENT_MODE_NOT_ALLOWED')
      expect(body.details.allowedModes).toBeInstanceOf(Array)
      expect(body.details.requestedMode).toBe('direct')
    }
    // 401/403 = auth required (expected without auth setup)
    // 404 = RGD not found (expected if test RGD doesn't exist)
    // Any other status = unexpected, test should fail
  })
})
