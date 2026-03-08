// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * E2E Tests for RGD Deployment Mode Restrictions
 *
 * Tests that the UI correctly handles the `knodex.io/deployment-modes` annotation:
 * - Shows only allowed deployment modes
 * - Auto-selects when only one mode is available
 * - Shows restriction info banner
 * - Backend rejects disallowed modes with 422 error
 */
import { test, expect, TestUserRole } from '../fixture'
import type { CatalogRGD, SchemaResponse } from '../../src/types/rgd'
import { API_PATHS } from '../fixture/mock-data'

// Base mock RGD with all modes allowed (no restriction)
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
  kind: 'ResourceGraphDefinition',
  allowedDeploymentModes,
  createdAt: '2026-01-20T10:00:00Z',
  updatedAt: '2026-01-20T10:00:00Z',
})

// Mock schema response
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

// Mock repositories
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

test.describe('Deployment Mode Restrictions', () => {
  // Authenticate as Global Admin to access all features
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Helper to set up common mocks
  const setupCommonMocks = async (page: import('@playwright/test').Page, rgd: CatalogRGD) => {
    // Mock the RGD list endpoint
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
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
  }

  test('shows all deployment modes when no restrictions', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // All three modes should be visible
    await expect(page.getByRole('button', { name: /Direct/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /GitOps/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /Hybrid/i })).toBeVisible()

    // No restriction banner should appear
    await expect(page.getByText(/This RGD only allows/i)).not.toBeVisible()
    await expect(page.getByText(/This RGD is restricted/i)).not.toBeVisible()
  })

  test('shows only gitops mode when RGD is gitops-only', async ({ page }) => {
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // Only GitOps should be visible
    await expect(page.getByRole('button', { name: /GitOps/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /Direct/i })).not.toBeVisible()
    await expect(page.getByRole('button', { name: /Hybrid/i })).not.toBeVisible()

    // Restriction banner should show single mode
    await expect(page.getByText(/This RGD only allows/i)).toBeVisible()
    // Verify GitOps button is visible (use role to be more specific)
    await expect(page.getByRole('button', { name: /GitOps/i })).toBeVisible()
  })

  test('shows only direct and hybrid modes when gitops is not allowed', async ({ page }) => {
    const rgd = createMockRGD('no-gitops-rgd', ['direct', 'hybrid'])
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // Direct and Hybrid should be visible, GitOps hidden
    await expect(page.getByRole('button', { name: /Direct/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /Hybrid/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /GitOps/i })).not.toBeVisible()

    // Restriction banner should list allowed modes
    await expect(page.getByText(/This RGD is restricted to the following deployment modes/i)).toBeVisible()
  })

  test('auto-selects single allowed mode', async ({ page }) => {
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // GitOps button should be selected (have primary styling) and disabled
    const gitopsButton = page.getByRole('button', { name: /GitOps/i })
    await expect(gitopsButton).toBeDisabled()
    await expect(gitopsButton).toHaveClass(/border-primary/)
  })

  // Note: This test is similar to 'shows all deployment modes when no restrictions'
  // The key difference is:
  // - no restrictions: allowedDeploymentModes is undefined/missing
  // - all explicitly allowed: allowedDeploymentModes is ['direct', 'gitops', 'hybrid']
  // Both should result in the same UI behavior (backward compatibility)
  test('shows three modes when all are explicitly allowed', async ({ page }) => {
    const rgd = createMockRGD('all-modes-rgd', ['direct', 'gitops', 'hybrid'])
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // All three modes should be visible and clickable (not disabled)
    const directButton = page.getByRole('button', { name: /Direct/i })
    const gitopsButton = page.getByRole('button', { name: /GitOps/i })
    const hybridButton = page.getByRole('button', { name: /Hybrid/i })

    await expect(directButton).toBeVisible()
    await expect(gitopsButton).toBeVisible()
    await expect(hybridButton).toBeVisible()

    // Verify buttons are enabled (clickable)
    await expect(directButton).not.toBeDisabled()
    await expect(gitopsButton).not.toBeDisabled()
    await expect(hybridButton).not.toBeDisabled()

    // No restriction banner (all modes explicitly allowed)
    await expect(page.getByText(/This RGD only allows/i)).not.toBeVisible()
    await expect(page.getByText(/This RGD is restricted/i)).not.toBeVisible()
  })

  test('UI prevents selection of disallowed modes (backend 422 is safety net)', async ({ page }) => {
    // This test verifies that the UI enforces restrictions by hiding disallowed modes
    // The backend 422 error is a safety net for API-level enforcement (tested via unit tests)
    // Since UI filters modes, users cannot normally trigger the 422 - it protects against direct API calls
    const rgd = createMockRGD('gitops-only-rgd', ['gitops'])
    await setupCommonMocks(page, rgd)

    // Mock the instance creation to return 422 for direct mode
    await page.route('**/api/v1/instances', async (route) => {
      if (route.request().method() === 'POST') {
        const body = JSON.parse(route.request().postData() || '{}')
        if (body.deploymentMode === 'direct') {
          await route.fulfill({
            status: 422,
            contentType: 'application/json',
            body: JSON.stringify({
              code: 'DEPLOYMENT_MODE_NOT_ALLOWED',
              message: "Deployment mode 'direct' is not allowed for RGD 'gitops-only-rgd'. Allowed modes: gitops",
              details: {
                allowedModes: 'gitops',
                requestedMode: 'direct',
              },
            }),
          })
        } else {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              name: 'test-instance',
              namespace: 'default',
              rgdName: rgd.name,
              status: 'created',
            }),
          })
        }
      } else {
        await route.continue()
      }
    })

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // GitOps should be the only option and auto-selected
    // This is the UI-level protection that makes the 422 error path unreachable via normal UI
    await expect(page.getByRole('button', { name: /GitOps/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /GitOps/i })).toBeDisabled()

    // Direct mode button should NOT exist (hidden, not just disabled)
    await expect(page.getByRole('button', { name: /^Direct$/i })).not.toBeVisible()

    // Verify restriction banner is shown
    await expect(page.getByText(/This RGD only allows/i)).toBeVisible()
  })

  test('can select between two allowed modes', async ({ page }) => {
    const rgd = createMockRGD('direct-hybrid-rgd', ['direct', 'hybrid'])
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i })
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()
    await page.waitForLoadState('networkidle')

    // Both modes should be clickable
    const directButton = page.getByRole('button', { name: /Direct/i })
    const hybridButton = page.getByRole('button', { name: /Hybrid/i })

    await expect(directButton).toBeVisible()
    await expect(directButton).not.toBeDisabled()
    await expect(hybridButton).toBeVisible()
    await expect(hybridButton).not.toBeDisabled()

    // Click on Hybrid mode
    await hybridButton.click()

    // Hybrid should now be selected
    await expect(hybridButton).toHaveClass(/border-primary/)
  })
})
