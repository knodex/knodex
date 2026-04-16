// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect, TestUserRole } from '../fixture'
import {
  mockRGDListResponse,
  mockRGDs,
  mockInstances,
  API_PATHS,
} from '../fixture/mock-data'
import type { Instance } from '../../src/types/rgd'
import type { Project, ProjectListResponse } from '../../src/types/project'

/**
 * Instance Detail View — E2E Tests
 *
 * Tests the redesigned instance detail page:
 * - Header with status, kind (linked to RGD), namespace
 * - Source row (direct vs gitops)
 * - Collapsible conditions
 * - Spec viewer with copy button
 * - RGD description display
 */

// Instance with matching project destinations
const detailInstance: Instance = {
  ...mockInstances[0],
  name: 'my-database',
  namespace: 'team-alpha',
  rgdName: 'postgres-database',
  rgdNamespace: 'databases',
  kind: 'PostgresDatabase',
  health: 'Healthy',
  deploymentMode: 'direct',
  conditions: [
    { type: 'Ready', status: 'True', reason: 'AllReady', message: 'All resources ready' },
    { type: 'Synced', status: 'True', reason: 'Synced', message: 'Resources synced' },
  ],
  spec: { replicas: 3, storage: '100Gi' },
  status: { state: 'ACTIVE' },
  createdAt: '2026-03-20T10:00:00Z',
  updatedAt: '2026-03-27T14:00:00Z',
}

// GitOps instance with git info
const gitopsInstance: Instance = {
  ...detailInstance,
  name: 'prod-api',
  namespace: 'team-alpha',
  deploymentMode: 'gitops',
  reconciliationSuspended: false,
  annotations: { 'gitops.knodex.io/vcs': 'github' },
  gitInfo: {
    repositoryUrl: 'test-org/test-repo',
    branch: 'main',
    commitSha: 'a1b2c3d4e5f6',
    path: 'instances/team-alpha/PostgresDatabase/prod-api.yaml',
    pushStatus: 'success',
  },
}

// GitOps instance with reconciliation suspended
const suspendedInstance: Instance = {
  ...gitopsInstance,
  name: 'suspended-api',
  reconciliationSuspended: true,
}

// Unhealthy instance with failing condition
const unhealthyInstance: Instance = {
  ...detailInstance,
  name: 'broken-cache',
  namespace: 'team-alpha',
  health: 'Unhealthy',
  conditions: [
    { type: 'Ready', status: 'False', reason: 'NotReady', message: 'Deployment failed: OOMKilled' },
    { type: 'Synced', status: 'True', reason: 'Synced', message: 'Resources synced' },
  ],
  status: { state: 'FAILED' },
}

// Projects that match via namespace patterns
const testProjects: Project[] = [
  {
    name: 'alpha-team',
    description: 'Alpha team deployments',
    destinations: [{ namespace: 'team-alpha' }],
    roles: [],
    resourceVersion: '1',
    createdAt: '2026-01-01T00:00:00Z',
    createdBy: 'admin',
  },
  {
    name: 'platform',
    description: 'Platform-wide resources',
    destinations: [{ namespace: 'team-*' }],
    roles: [],
    resourceVersion: '1',
    createdAt: '2026-01-01T00:00:00Z',
    createdBy: 'admin',
  },
]

const testProjectListResponse: ProjectListResponse = {
  items: testProjects,
  totalCount: testProjects.length,
}

test.describe('Instance Detail View', () => {
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN })

  function setupMocks(page: import('@playwright/test').Page, instance: Instance) {
    return Promise.all([
      // Mock account/info so session restore succeeds (prevents "Connection Error" state)
      page.route('**/api/v1/account/info', async (route) => {
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
      }),
      // Mock individual RGD fetch (for description/parentRGD — must be registered before the list mock)
      page.route('**/api/v1/rgds/postgres-database*', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDs[0]),
        })
      }),
      // Mock RGD list
      page.route(/\/api\/v1\/rgds(\?|$)/, async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockRGDListResponse),
        })
      }),
      // Mock instance fetch
      page.route(`**${API_PATHS.instances}**`, async (route) => {
        const url = route.request().url()
        if (url.includes(`/${instance.name}`)) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(instance),
          })
        } else if (url.includes('/count')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ count: 1 }),
          })
        } else {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ items: [instance], totalCount: 1, page: 1, pageSize: 10 }),
          })
        }
      }),
      // Mock namespaced instance fetch (K8s-aligned path)
      page.route(`**${API_PATHS.namespacedInstances}**`, async (route) => {
        const url = route.request().url()
        if (url.includes(`/${instance.name}`)) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(instance),
          })
        } else if (url.includes('/count')) {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ count: 1 }),
          })
        } else {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ items: [instance], totalCount: 1, page: 1, pageSize: 10 }),
          })
        }
      }),
      // Mock projects (for namespace → project resolution)
      page.route(`**${API_PATHS.projects}**`, async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(testProjectListResponse),
        })
      }),
      // Mock can-i permissions
      page.route(`**${API_PATHS.canI}**`, async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ allowed: true }),
        })
      }),
      // Mock history/timeline
      page.route('**/history**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ timeline: [], events: [] }),
        })
      }),
      page.route('**/timeline**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ namespace: instance.namespace, kind: instance.kind, name: instance.name, timeline: [] }),
        })
      }),
    ])
  }

  test.describe('Header metadata', () => {
    test('shows instance name, health status, kind linked to RGD, and namespace', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      // Instance name as heading
      await expect(page.getByRole('heading', { name: detailInstance.name })).toBeVisible()

      // Health status with label
      await expect(page.getByText('Healthy')).toBeVisible()

      // Kind linked to RGD catalog
      const kindLink = page.getByRole('link', { name: 'PostgresDatabase' })
      await expect(kindLink).toBeVisible()
      await expect(kindLink).toHaveAttribute('href', '/catalog/postgres-database')

      // Namespace explicitly labeled
      await expect(page.getByText('Namespace')).toBeVisible()
      await expect(page.getByText('team-alpha', { exact: true })).toBeVisible()
    })

    test('shows RGD description as subtitle', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      // RGD description from parent RGD
      await expect(page.getByText('PostgreSQL database with automated backups and monitoring')).toBeVisible()
    })
  })

  test.describe('Source row', () => {
    test('shows "Direct deployment" for direct mode', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      await expect(page.getByText('Direct deployment')).toBeVisible()
    })

    test('shows repository link for gitops mode', async ({ page }) => {
      await setupMocks(page, gitopsInstance)
      await page.goto(`/instances/${gitopsInstance.namespace}/${gitopsInstance.kind}/${gitopsInstance.name}`)

      const repoLink = page.getByRole('link', { name: 'test-org/test-repo' })
      await expect(repoLink).toBeVisible()
      await expect(repoLink).toHaveAttribute(
        'href',
        'https://github.com/test-org/test-repo/blob/main/instances/team-alpha/PostgresDatabase/prod-api.yaml'
      )
    })

    test('does not show branch or path chips in source row for gitops mode', async ({ page }) => {
      await setupMocks(page, gitopsInstance)
      await page.goto(`/instances/${gitopsInstance.namespace}/${gitopsInstance.kind}/${gitopsInstance.name}`)

      // Source row is the thin metadata bar above the tabs (not the Deployment Information card)
      const sourceRow = page.locator('[style*="border-bottom"]').filter({ hasText: 'Source' })
      await expect(sourceRow).toBeVisible()
      // Branch chip and path chip should not appear in the source row
      await expect(sourceRow.getByText('main')).not.toBeVisible()
      await expect(sourceRow.getByText('instances/team-alpha')).not.toBeVisible()
    })
  })

  test.describe('Deployment Information card', () => {
    test('shows Synchronisation: Synced for active gitops instance', async ({ page }) => {
      await setupMocks(page, gitopsInstance)
      await page.goto(`/instances/${gitopsInstance.namespace}/${gitopsInstance.kind}/${gitopsInstance.name}`)

      // Expand the Deployment Information card (collapsed by default)
      await page.getByRole('button', { name: 'Deployment Information' }).click()

      await expect(page.getByText('Synchronisation')).toBeVisible()
      await expect(page.getByText('Synced')).toBeVisible()
      await expect(page.getByText('Suspended')).not.toBeVisible()
    })

    test('shows Synchronisation: Suspended when reconciliation is suspended', async ({ page }) => {
      await setupMocks(page, suspendedInstance)
      await page.goto(`/instances/${suspendedInstance.namespace}/${suspendedInstance.kind}/${suspendedInstance.name}`)

      // Expand the Deployment Information card (collapsed by default)
      await page.getByRole('button', { name: 'Deployment Information' }).click()

      await expect(page.getByText('Synchronisation')).toBeVisible()
      // Use exact match to avoid matching the "suspended-api" instance name heading
      await expect(page.getByText('Suspended', { exact: true })).toBeVisible()
      await expect(page.getByText('Synced', { exact: true })).not.toBeVisible()
    })

    test('shows Repository row linking to repo root', async ({ page }) => {
      await setupMocks(page, gitopsInstance)
      await page.goto(`/instances/${gitopsInstance.namespace}/${gitopsInstance.kind}/${gitopsInstance.name}`)

      // Expand the Deployment Information card (collapsed by default)
      await page.getByRole('button', { name: 'Deployment Information' }).click()

      await expect(page.getByText('Repository')).toBeVisible()
      const repoLink = page.getByRole('link', { name: 'test-org/test-repo' }).nth(1)
      await expect(repoLink).toHaveAttribute('href', 'https://github.com/test-org/test-repo')
    })

    test('does not show Deployment Information card for direct mode', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      await expect(page.getByText('Synchronisation')).not.toBeVisible()
      await expect(page.getByText('Repository')).not.toBeVisible()
    })
  })

  test.describe('Conditions', () => {
    test('shows collapsed conditions with count when all healthy', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      // Conditions summary visible
      await expect(page.getByText('2/2')).toBeVisible()

      // Individual conditions NOT visible (collapsed)
      await expect(page.getByText('AllReady')).not.toBeVisible()
    })

    test('expands conditions on click', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      // Click the conditions toggle
      await page.getByRole('button', { name: /conditions/i }).click()

      // Now individual conditions are visible
      await expect(page.getByText('Ready', { exact: true })).toBeVisible()
      await expect(page.getByText('All resources ready')).toBeVisible()
    })

    test('auto-expands conditions when any is failing', async ({ page }) => {
      await setupMocks(page, unhealthyInstance)
      await page.goto(`/instances/${unhealthyInstance.namespace}/${unhealthyInstance.kind}/${unhealthyInstance.name}`)

      // Failing condition visible immediately (auto-expanded)
      await expect(page.getByText('Deployment failed: OOMKilled')).toBeVisible()
      await expect(page.getByText('1/2')).toBeVisible()
    })
  })

  test.describe('Spec viewer', () => {
    test('shows spec JSON with copy button', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      // Navigate to Spec tab
      await page.getByRole('tab', { name: /spec/i }).click()

      // Spec content visible
      await expect(page.getByTestId('spec-content')).toBeVisible()
      await expect(page.getByText('"replicas": 3')).toBeVisible()

      // Copy button present
      await expect(page.getByRole('button', { name: /copy/i })).toBeVisible()
    })
  })

  test.describe('Tab navigation', () => {
    test('defaults to Status tab', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      const statusTab = page.getByRole('tab', { name: /^Status$/i })
      await expect(statusTab).toHaveAttribute('aria-selected', 'true')
    })

    test('switches to Deployment History tab', async ({ page }) => {
      await setupMocks(page, detailInstance)
      await page.goto(`/instances/${detailInstance.namespace}/${detailInstance.kind}/${detailInstance.name}`)

      await page.getByRole('tab', { name: /deployment history/i }).click()

      // Deployment History content area visible
      const historyTab = page.getByRole('tab', { name: /deployment history/i })
      await expect(historyTab).toHaveAttribute('aria-selected', 'true')
    })
  })
})
