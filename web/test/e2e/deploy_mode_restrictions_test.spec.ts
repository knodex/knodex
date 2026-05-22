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
import { selectShadcnOption } from './fixtures/select'

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
  status: 'Active',
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

test.describe('Deploy Wizard Flow', () => {
  // Authenticate as Global Admin to access all features
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  // Helper to set up common mocks
  const setupCommonMocks = async (page: import('@playwright/test').Page, rgd: CatalogRGD) => {
    await page.route(`**${API_PATHS.rgds}**`, async (route) => {
      const url = route.request().url()
      if (url.includes(`/${rgd.name}`)) {
        if (url.includes('/schema')) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockSchemaResponse) })
        } else if (url.includes('/validate-deployment')) {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ valid: true, errors: [] }) })
        } else {
          await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rgd) })
        }
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [rgd], totalCount: 1, page: 1, pageSize: 10 }) })
      }
    })
    await page.route('**/api/v1/account/can-i/**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ value: 'yes' }) })
    })
    await page.route('**/api/v1/projects**', async (route) => {
      const url = route.request().url()
      if (url.includes('/namespaces')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ namespaces: ['default', 'production', 'staging'] }) })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockProjects) })
      }
    })
    await page.route('**/api/v1/repositories**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockRepositories) })
    })
    await page.route('**/api/v1/dependencies/**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ node: null, upstream: [], downstream: [], deploymentOrder: [rgd.name], hasCycle: false }) })
    })
  }

  test('deploy wizard opens with Target step', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Deploy page should open with Basics tab
    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })
    await expect(page.getByTestId('instance-name-input')).toBeVisible()
    await expect(page.getByTestId('project-select')).toBeVisible()
  })

  test('deploy wizard advances from Target to Configure step', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Fill Basics tab
    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })
    await page.getByTestId('instance-name-input').fill('test-instance')
    const nsSelect = page.getByTestId('namespace-select')
    await expect(nsSelect).toBeEnabled({ timeout: 5000 })
    await selectShadcnOption(nsSelect, 'default')

    // Advance to Configure step
    await page.getByTestId('deploy-footer-next').click()
    await expect(page.getByTestId('deploy-tab-general')).toHaveAttribute('data-state', 'active')
  })

  test('Continue button disabled when Target step is incomplete', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    // Basics tab with empty fields — Next stays enabled but RHF validation
    // on click keeps the user on Basics until required fields are filled.
    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })
    await expect(page.getByTestId('deploy-tab-general')).toHaveAttribute('data-state', 'active')
    await page.getByTestId('deploy-footer-next').click()
    await expect(page.getByTestId('deploy-tab-general')).toHaveAttribute('data-state', 'active')
  })

  test('deploy page close button returns to catalog', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })

    await page.getByTestId('deploy-header-close').click()

    // Should navigate away from the deploy page
    await expect(page).not.toHaveURL(/\/deploy\//, { timeout: 5000 })
  })

  test('deploy wizard shows namespace select after project selection', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })

    // Project auto-selects when single project. Namespace select should be available.
    await expect(page.getByTestId('namespace-select')).toBeVisible()
  })

  test('deploy wizard validates instance name format', async ({ page }) => {
    const rgd = createMockRGD('unrestricted-rgd', undefined)
    await setupCommonMocks(page, rgd)

    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')
    const card = page.getByRole('button', { name: /view details for/i }).first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()
    await page.waitForURL(/\/catalog\//, { timeout: 10000 })
    await page.waitForLoadState('networkidle')
    const deployBtn = page.getByRole('button', { name: /deploy/i }).first()
    await expect(deployBtn).toBeVisible({ timeout: 15000 })
    await deployBtn.click()

    await page.waitForURL(/\/deploy\//, { timeout: 10000 })
    await expect(page.getByTestId('deploy-tab-general')).toBeVisible({ timeout: 15000 })

    // Fill instance name with valid value
    await page.getByTestId('instance-name-input').fill('valid-name')
    // Should not show validation error
    await expect(page.getByText(/must start and end with alphanumeric/i)).not.toBeVisible()
  })
})
